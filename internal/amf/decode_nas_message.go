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

	"github.com/free5gc/nas"
	"github.com/free5gc/nas/security"
	"go.uber.org/zap"
)

// DecodeNASMessage parses a 5GS NAS PDU (plain or security-protected) and
// returns the decoded message together with a policy Verdict. The caller
// dispatches to a GMM handler based on the verdict. The only ue mutation
// performed here is advancing ue.Current().ULCount.
//
// See TS 24.501 §4.4.4.3 and TS 33.501 §6.4.6 step 3 for the policy.
func DecodeNASMessage(ue *AmfUe, payload []byte) (*DecodeResult, error) {
	ue.Mutex.Lock()
	defer ue.Mutex.Unlock()

	if payload == nil {
		return nil, fmt.Errorf("nas payload is empty")
	}

	if len(payload) < 2 {
		return nil, fmt.Errorf("nas payload is too short")
	}

	msg := new(nas.Message)
	msg.SecurityHeaderType = nas.GetSecurityHeaderType(payload) & 0x0f

	conn := ue.NasConn()

	if msg.SecurityHeaderType == nas.SecurityHeaderTypePlainNas {
		// TS 24.501 §4.4.4.3: once secure exchange of NAS messages is
		// established for the connection, a message that is not integrity
		// protected is discarded.
		if conn != nil && conn.secureExchangeEstablished {
			return nil, fmt.Errorf("plain NAS message discarded: secure exchange already established (TS 24.501 §4.4.4.3)")
		}

		return decodePlainNAS(msg, payload)
	}

	return decodeProtectedNAS(ue, msg, payload, conn)
}

// NasIntegrityVerified reports whether payload is an integrity-protected NAS
// PDU whose MAC verifies against this UE's current 5G NAS security context. It
// does not mutate any UE state: the uplink count is evaluated on a copy. A
// plain NAS PDU, a PDU that fails the MAC check, or the absence of a usable
// security context all return false.
//
// It is the authorization gate for an inbound message that resolved to an
// existing context by GUTI/5G-S-TMSI: only a message proven to originate from
// the holder of the keys may act on that context (TS 24.501 §4.4.4.3).
func (ue *AmfUe) NasIntegrityVerified(payload []byte) bool {
	if ue == nil {
		return false
	}

	ue.Mutex.RLock()
	defer ue.Mutex.RUnlock()

	fc := ue.current.Load()
	if fc == nil || !fc.SecurityContextAvailable {
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

	cnt := fc.ULCount // value copy; never committed back to the context
	if cnt.SQN() > sqn {
		cnt.SetOverflow(cnt.Overflow() + 1)
	}

	cnt.SetSQN(sqn)

	mac, err := security.NASMacCalculate(fc.IntegrityAlg, fc.KnasInt, cnt.Get(), security.Bearer3GPP, security.DirectionUplink, payload[6:])
	if err != nil {
		return false
	}

	return hmac.Equal(mac, receivedMac)
}

// ReuseForInboundNAS reports whether an inbound NAS PDU that resolved to this
// committed context by GUTI/5G-S-TMSI may act on it: only when it is
// integrity-verified against the context. Any other message is processed on a
// fresh context, so context resolution never mutates a committed context — it
// either reuses a verified one or yields a fresh one, leaving the committed
// NAS security context and PDU sessions untouched (TS 24.501 §4.4.4.3). On a
// fresh context a registration re-authenticates, a service request is rejected
// with 5GMM cause #9, and a deregistration is ignored — each the spec outcome.
func (ue *AmfUe) ReuseForInboundNAS(payload []byte) bool {
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

	verdict := classifyNasPdu(msgType, nas.SecurityHeaderTypePlainNas, false)
	if verdict == VerdictReject {
		return nil, fmt.Errorf(
			"plain NAS message type %d not permitted by TS 24.501 §4.4.4.3",
			msgType,
		)
	}

	return &DecodeResult{Message: msg, Verdict: verdict}, nil
}

func decodeProtectedNAS(ue *AmfUe, msg *nas.Message, payload []byte, conn *ActiveNasConnection) (*DecodeResult, error) {
	if len(payload) < 7 {
		return nil, fmt.Errorf("nas payload is too short")
	}

	securityHeader := payload[0:6]
	sequenceNumber := payload[6]
	receivedMac32 := securityHeader[2:]
	// remove security Header except for sequence Number
	payload = payload[6:]

	ciphered := false

	// Work on a copy of the uplink count and commit it to the security context
	// only once the MAC is verified, so an unauthenticated message cannot
	// advance (desync) the count of a genuine UE (TS 33.501 §6.4.3).
	cnt := ue.Current().ULCount

	switch msg.SecurityHeaderType {
	case nas.SecurityHeaderTypeIntegrityProtected:
	case nas.SecurityHeaderTypeIntegrityProtectedAndCiphered:
		ciphered = true
	case nas.SecurityHeaderTypeIntegrityProtectedAndCipheredWithNew5gNasSecurityContext:
		ciphered = true

		cnt.Set(0, 0)
	default:
		return nil, fmt.Errorf("wrong security header type: 0x%0x", msg.SecurityHeaderType)
	}

	if cnt.SQN() > sequenceNumber {
		ue.Log.Debug("set ULCount overflow")
		cnt.SetOverflow(cnt.Overflow() + 1)
	}

	cnt.SetSQN(sequenceNumber)

	mac32, err := security.NASMacCalculate(ue.Current().IntegrityAlg, ue.Current().KnasInt, cnt.Get(), security.Bearer3GPP,
		security.DirectionUplink, payload)
	if err != nil {
		return nil, fmt.Errorf("error calculating mac: %+v", err)
	}

	macVerified := hmac.Equal(mac32, receivedMac32)
	if !macVerified {
		ue.Log.Warn("NAS MAC verification failed")
	}

	if ciphered {
		ue.Log.Debug("Decrypt NAS message", zap.Uint8("algorithm", ue.Current().CipheringAlg), zap.Uint32("ULCount", cnt.Get()))

		if err = security.NASEncrypt(ue.Current().CipheringAlg, ue.Current().KnasEnc, cnt.Get(), security.Bearer3GPP,
			security.DirectionUplink, payload[1:]); err != nil {
			return nil, fmt.Errorf("error encrypting: %+v", err)
		}
	}

	// remove sequence number
	payload = payload[1:]
	if err := msg.PlainNasDecode(&payload); err != nil {
		return nil, err
	}

	msgType := msg.GmmHeader.GetMessageType()

	verdict := classifyNasPdu(msgType, msg.SecurityHeaderType, macVerified)
	if verdict == VerdictReject {
		return nil, fmt.Errorf("mac verification failed for the nas message: %v", msgType)
	}

	// TS 24.501 §4.4.4.3: once secure exchange is established, a message failing
	// the integrity check is discarded rather than processed.
	if conn != nil && conn.secureExchangeEstablished && !macVerified {
		return nil, fmt.Errorf("nas message discarded: integrity check failed after secure exchange established (TS 24.501 §4.4.4.3)")
	}

	if macVerified {
		ue.Current().ULCount = cnt

		// A successfully integrity-checked message marks secure exchange of NAS
		// messages as established for this connection (TS 24.501 §4.4.4.3).
		if conn != nil {
			conn.secureExchangeEstablished = true
		}
	}

	return &DecodeResult{Message: msg, Verdict: verdict}, nil
}
