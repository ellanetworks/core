// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import "github.com/ellanetworks/core/nas/eps"

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
