// SPDX-FileCopyrightText: Ella Networks Inc.
//
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"github.com/ellanetworks/core/nas/fgs"
)

// DecodeResult is the output of the pure NAS decoder. A rejected PDU is returned as
// a decode error (never a result), so a result is always processable; IntegrityVerified
// reports whether its MAC was verified — false for a plain or MAC-failed message admitted
// before secure exchange per TS 24.501 §4.4.4.3.
type DecodeResult struct {
	// MessageType is the 5GMM message type of Plain (TS 24.501 §9.7); meaningful when IsGMM.
	MessageType uint8
	// IsGMM reports whether Plain is a 5GMM message; false for a standalone 5GSM message on N1.
	IsGMM             bool
	IntegrityVerified bool
	// Plain is the decoded plain 5GMM message (after any decipher), the input for the
	// home-built (nas/fgs) handlers. It starts with the extended protocol discriminator.
	Plain []byte
}

// plainNasAllowed reports whether a NAS message type may be processed without a verified
// MAC before secure exchange (TS 24.501 §4.4.4.3, TS 33.501) — either sent as plain NAS,
// or received integrity-protected with a failed MAC. SERVICE REQUEST is on the spec's
// MAC-failed list but absent here: it is verified before context binding by the dedicated
// HandleServiceRequest and rejected with cause #9 on failure, never admitted here.
func plainNasAllowed(msgType uint8) bool {
	switch msgType {
	case uint8(fgs.MsgRegistrationRequest),
		uint8(fgs.MsgIdentityResponse),
		uint8(fgs.MsgAuthenticationResponse),
		uint8(fgs.MsgAuthenticationFailure),
		uint8(fgs.MsgSecurityModeReject),
		uint8(fgs.MsgDeregistrationRequestUEOrig),
		uint8(fgs.MsgDeregistrationAcceptUETerm):
		return true
	default:
		return false
	}
}
