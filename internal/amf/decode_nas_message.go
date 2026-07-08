// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// Modified by Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"crypto/hmac"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/security"
	"go.uber.org/zap"
)

// DecodeNASMessage parses a 5GS NAS PDU (plain or security-protected) and
// returns the decoded message together with a policy Verdict. The only UE
// mutation performed here is advancing ue.ULCount (TS 24.501, TS 33.501).
func DecodeNASMessage(ue *UeContext, payload []byte) (*DecodeResult, error) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	if payload == nil {
		return nil, fmt.Errorf("nas payload is empty")
	}

	if len(payload) < 2 {
		return nil, fmt.Errorf("nas payload is too short")
	}

	msg := new(nas.Message)
	msg.SecurityHeaderType = nas.GetSecurityHeaderType(payload) & 0x0f

	conn := ue.Conn()

	if msg.SecurityHeaderType == nas.SecurityHeaderTypePlainNas {
		// TS 24.501: once secure NAS exchange is established for the
		// connection, a message that is not integrity protected is discarded.
		if conn.SecureExchangeEstablished() {
			return nil, fmt.Errorf("plain NAS message discarded: secure exchange already established (TS 24.501)")
		}

		return decodePlainNAS(msg, payload)
	}

	return decodeProtectedNAS(ue, msg, payload, conn)
}

// NasIntegrityVerified reports whether payload is an integrity-protected NAS
// PDU whose MAC verifies against this UE's current 5G NAS security context. It
// does not mutate any UE state: the uplink count is evaluated on a copy.
//
// It is the authorization gate for an inbound message that resolved to an
// existing context by GUTI/5G-S-TMSI: only a message proven to originate from
// the holder of the keys may act on that context (TS 24.501).
func (ue *UeContext) NasIntegrityVerified(payload []byte) bool {
	if ue == nil {
		return false
	}

	ue.mu.Lock()
	defer ue.mu.Unlock()

	if !ue.secured {
		return false
	}

	if len(payload) < 7 {
		return false
	}

	switch nas.GetSecurityHeaderType(payload) & 0x0f {
	case nas.SecurityHeaderTypeIntegrityProtected,
		nas.SecurityHeaderTypeIntegrityProtectedAndCiphered:
	default:
		return false
	}

	receivedMac := payload[2:6]
	sqn := payload[6]

	cnt := ue.ulCount.ReconcileUplink(sqn) // never committed back to the context

	mac, err := security.NASMacCalculate(ue.integrityAlg, ue.knasInt, cnt.Value(), security.Bearer3GPP, security.DirectionUplink, payload[6:])
	if err != nil {
		return false
	}

	return hmac.Equal(mac, receivedMac)
}

// ReuseForInboundNAS reports whether an inbound NAS PDU that resolved to this
// committed context by GUTI/5G-S-TMSI may act on it: only when integrity-verified.
// Any other message is processed on a fresh context, leaving the committed
// security context and PDU sessions untouched (TS 24.501).
func (ue *UeContext) ReuseForInboundNAS(payload []byte) bool {
	return ue.NasIntegrityVerified(payload)
}

func decodePlainNAS(msg *nas.Message, payload []byte) (*DecodeResult, error) {
	if err := msg.PlainNasDecode(&payload); err != nil {
		return nil, err
	}

	if msg.GmmMessage == nil {
		return nil, fmt.Errorf("plain NAS message has no GMM body")
	}

	msgType := msg.GmmHeader.GetMessageType()

	if classifyNasPdu(msgType, nas.SecurityHeaderTypePlainNas, false) == VerdictReject {
		return nil, fmt.Errorf(
			"plain NAS message type %d not permitted by TS 24.501",
			msgType,
		)
	}

	return &DecodeResult{Message: msg, IntegrityVerified: false}, nil
}

func decodeProtectedNAS(ue *UeContext, msg *nas.Message, payload []byte, conn *UeConn) (*DecodeResult, error) {
	if len(payload) < 7 {
		return nil, fmt.Errorf("nas payload is too short")
	}

	securityHeader := payload[0:6]
	sequenceNumber := payload[6]
	receivedMac32 := securityHeader[2:]
	payload = payload[6:]

	ciphered := false

	// Work on a copy of the uplink count and commit it to the security context
	// only once the MAC is verified, so an unauthenticated message cannot
	// advance (desync) the count of a genuine UE (TS 33.501).
	cnt := ue.ulCount

	switch msg.SecurityHeaderType {
	case nas.SecurityHeaderTypeIntegrityProtected:
	case nas.SecurityHeaderTypeIntegrityProtectedAndCiphered:
		ciphered = true
	case nas.SecurityHeaderTypeIntegrityProtectedAndCipheredWithNew5gNasSecurityContext:
		ciphered = true

		cnt = 0
	default:
		return nil, fmt.Errorf("wrong security header type: 0x%0x", msg.SecurityHeaderType)
	}

	cnt = cnt.ReconcileUplink(sequenceNumber)

	mac32, err := security.NASMacCalculate(ue.integrityAlg, ue.knasInt, cnt.Value(), security.Bearer3GPP,
		security.DirectionUplink, payload)
	if err != nil {
		return nil, fmt.Errorf("error calculating mac: %+v", err)
	}

	macVerified := hmac.Equal(mac32, receivedMac32)
	if !macVerified {
		logger.AmfLog.Warn("NAS MAC verification failed")
	}

	if ciphered {
		logger.AmfLog.Debug("Decrypt NAS message", zap.Uint8("algorithm", ue.cipheringAlg), zap.Uint32("ULCount", cnt.Value()))

		if err = security.NASEncrypt(ue.cipheringAlg, ue.knasEnc, cnt.Value(), security.Bearer3GPP,
			security.DirectionUplink, payload[1:]); err != nil {
			return nil, fmt.Errorf("error encrypting: %+v", err)
		}
	}

	payload = payload[1:]
	if err := msg.PlainNasDecode(&payload); err != nil {
		return nil, err
	}

	msgType := msg.GmmHeader.GetMessageType()

	if classifyNasPdu(msgType, msg.SecurityHeaderType, macVerified) == VerdictReject {
		return nil, fmt.Errorf("mac verification failed for the nas message: %v", msgType)
	}

	// TS 24.501: once secure exchange is established, a message failing
	// the integrity check is discarded.
	if conn.SecureExchangeEstablished() && !macVerified {
		return nil, fmt.Errorf("nas message discarded: integrity check failed after secure exchange established (TS 24.501)")
	}

	if macVerified {
		ue.ulCount = cnt

		// First verified message establishes secure exchange on the connection (TS 24.501).
		conn.MarkSecureExchangeEstablished()
	}

	return &DecodeResult{Message: msg, IntegrityVerified: macVerified}, nil
}
