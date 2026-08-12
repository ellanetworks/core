// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// Modified by Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"errors"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/nasreply"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
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
// other error (one not yet classified into a disposition) resolves to an audited silent
// discard, so an unexpected failure fails safe rather than replying blindly.
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

// DecodeNASMessage parses a 5GS NAS PDU (plain or security-protected), rejecting a
// PDU not admissible in the current security state as a decode error. The only UE
// mutation performed here is advancing ue.ULCount (TS 24.501, TS 33.501).
func DecodeNASMessage(ue *UeContext, payload []byte) (*DecodeResult, error) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	if payload == nil {
		return nil, silentDecode(nasreply.ReasonTooShort, "nas payload is empty")
	}

	securityHeaderType, err := fgs.PeekSecurityHeaderType(payload)
	if err != nil {
		return nil, silentDecode(nasreply.ReasonTooShort, "nas payload is too short")
	}

	conn := ue.Conn()

	if securityHeaderType == fgs.SHTPlain {
		if conn.SecureExchangeEstablished() {
			// TS 24.501 §4.4.4.3: a plain message received after secure exchange is
			// discarded — but only a real, decodable NAS message (a genuine integrity
			// violation). A plain PDU that does not decode to a valid message is a protocol
			// error, answered with a 5GMM STATUS #111 (§7), not silently ignored. Neither
			// path processes the message, so integrity protection is not weakened.
			if _, _, err := DecodePlainGmm(payload); err != nil {
				return nil, statusDecode(nasreply.CauseProtocolErrorUnspecified, "undecodable plain NAS after secure exchange")
			}

			return nil, silentDecode(nasreply.ReasonIntegrityFail, "plain NAS discarded: secure exchange established (TS 24.501 §4.4.4.3)")
		}

		return decodePlainNAS(payload)
	}

	return decodeProtectedNAS(ue, securityHeaderType, payload, conn)
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

	// Without an installed security context there is nothing to verify against:
	// the algorithm fields of an uninstalled context name NIA0, under which an
	// all-zero MAC would verify (TS 33.501 §5.5.2).
	if !ue.secured || ue.sc == nil {
		return false
	}

	spm, err := fgs.ParseSecurityProtectedMessage(payload)
	if err != nil {
		return false
	}

	switch spm.SecurityHeaderType {
	case fgs.SHTIntegrityProtected, fgs.SHTIntegrityProtectedCiphered:
	default:
		return false
	}

	// Never committed back to the context: this only asks whether the message
	// would verify. An exhausted count can verify nothing.
	cnt, err := ue.ulCount.Estimate(spm.SequenceNumber)
	if err != nil {
		return false
	}

	_, _, err = fgs.Unprotect(payload, cnt, nas.DirectionUplink, ue.sc,
		fgs.SHTIntegrityProtected, fgs.SHTIntegrityProtectedCiphered)

	return err == nil
}

// ReuseForInboundNAS reports whether an inbound NAS PDU that resolved to this
// committed context by GUTI/5G-S-TMSI may act on it: only when integrity-verified.
// Any other message is processed on a fresh context, leaving the committed
// security context and PDU sessions untouched (TS 24.501).
func (ue *UeContext) ReuseForInboundNAS(payload []byte) bool {
	return ue.NasIntegrityVerified(payload)
}

// GmmDecodeFailureCause maps a plain-NAS decode failure to the 5GMM STATUS cause
// the sender is told: #97 when the message type is one the AMF does not define
// (TS 24.501 §7.4), otherwise #96 for a defined type whose body is malformed
// (§7.5.1). body is the inner plain NAS message; its message type is the third
// octet, absent on a too-short PDU.
func GmmDecodeFailureCause(body []byte) uint8 {
	if len(body) >= 3 && !gmmTypeDefined(body[2]) {
		return nasreply.CauseMessageTypeNotImplemented
	}

	return nasreply.CauseInvalidMandatoryInfo
}

// DecodePlainGmm validates a plain NAS message and reports its 5GMM message
// type: it errors on an empty body, a disallowed extended protocol
// discriminator, an unassigned 5GMM message type, or an Ella-parsed uplink type
// whose mandatory content is malformed. isGMM is false for a standalone 5GSM
// message (EPD 0x2E), which the caller discards.
func DecodePlainGmm(body []byte) (msgType uint8, isGMM bool, err error) {
	if len(body) == 0 {
		return 0, false, fmt.Errorf("empty NAS message")
	}

	epd := fgs.ProtocolDiscriminator(body[0])
	if epd != fgs.EPD5GMM && epd != fgs.EPD5GSM {
		return 0, false, fmt.Errorf("extended protocol discriminator %#x not allowed in NAS message", uint8(epd))
	}

	if len(body) < 3 {
		return 0, epd == fgs.EPD5GMM, fmt.Errorf("NAS message too short")
	}

	msgType = body[2]
	isGMM = epd == fgs.EPD5GMM

	if !isGMM {
		// A standalone 5GSM message is not processed on N1; the caller drops it. Its
		// body is not validated here.
		return msgType, false, nil
	}

	if _, ok := gmmMessageTypeNames[msgType]; !ok {
		return msgType, true, fmt.Errorf("unassigned 5GMM message type %#x", msgType)
	}

	return msgType, true, parseCheckGmm(msgType, body)
}

