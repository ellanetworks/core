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
	VerdictReject Verdict = iota
	VerdictIntegrityVerified
	// VerdictPlainAllowed: plain NAS with a message type on the TS 24.501 whitelist.
	VerdictPlainAllowed
	// VerdictMacFailedAllowed: integrity-protected with a failed MAC, message type on
	// the TS 33.501 whitelist.
	VerdictMacFailedAllowed
)

// DecodeResult is the output of the pure NAS decoder. A rejected PDU is returned as
// a decode error (never a result), so a result is always processable; IntegrityVerified
// reports whether its MAC was verified — false for a plain or MAC-failed message admitted
// before secure exchange per TS 24.501 §4.4.4.3.
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

// macFailedAllowed reports whether a message type may be processed after an
// integrity-check failure (TS 24.501 §4.4.4.3, TS 33.501). SERVICE REQUEST, on the
// spec's list, is absent: it is verified before context binding by the dedicated
// HandleServiceRequest and rejected with cause #9 on failure, never admitted here.
func macFailedAllowed(msgType uint8) bool {
	return plainNasAllowed(msgType)
}
