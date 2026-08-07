// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import "github.com/ellanetworks/core/nas/eps"

// cipheringRequiredFor reports whether a plain uplink NAS message has to arrive
// ciphered once ciphering has started (TS 24.301 §4.4.5). Only two EMM messages
// are exempt, so anything that does not decode as one of those — every ESM
// message included, since its protocol discriminator fails the EMM peek —
// requires ciphering.
func cipheringRequiredFor(plain []byte) bool {
	mt, err := eps.PeekMessageType(plain)
	if err != nil {
		return true
	}

	return cipheringRequired(mt)
}

// cipheringRequired reports whether a security-protected uplink message of this
// type has to arrive ciphered once ciphering has started. TS 24.301 §4.4.5 has
// the UE send ATTACH REQUEST and TRACKING AREA UPDATE REQUEST always unciphered.
func cipheringRequired(mt eps.MessageType) bool {
	switch mt {
	case eps.MsgAttachRequest, eps.MsgTrackingAreaUpdateRequest:
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