// parseCheckGmm validates the mandatory content of the uplink 5GMM messages Ella
// parses; a defined type without a dedicated parser (downlink types, header-only
// uplink types) is accepted on its already-validated header.
//
// A syntactically incorrect optional element is not a decode failure: TS 24.501
// §7.7.1 has the receiver treat it as absent and process the message, and the
// codec marks the elements a security decision reads Critical so those still
// fail hard. Rejecting the message here would keep a UE from registering
// because of an element the parser merely disliked.
func parseCheckGmm(msgType uint8, body []byte) error {
	var err error

	switch fgs.MessageType(msgType) {
	case fgs.MsgRegistrationRequest:
		_, err = fgs.ParseRegistrationRequest(body)
	case fgs.MsgServiceRequest:
		_, err = fgs.ParseServiceRequest(body)
	case fgs.MsgDeregistrationRequestUEOrig:
		_, err = fgs.ParseDeregistrationRequestUEOriginating(body)
	case fgs.MsgIdentityResponse:
		_, err = fgs.ParseIdentityResponse(body)
	case fgs.MsgAuthenticationResponse:
		_, err = fgs.ParseAuthenticationResponse(body)
	case fgs.MsgAuthenticationFailure:
		_, err = fgs.ParseAuthenticationFailure(body)
	case fgs.MsgSecurityModeComplete:
		_, err = fgs.ParseSecurityModeComplete(body)
	case fgs.MsgSecurityModeReject:
		_, err = fgs.ParseSecurityModeReject(body)
	case fgs.MsgGMMStatus:
		_, err = fgs.ParseGMMStatus(body)
	case fgs.MsgNotificationResponse:
		_, err = fgs.ParseNotificationResponse(body)
	case fgs.MsgULNASTransport:
		_, err = fgs.ParseULNASTransport(body)
	}

	if err != nil && nas.SoftOnly(err) {
		return nil
	}

	return err
}

func decodePlainNAS(payload []byte) (*DecodeResult, error) {
	// Capture whether the message-type octet is present so a too-short PDU (§7.2.1, silent)
	// is told apart from a decodable type whose body is malformed (§7.5.1, 5GMM STATUS #96).
	typeReadable := len(payload) >= 3

	msgType, isGMM, err := DecodePlainGmm(payload)
	if err != nil {
		if !typeReadable {
			return nil, silentDecode(nasreply.ReasonTooShort, "plain NAS too short to classify: %v", err)
		}

		return nil, statusDecode(GmmDecodeFailureCause(payload), "plain NAS decode failed: %v", err)
	}

	if !isGMM {
		return nil, silentDecode(nasreply.ReasonOutOfState, "plain NAS message has no GMM body")
	}

	if !plainNasAllowed(msgType) {
		return nil, silentDecode(nasreply.ReasonIntegrityFail, "plain NAS message type %d not permitted by TS 24.501 §4.4.4.3", msgType)
	}

	return &DecodeResult{MessageType: msgType, IsGMM: true, IntegrityVerified: false, ArrivedPlain: true, Plain: payload}, nil
}

