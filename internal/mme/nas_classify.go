// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import "github.com/ellanetworks/core/nas/eps"

// verdict is the result of classifying an inbound EMM NAS PDU against its
// security header and MAC result. It is the single authority on whether a PDU
// may be processed (TS 24.301 §4.4.4.3), mirroring the 5G AMF's Verdict /
// classifyNasPdu. It does not consider whether secure exchange is already
// established on the connection — the caller discards anything but
// verdictIntegrityVerified once it is.
type verdict int

const (
	verdictReject verdict = iota
	verdictIntegrityVerified
	verdictPlainAllowed
	verdictMacFailedAllowed
)

// classifyNasPdu reports whether an EMM message of the given type may be
// processed, given its security header and (for protected messages) MAC result,
// per TS 24.301 §4.4.4.3.
func classifyNasPdu(mt eps.MessageType, securityHeader uint8, macVerified bool) verdict {
	if securityHeader == uint8(eps.SHTPlain) {
		if plainNasAllowed(mt) {
			return verdictPlainAllowed
		}

		return verdictReject
	}

	if macVerified {
		return verdictIntegrityVerified
	}

	if macFailedAllowed(mt) {
		return verdictMacFailedAllowed
	}

	return verdictReject
}

// plainNasAllowed reports whether an EMM message may be processed without
// integrity protection (TS 24.301 §4.4.4.3).
//
// EPS SERVICE REQUEST and TRACKING AREA UPDATE arrive as their own S1AP Initial
// UE Message and are integrity-verified there before binding a context, so they
// never reach this EMM dispatch path — the one justified divergence from the 5G
// whitelist (which lists SERVICE REQUEST). DETACH ACCEPT is likewise omitted: a
// network-initiated detach is acknowledged under the established security
// context, and a stray plain DETACH ACCEPT is left to the detach guard.
func plainNasAllowed(mt eps.MessageType) bool {
	switch mt {
	case eps.MsgAttachRequest,
		eps.MsgIdentityResponse,
		eps.MsgAuthenticationResponse,
		eps.MsgAuthenticationFailure,
		eps.MsgSecurityModeReject,
		eps.MsgDetachRequest:
		return true
	}

	return false
}

// macFailedAllowed reports whether an EMM message may be processed after a failed
// integrity check, before secure exchange is established on the connection; the
// §4.4.4.3 whitelist is the same as plainNasAllowed (TS 24.301 §4.4.4.3).
func macFailedAllowed(mt eps.MessageType) bool {
	return plainNasAllowed(mt)
}
