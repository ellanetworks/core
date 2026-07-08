// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// Modified by Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"crypto/hmac"
	"errors"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/nasreply"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/security"
	"go.uber.org/zap"
)

// decodeError couples a decode or classify failure to the nasreply.Disposition the ingress
// finalizer must apply, so a NAS PDU the AMF cannot process draws the STATUS the spec mandates
// or an audited silence — never a bare drop.
type decodeError struct {
	disposition nasreply.Disposition
	detail      string
}

func (e *decodeError) Error() string { return e.detail }

// DispositionForDecodeError returns the disposition the decode layer attached to err. Any
// other error (e.g. a plain free5gc decode error not yet classified) resolves to an audited
// silent discard, so an unexpected failure fails safe rather than replying blindly.
func DispositionForDecodeError(err error) nasreply.Disposition {
	if de, ok := errors.AsType[*decodeError](err); ok {
		return de.disposition
	}

	return nasreply.Silent(nasreply.ReasonUnspecified)
}

func silentDecode(reason nasreply.Reason, format string, args ...any) error {
	return &decodeError{disposition: nasreply.Silent(reason), detail: fmt.Sprintf(format, args...)}
}

func statusDecode(cause uint8, format string, args ...any) error {
	return &decodeError{disposition: nasreply.StatusMM(cause), detail: fmt.Sprintf(format, args...)}
}

// DecodeNASMessage parses a 5GS NAS PDU (plain or security-protected) and
// returns the decoded message together with a policy Verdict. The only UE
// mutation performed here is advancing ue.ULCount (TS 24.501, TS 33.501).
func DecodeNASMessage(ue *UeContext, payload []byte) (*DecodeResult, error) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	if payload == nil {
		return nil, silentDecode(nasreply.ReasonTooShort, "nas payload is empty")
	}

	if len(payload) < 2 {
		return nil, silentDecode(nasreply.ReasonTooShort, "nas payload is too short")
	}

	msg := new(nas.Message)
	msg.SecurityHeaderType = nas.GetSecurityHeaderType(payload) & 0x0f

	conn := ue.Conn()

	if msg.SecurityHeaderType == nas.SecurityHeaderTypePlainNas {
		// TS 24.501: once secure NAS exchange is established for the
		// connection, a message that is not integrity protected is discarded.
		if conn.SecureExchangeEstablished() {
			return nil, silentDecode(nasreply.ReasonIntegrityFail, "plain NAS discarded: secure exchange established (TS 24.501 §4.4.4.3)")
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
	// PlainNasDecode consumes payload; capture whether the message-type octet was present so
	// a too-short PDU (§7.2.1, silent) is told apart from a decodable type whose body is
	// malformed (§7.5.1, 5GMM STATUS #96).
	typeReadable := len(payload) >= 3

	if err := msg.PlainNasDecode(&payload); err != nil {
		if !typeReadable {
			return nil, silentDecode(nasreply.ReasonTooShort, "plain NAS too short to classify: %v", err)
		}

		return nil, statusDecode(nasreply.CauseInvalidMandatoryInfo, "plain NAS decode failed: %v", err)
	}

	if msg.GmmMessage == nil {
		return nil, silentDecode(nasreply.ReasonOutOfState, "plain NAS message has no GMM body")
	}

	msgType := msg.GmmHeader.GetMessageType()

	if classifyNasPdu(msgType, nas.SecurityHeaderTypePlainNas, false) == VerdictReject {
		return nil, silentDecode(nasreply.ReasonIntegrityFail, "plain NAS message type %d not permitted by TS 24.501 §4.4.4.3", msgType)
	}

	return &DecodeResult{Message: msg, IntegrityVerified: false}, nil
}

func decodeProtectedNAS(ue *UeContext, msg *nas.Message, payload []byte, conn *UeConn) (*DecodeResult, error) {
	if len(payload) < 7 {
		return nil, silentDecode(nasreply.ReasonTooShort, "nas payload is too short")
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
		return nil, silentDecode(nasreply.ReasonUnspecified, "wrong security header type: 0x%0x", msg.SecurityHeaderType)
	}

	cnt = cnt.ReconcileUplink(sequenceNumber)

	mac32, err := security.NASMacCalculate(ue.integrityAlg, ue.knasInt, cnt.Value(), security.Bearer3GPP,
		security.DirectionUplink, payload)
	if err != nil {
		return nil, silentDecode(nasreply.ReasonUnspecified, "error calculating mac: %+v", err)
	}

	macVerified := hmac.Equal(mac32, receivedMac32)
	if !macVerified {
		logger.AmfLog.Warn("NAS MAC verification failed")
	}

	if ciphered {
		logger.AmfLog.Debug("Decrypt NAS message", zap.Uint8("algorithm", ue.cipheringAlg), zap.Uint32("ULCount", cnt.Value()))

		if err = security.NASEncrypt(ue.cipheringAlg, ue.knasEnc, cnt.Value(), security.Bearer3GPP,
			security.DirectionUplink, payload[1:]); err != nil {
			return nil, silentDecode(nasreply.ReasonUnspecified, "error encrypting: %+v", err)
		}
	}

	payload = payload[1:]
	if err := msg.PlainNasDecode(&payload); err != nil {
		// A malformed body under a verified MAC is a protocol error the sender can act on
		// (5GMM STATUS #96); under an unverified MAC it is indistinguishable from garbage,
		// so it is discarded silently (TS 24.501 §4.4.4.3).
		if macVerified {
			return nil, statusDecode(nasreply.CauseInvalidMandatoryInfo, "protected NAS decode failed: %v", err)
		}

		return nil, silentDecode(nasreply.ReasonIntegrityFail, "protected NAS decode failed under unverified MAC: %v", err)
	}

	msgType := msg.GmmHeader.GetMessageType()

	if classifyNasPdu(msgType, msg.SecurityHeaderType, macVerified) == VerdictReject {
		return nil, silentDecode(nasreply.ReasonIntegrityFail, "mac verification failed for the nas message: %v", msgType)
	}

	// TS 24.501: once secure exchange is established, a message failing
	// the integrity check is discarded.
	if conn.SecureExchangeEstablished() && !macVerified {
		return nil, silentDecode(nasreply.ReasonIntegrityFail, "nas message discarded: integrity check failed after secure exchange established (TS 24.501 §4.4.4.3)")
	}

	if macVerified {
		ue.ulCount = cnt

		// First verified message establishes secure exchange on the connection (TS 24.501).
		conn.MarkSecureExchangeEstablished()
	}

	return &DecodeResult{Message: msg, IntegrityVerified: macVerified}, nil
}
