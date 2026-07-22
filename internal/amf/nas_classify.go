// SPDX-FileCopyrightText: Ella Networks Inc.
//
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"github.com/free5gc/nas"
)

// DecodeResult is the output of the pure NAS decoder. A rejected PDU is returned as
// a decode error (never a result), so a result is always processable; IntegrityVerified
// reports whether its MAC was verified — false for a plain or MAC-failed message admitted
// before secure exchange per TS 24.501 §4.4.4.3.
type DecodeResult struct {
	Message           *nas.Message
	IntegrityVerified bool
	// PlainBody is the decoded plain 5GMM message (after any decipher), the input
	// for home-built (nas/fgs) handlers during the incremental migration off
	// free5gc. It starts with the extended protocol discriminator.
	PlainBody []byte
}

// plainNasAllowed reports whether a NAS message type may be processed without a verified
// MAC before secure exchange (TS 24.501 §4.4.4.3, TS 33.501) — either sent as plain NAS,
// or received integrity-protected with a failed MAC. SERVICE REQUEST is on the spec's
// MAC-failed list but absent here: it is verified before context binding by the dedicated
// HandleServiceRequest and rejected with cause #9 on failure, never admitted here.
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