func decodeProtectedNAS(ue *UeContext, headerType fgs.SecurityHeaderType, payload []byte, conn *UeConn) (*DecodeResult, error) {
	spm, err := fgs.ParseSecurityProtectedMessage(payload)
	if err != nil {
		return nil, silentDecode(nasreply.ReasonTooShort, "nas payload is too short")
	}

	// A protected message can only be verified against an installed context. The
	// message is discarded rather than verified under whatever the algorithm
	// fields hold, which for an uninstalled context is NIA0 (TS 33.501 §5.5.2).
	if ue.sc == nil {
		return nil, silentDecode(nasreply.ReasonIntegrityFail, "protected NAS message for a UE with no security context")
	}

	sequenceNumber := spm.SequenceNumber

	// Work on a copy of the uplink counter and commit to the security context
	// only once the MAC is verified, so an unauthenticated message cannot
	// advance (desync) the count of a genuine UE (TS 33.501).
	counter := ue.ulCount

	switch headerType {
	case fgs.SHTIntegrityProtected, fgs.SHTIntegrityProtectedCiphered:
	case fgs.SHTIntegrityProtectedCipheredNewContext:
		// TS 24.501 §4.4.4.3 reserves this header type for the SECURITY MODE
		// COMPLETE answering a command in flight, so it is refused anywhere else:
		// accepting it later would let a replay of that captured message roll the
		// uplink count back to zero under unchanged keys, and every message
		// captured after it would replay in turn.
		//
		// No reset accompanies it. Building the command already reset both counts
		// (BuildSecurityModeCommand), so the genuine answer verifies at count zero, and
		// resetting on receipt is what made the rollback reachable.
		if ue.regStepLocked() != RegStepSecurityMode {
			return nil, silentDecode(nasreply.ReasonOutOfState,
				"new-context security header type outside the security mode procedure")
		}
	default:
		// A reserved/unrecognized security header type is not a valid NAS message: a protocol
		// error answered with a 5GMM STATUS #111 (§7), not silently ignored. The message is
		// never processed, so integrity protection is not weakened.
		return nil, statusDecode(nasreply.CauseProtocolErrorUnspecified, "wrong security header type: 0x%0x", uint8(headerType))
	}

	// An exhausted uplink count accepts nothing further under this security
	// context: wrapping would verify a replay of an already-accepted message
	// (TS 33.501 §6.4.3.1). The UE has to re-authenticate.
	cnt, cerr := counter.Estimate(sequenceNumber)
	if cerr != nil {
		return nil, silentDecode(nasreply.ReasonIntegrityFail, "uplink NAS COUNT exhausted: %v", cerr)
	}

	// The types accepted here are the ones the switch above admitted; passing them
	// again makes the wrapper enforce it rather than the caller.
	plain, _, uerr := fgs.Unprotect(payload, cnt, nas.DirectionUplink, ue.sc,
		fgs.SHTIntegrityProtected, fgs.SHTIntegrityProtectedCiphered, fgs.SHTIntegrityProtectedCipheredNewContext)
	if uerr == nil {
		// Before the commit so a discarded message does not advance the count, and
		// before MarkSecureExchangeEstablished so the initial NAS message of a new
		// connection stays outside the guard (TS 24.501 §4.4.5).
		if conn.SecureExchangeEstablished() && headerType == fgs.SHTIntegrityProtected && cipheringRequiredFor(plain) {
			logger.AmfLog.Warn("discarding unciphered NAS message received after ciphering started")

			return nil, silentDecode(nasreply.ReasonIntegrityFail, "NAS discarded: unciphered after ciphering started (TS 24.501 §4.4.5)")
		}

		// MAC verified: commit the estimated count and establish secure exchange on the
		// connection before dispatch, so a replay estimates to a count whose MAC fails
		// (TS 24.501 §4.4.4.3).
		_ = counter.Commit(cnt)
		ue.ulCount = counter

		conn.MarkSecureExchangeEstablished()

		msgType, isGMM, derr := DecodePlainGmm(plain)
		if derr != nil {
			// A malformed body under a verified MAC is a protocol error the sender can act on
			// (5GMM STATUS #96, or #97 for an undefined message type).
			return nil, statusDecode(GmmDecodeFailureCause(plain), "protected NAS decode failed: %v", derr)
		}

		return &DecodeResult{MessageType: msgType, IsGMM: isGMM, IntegrityVerified: true, Plain: plain}, nil
	}

	if !errors.Is(uerr, fgs.ErrMACMismatch) {
		return nil, silentDecode(nasreply.ReasonUnspecified, "error unprotecting nas message: %v", uerr)
	}

	logger.AmfLog.Warn("NAS MAC verification failed")

	// TS 24.501 §4.4.4.3: once secure exchange is established, a message failing
	// the integrity check is discarded.
	if conn.SecureExchangeEstablished() {
		return nil, silentDecode(nasreply.ReasonIntegrityFail, "nas message discarded: integrity check failed after secure exchange established (TS 24.501 §4.4.4.3)")
	}

	// The plaintext type is readable only for an integrity-only (unciphered)
	// security header; a ciphered body under a failed MAC is not deciphered, so
	// such a message is dropped.
	if headerType != fgs.SHTIntegrityProtected {
		return nil, silentDecode(nasreply.ReasonIntegrityFail, "mac verification failed for ciphered nas message")
	}

	body := spm.UnverifiedPayload

	msgType, isGMM, derr := DecodePlainGmm(body)
	if derr != nil {
		return nil, silentDecode(nasreply.ReasonIntegrityFail, "protected NAS decode failed under unverified MAC: %v", derr)
	}

	// An integrity-protected message with a failed MAC is admitted only for the
	// whitelisted types processed before secure exchange (TS 24.501 §4.4.4.3).
	if !plainNasAllowed(msgType) {
		return nil, silentDecode(nasreply.ReasonIntegrityFail, "mac verification failed for the nas message: %v", msgType)
	}

	return &DecodeResult{MessageType: msgType, IsGMM: isGMM, IntegrityVerified: false, Plain: body}, nil
}
