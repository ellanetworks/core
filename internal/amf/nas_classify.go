// SPDX-FileCopyrightText: Ella Networks Inc.
//
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"github.com/free5gc/nas"
)

// Verdict is the result of classifying a NAS PDU against the current UE
// security state.
type Verdict int

const (
	// VerdictReject means the PDU must be dropped.
	VerdictReject Verdict = iota
	// VerdictIntegrityVerified means the PDU was integrity-protected and its MAC verified.
	VerdictIntegrityVerified
	// VerdictPlainAllowed means the PDU was plain NAS with a message type on the TS 24.501 whitelist.
	VerdictPlainAllowed
	// VerdictMacFailedAllowed means the PDU was integrity-protected, its MAC failed,
	// and its message type is on the TS 33.501 whitelist.
	VerdictMacFailedAllowed
)

// DecodeResult is the output of the pure NAS decoder. A rejected PDU is returned as
// a decode error (never a result), so a result is always processable; IntegrityVerified
// reports whether its MAC was verified — false for a plain or MAC-failed message admitted
// before secure exchange per TS 24.501 §4.4.4.3. Mirrors the MME's DecodeResult.
type DecodeResult struct {
	Message           *nas.Message
	IntegrityVerified bool
}

// classifyNasPdu decides whether a NAS PDU of the given type, security
// header, and MAC-verification state may be processed.
//
// macVerified has meaning only when the PDU is integrity-protected:
//   - true  → MAC verified OK
//   - false → MAC verification failed
//
// For plain NAS (security header == PlainNas) macVerified is ignored.
func classifyNasPdu(msgType, securityHeader uint8, macVerified bool) Verdict {
	if securityHeader == nas.SecurityHeaderTypePlainNas {
		if plainNasAllowed(msgType) {
			return VerdictPlainAllowed
		}

		return VerdictReject
	}

	if macVerified {
		return VerdictIntegrityVerified
	}

	if macFailedAllowed(msgType) {
		return VerdictMacFailedAllowed
	}

	return VerdictReject
}

// plainNasAllowed reports whether a NAS message type may be processed
// without integrity protection per TS 24.501.
func plainNasAllowed(msgType uint8) bool {
	switch msgType {
	case nas.MsgTypeRegistrationRequest,
		nas.MsgTypeIdentityResponse,
		nas.MsgTypeAuthenticationResponse,
		nas.MsgTypeAuthenticationFailure,
		nas.MsgTypeSecurityModeReject,
		nas.MsgTypeDeregistrationRequestUEOriginatingDeregistration,
		nas.MsgTypeDeregistrationAcceptUETerminatedDeregistration:
		return true
	default:
		return false
	}
}

// macFailedAllowed reports whether a NAS message type may be processed
// after MAC verification failure per TS 24.501 §4.4.4.3. CONTROL PLANE
// SERVICE REQUEST is on the spec's MAC-failed list but is a CIoT
// control-plane procedure Ella Core does not implement, so it has no
// message-type constant to admit here.
// macFailedAllowed reports whether a message type is processed when the integrity check
// fails (TS 24.501 §4.4.4.3 / TS 33.501). SERVICE REQUEST is deliberately absent: it is
// resolved-or-rejected by the dedicated pre-context HandleServiceRequest (which verifies
// integrity before binding and rejects #9 on failure), never through this classify path —
// mirrors the MME, whose macFailedAllowed also omits SERVICE REQUEST.
func macFailedAllowed(msgType uint8) bool {
	return plainNasAllowed(msgType)
}
