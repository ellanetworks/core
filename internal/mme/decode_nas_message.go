// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"errors"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/nasreply"
	"github.com/ellanetworks/core/nas/eps"
	"go.uber.org/zap"
)

// decodeError couples a decode or classify failure to the nasreply.Disposition the ingress
// finalizer must apply, so a NAS PDU the MME cannot process draws an audited silence — never a
// bare drop. The decoder only peeks the message type, so every decode failure resolves to a
// silence; a malformed-but-typed message is caught (and answered) by its handler.
type decodeError struct {
	disposition nasreply.Disposition
	detail      string
}

func (e *decodeError) Error() string { return e.detail }

// DispositionForDecodeError returns the disposition the decode layer attached to err, or an
// audited silent discard for any other error, so an unexpected failure fails safe.
func DispositionForDecodeError(err error) nasreply.Disposition {
	if de, ok := errors.AsType[*decodeError](err); ok {
		return de.disposition
	}

	return nasreply.Silent(nasreply.ReasonUnspecified)
}

func silentDecode(reason nasreply.Reason, format string, args ...any) error {
	return &decodeError{disposition: nasreply.Silent(reason), detail: fmt.Sprintf(format, args...)}
}

// DecodeResult is the outcome of decoding an inbound EMM NAS PDU: the plaintext
// body to dispatch and how it was authorized.
type DecodeResult struct {
	Plain []byte
	// IntegrityVerified is true when the PDU's MAC verified against the UE's
	// security context. A false value covers both a plain whitelisted PDU and one
	// accepted without a verifiable MAC before secure exchange (TS 24.301
	// §4.4.4.3); the caller authenticates the subscriber before progressing.
	IntegrityVerified bool
}

