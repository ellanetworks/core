// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import "github.com/ellanetworks/core/nas/eps"

// verdict classifies an inbound EMM NAS PDU by security header and MAC result,
// deciding whether it may be processed (TS 24.301 §4.4.4.3). It does not consider
// whether secure exchange is already established on the connection; the caller
// discards anything but verdictIntegrityVerified once it is.
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
// integrity protection before secure exchange is established (TS 24.301 §4.4.4.3).
// TRACKING AREA UPDATE REQUEST and SERVICE REQUEST are on the spec list but are
// absent here: each is integrity-verified at its S1AP Initial UE Message (S-TMSI
// resume / short-MAC) before a context is bound, so it never reaches this EMM
// dispatch path — and its handler assumes a verified message.
func plainNasAllowed(mt eps.MessageType) bool {
	switch mt {
	case eps.MsgAttachRequest,
		eps.MsgIdentityResponse,
		eps.MsgAuthenticationResponse,
		eps.MsgAuthenticationFailure,
		eps.MsgSecurityModeReject,
		eps.MsgDetachRequest,
		eps.MsgDetachAccept:
		return true
	}

	return false
}

// macFailedAllowed reports whether an EMM message may be processed after a failed
// integrity check, before secure exchange is established (TS 24.301 §4.4.4.3). The
// spec's MAC-failed list adds SERVICE REQUEST, EXTENDED SERVICE REQUEST and
// CONTROL PLANE SERVICE REQUEST to the plain list; none reaches this path (SERVICE
// REQUEST is security-header-typed and verified at S1AP, and the other two are 4G
// CS-fallback/CIoT procedures Ella Core does not implement), so the list matches
// plainNasAllowed.
func macFailedAllowed(mt eps.MessageType) bool {
	return plainNasAllowed(mt)
}
