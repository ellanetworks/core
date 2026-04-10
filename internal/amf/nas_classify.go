// Copyright 2026 Ella Networks
//
// SPDX-License-Identifier: Apache-2.0

package amf

import (
	"github.com/free5gc/nas"
)

// Verdict is the result of classifying a NAS PDU against the current UE
// security state. It is the only thing permitted to answer the question
// "may this NAS PDU be handed to a handler?".
type Verdict int

const (
	// VerdictReject means the PDU must be dropped. Handlers must not run.
	VerdictReject Verdict = iota
	// VerdictIntegrityVerified means the PDU was integrity-protected and
	// its MAC verified successfully.
	VerdictIntegrityVerified
	// VerdictPlainAllowed means the PDU was plain NAS and its message
	// type is on the TS 24.501 §4.4.4.3 whitelist. Handlers run with
	// MacFailed=true.
	VerdictPlainAllowed
	// VerdictMacFailedAllowed means the PDU was integrity-protected, its
	// MAC failed, and its message type is on the TS 33.501 §6.4.6 step 3
	// whitelist. Handlers run with MacFailed=true.
	VerdictMacFailedAllowed
)

// DecodeResult is the output of the pure NAS decoder. Callers inspect
// Verdict and are the only sites allowed to mutate UE security state.
type DecodeResult struct {
	Message *nas.Message
	Verdict Verdict
}

// classifyNasPdu is the single authority on whether a NAS PDU of the
// given type, security header, and UE security-context state may be
// processed. plainNasAllowed and macFailedAllowed are reached only from
// here.
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
// without integrity protection per TS 24.501 §4.4.4.3.
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
// after MAC verification failure per TS 33.501 §6.4.6 step 3. It extends
// plainNasAllowed with ServiceRequest.
func macFailedAllowed(msgType uint8) bool {
	if plainNasAllowed(msgType) {
		return true
	}

	return msgType == nas.MsgTypeServiceRequest
}
