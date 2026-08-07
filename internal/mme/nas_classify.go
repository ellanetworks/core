// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import "github.com/ellanetworks/core/nas/eps"

// An ESM message fails the EMM protocol-discriminator check, so it lands on the
// required-ciphered default.
func cipheringRequiredFor(plain []byte) bool {
	mt, err := eps.PeekMessageType(plain)
	if err != nil {
		return true
	}

	return cipheringRequired(mt)
}

// TS 24.301 §4.4.5 has the UE send ATTACH REQUEST and TRACKING AREA UPDATE
// REQUEST always unciphered, and the initial NAS message of a new NAS signalling
// connection unciphered whatever it is. An initial NAS message is one that can
// trigger the establishment of that connection (§3.1), so for the types below
// whether ciphering was owed depends on a UE-side decision the receiver cannot
// observe; only the types that can never be initial are held to it. SERVICE
// REQUEST carries its own security header type and never reaches here.
func cipheringRequired(mt eps.MessageType) bool {
	switch mt {
	case eps.MsgAttachRequest,
		eps.MsgDetachRequest,
		eps.MsgTrackingAreaUpdateRequest,
		eps.MsgExtendedServiceRequest,
		eps.MsgControlPlaneServiceRequest:
		return false
	}

	return true
}

// plainNasAllowed reports whether an EMM message may be processed without a verified
// MAC before secure exchange is established (TS 24.301 §4.4.4.3) — either sent as plain
// NAS, or received integrity-protected with a failed MAC. The spec's plain and
// MAC-failed lists coincide for Ella Core: TRACKING AREA UPDATE REQUEST and SERVICE
// REQUEST are integrity-verified at their S1AP Initial UE Message (S-TMSI resume /
// short-MAC) before a context is bound, so they never reach this EMM dispatch path;
// EXTENDED and CONTROL PLANE SERVICE REQUEST are CS-fallback/CIoT procedures Ella Core
// does not implement.
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