// DecodeNASMessage unwraps NAS security on an inbound EMM PDU and classifies it
// against TS 24.301 §4.4.4.3, returning a DecodeResult or an error (already
// logged) when the PDU must be dropped. The only UE state it mutates is the
// uplink NAS COUNT (committed only on a verified MAC) and the connection's
// secure-exchange flag.
func DecodeNASMessage(ue *UeContext, nas []byte) (*DecodeResult, error) {
	if len(nas) < 1 {
		return nil, silentDecode(nasreply.ReasonTooShort, "nas payload is empty")
	}

	// Secure exchange is tracked per NAS signalling connection (TS 24.301
	// §4.4.4.3); ue.secured is the separate per-UE "has a security context"
	// notion used by handover/path-switch.
	conn := ue.Conn()
	connSecured := conn != nil && conn.SecureExchangeEstablished()

	securityHeader := nas[0] >> 4

	if securityHeader == uint8(eps.SHTPlain) {
		mt, err := eps.PeekMessageType(nas)
		if err != nil {
			logger.MmeLog.Warn("failed to read EMM message type", zap.Error(err))
			return nil, silentDecode(nasreply.ReasonTooShort, "read EMM message type: %v", err)
		}

		// TS 24.301 §4.4.4.3: once secure exchange is established on the
		// connection, a message that is not integrity protected is discarded, so a
		// forged plain NAS message cannot disrupt an authenticated UE.
		if connSecured {
			logger.MmeLog.Warn("discarding plain NAS message: secure exchange already established",
				zap.String("imsi", ue.IMSI()))

			return nil, silentDecode(nasreply.ReasonIntegrityFail, "plain NAS discarded: secure exchange established (TS 24.301 §4.4.4.3)")
		}

		if classifyNasPdu(mt, securityHeader, false) != verdictPlainAllowed {
			logger.MmeLog.Warn("discarding plain NAS message not permitted without integrity (TS 24.301 §4.4.4.3)",
				zap.String("message", EmmMessageTypeName(mt)))

			return nil, silentDecode(nasreply.ReasonIntegrityFail, "plain NAS %s not permitted (TS 24.301 §4.4.4.3)", EmmMessageTypeName(mt))
		}

		return &DecodeResult{Plain: nas}, nil
	}

	if len(nas) < 6 {
		logger.MmeLog.Warn("security-protected NAS message too short")
		return nil, silentDecode(nasreply.ReasonTooShort, "protected NAS message too short")
	}

	// Verify against the UE's security context. Replay protection: a stale or
	// replayed message estimates to a NAS COUNT whose MAC fails to verify, so it
	// is dropped (TS 24.301).
	p, count, err := ue.TryUnprotectUplink(nas)
	if err == nil {
		ue.CommitUplinkCount(count)

		// First verified message establishes secure exchange on the connection (TS 24.301 §4.4.4.3).
		if conn != nil {
			conn.MarkSecureExchangeEstablished()
		}

		return &DecodeResult{Plain: p, IntegrityVerified: true}, nil
	}

	body := nas[6:]

	// A switch-off DETACH REQUEST is honoured without integrity protection only
	// before secure exchange is established (TS 24.301 §4.4.4.3). Its body is
	// readable even under a null-cipher security header, so it is checked here,
	// ahead of the ciphered type peek that a genuinely ciphered body would defeat.
	if !connSecured && isSwitchOffDetach(body) {
		return &DecodeResult{Plain: body}, nil
	}

	if connSecured {
		logger.MmeLog.Warn("discarding NAS message: integrity check failed after secure exchange established",
			zap.String("imsi", ue.IMSI()))

		return nil, silentDecode(nasreply.ReasonIntegrityFail, "NAS discarded: integrity failed after secure exchange (TS 24.301 §4.4.4.3)")
	}

	// The plaintext type is readable only for an integrity-only (unciphered)
	// security header (types 1 and 3); a ciphered body peeks to a meaningless type,
	// so such a message is dropped.
	if securityHeader != uint8(eps.SHTIntegrityProtected) && securityHeader != uint8(eps.SHTIntegrityProtectedNewContext) {
		logger.MmeLog.Warn("NAS integrity check failed",
			zap.Error(err),
			zap.Uint8("security-header-type", securityHeader),
			zap.Bool("has-security-context", ue.HasKASME()))

		return nil, silentDecode(nasreply.ReasonIntegrityFail, "NAS integrity check failed (ciphered, unreadable): %v", err)
	}

	mt, perr := eps.PeekMessageType(body)
	if perr != nil {
		logger.MmeLog.Warn("NAS integrity check failed; unreadable message type", zap.Error(err))
		return nil, silentDecode(nasreply.ReasonIntegrityFail, "NAS integrity check failed; unreadable type: %v", err)
	}

	// TS 24.301 §4.4.4.3: certain EMM messages are processed even when the MAC
	// fails, but only before secure exchange is established (no usable security
	// context, e.g. a fresh context after an MME restart). The subscriber is
	// authenticated before the procedure is progressed.
	if classifyNasPdu(mt, securityHeader, false) != verdictMacFailedAllowed {
		logger.MmeLog.Warn("NAS integrity check failed",
			zap.Error(err),
			zap.String("attempted-message", EmmMessageTypeName(mt)),
			zap.Uint8("security-header-type", securityHeader),
			zap.Uint32("expected-ul-count", ue.ULCount()),
			zap.Uint8("integrity-alg", ue.EIA()),
			zap.Bool("has-security-context", ue.HasKASME()))

		return nil, silentDecode(nasreply.ReasonIntegrityFail, "NAS integrity check failed: %s not whitelisted", EmmMessageTypeName(mt))
	}

	return &DecodeResult{Plain: body}, nil
}

// isSwitchOffDetach reports whether body is a plain UE-originating DETACH REQUEST
// with the switch-off flag set (TS 24.301 §5.5.2.2.1), readable even under a
// null-cipher security header.
func isSwitchOffDetach(body []byte) bool {
	if mt, err := eps.PeekMessageType(body); err != nil || mt != eps.MsgDetachRequest {
		return false
	}

	req, err := eps.ParseDetachRequestUE(body)

	return err == nil && req.SwitchOff
}
