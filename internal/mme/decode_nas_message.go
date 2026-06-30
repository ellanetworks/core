// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/nas/eps"
	"go.uber.org/zap"
)

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
		return nil, fmt.Errorf("nas payload is empty")
	}

	// Secure exchange is tracked per NAS signalling connection (TS 24.301
	// §4.4.4.3); ue.secured is the separate per-UE "has a security context"
	// notion used by handover/path-switch.
	conn := ue.S1
	connSecured := conn != nil && conn.SecureExchangeEstablished()

	securityHeader := nas[0] >> 4

	if securityHeader == uint8(eps.SHTPlain) {
		mt, err := eps.PeekMessageType(nas)
		if err != nil {
			logger.MmeLog.Warn("failed to read EMM message type", zap.Error(err))
			return nil, fmt.Errorf("read EMM message type: %w", err)
		}

		// TS 24.301 §4.4.4.3: once secure exchange is established on the
		// connection, a message that is not integrity protected is discarded, so a
		// forged plain NAS message cannot disrupt an authenticated UE.
		if connSecured {
			logger.MmeLog.Warn("discarding plain NAS message: secure exchange already established",
				zap.String("imsi", ue.IMSI()))

			return nil, fmt.Errorf("plain NAS discarded: secure exchange established")
		}

		if classifyNasPdu(mt, securityHeader, false) != verdictPlainAllowed {
			logger.MmeLog.Warn("discarding plain NAS message not permitted without integrity (TS 24.301 §4.4.4.3)",
				zap.String("message", EmmMessageTypeName(mt)))

			return nil, fmt.Errorf("plain NAS %s not permitted", EmmMessageTypeName(mt))
		}

		return &DecodeResult{Plain: nas}, nil
	}

	if len(nas) < 6 {
		logger.MmeLog.Warn("security-protected NAS message too short")
		return nil, fmt.Errorf("protected NAS message too short")
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
	// readable even under a null-cipher security header, so it is checked before
	// the type peek below (which a genuinely ciphered body would defeat). DETACH
	// REQUEST is on the no-integrity whitelist, so it dispatches via the
	// WithoutIntegrity path.
	if !connSecured && isSwitchOffDetach(body) {
		return &DecodeResult{Plain: body}, nil
	}

	if connSecured {
		logger.MmeLog.Warn("discarding NAS message: integrity check failed after secure exchange established",
			zap.String("imsi", ue.IMSI()))

		return nil, fmt.Errorf("NAS discarded: integrity failed after secure exchange")
	}

	// The plaintext type is readable only for an integrity-only (unciphered)
	// security header (types 1 and 3); a ciphered body peeks to a meaningless type,
	// so such a message is dropped.
	if securityHeader != uint8(eps.SHTIntegrityProtected) && securityHeader != uint8(eps.SHTIntegrityProtectedNewContext) {
		logger.MmeLog.Warn("NAS integrity check failed",
			zap.Error(err),
			zap.Uint8("security-header-type", securityHeader),
			zap.Bool("has-security-context", ue.HasKASME()))

		return nil, fmt.Errorf("NAS integrity check failed (ciphered, unreadable): %w", err)
	}

	mt, perr := eps.PeekMessageType(body)
	if perr != nil {
		logger.MmeLog.Warn("NAS integrity check failed; unreadable message type", zap.Error(err))
		return nil, fmt.Errorf("NAS integrity check failed; unreadable type: %w", err)
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

		return nil, fmt.Errorf("NAS integrity check failed: %s not whitelisted", EmmMessageTypeName(mt))
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
