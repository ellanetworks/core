// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"testing"

	"github.com/ellanetworks/core/nas/fgs"
)

// TestPlainNasAllowed pins down the TS 24.501 whitelist used by
// the NAS decoder for plain NAS PDUs. Any change to this list is a
// security boundary change.
func TestPlainNasAllowed(t *testing.T) {
	allowed := map[uint8]bool{
		uint8(fgs.MsgRegistrationRequest):         true,
		uint8(fgs.MsgIdentityResponse):            true,
		uint8(fgs.MsgAuthenticationResponse):      true,
		uint8(fgs.MsgAuthenticationFailure):       true,
		uint8(fgs.MsgSecurityModeReject):          true,
		uint8(fgs.MsgDeregistrationRequestUEOrig): true,
		uint8(fgs.MsgDeregistrationAcceptUETerm):  true,
	}

	allTypes := []uint8{
		uint8(fgs.MsgRegistrationRequest),
		uint8(fgs.MsgRegistrationAccept),
		uint8(fgs.MsgRegistrationComplete),
		uint8(fgs.MsgRegistrationReject),
		uint8(fgs.MsgDeregistrationRequestUEOrig),
		uint8(fgs.MsgDeregistrationAcceptUEOrig),
		uint8(fgs.MsgDeregistrationRequestUETerm),
		uint8(fgs.MsgDeregistrationAcceptUETerm),
		uint8(fgs.MsgServiceRequest),
		uint8(fgs.MsgServiceReject),
		uint8(fgs.MsgServiceAccept),
		uint8(fgs.MsgConfigurationUpdateCommand),
		uint8(fgs.MsgConfigurationUpdateComplete),
		uint8(fgs.MsgAuthenticationRequest),
		uint8(fgs.MsgAuthenticationResponse),
		uint8(fgs.MsgAuthenticationReject),
		uint8(fgs.MsgAuthenticationFailure),
		uint8(fgs.MsgAuthenticationResult),
		uint8(fgs.MsgIdentityRequest),
		uint8(fgs.MsgIdentityResponse),
		uint8(fgs.MsgSecurityModeCommand),
		uint8(fgs.MsgSecurityModeComplete),
		uint8(fgs.MsgSecurityModeReject),
		uint8(fgs.MsgGMMStatus),
		uint8(fgs.MsgNotification),
		uint8(fgs.MsgNotificationResponse),
		uint8(fgs.MsgULNASTransport),
		uint8(fgs.MsgDLNASTransport),
	}

	for _, mt := range allTypes {
		want := allowed[mt]
		if got := plainNasAllowed(mt); got != want {
			t.Errorf("plainNasAllowed(%d): got %v, want %v", mt, got, want)
		}
	}
}

// TestPlainNasAllowed_RejectsServiceRequest pins ServiceRequest off the
// plain-NAS whitelist.
func TestPlainNasAllowed_RejectsServiceRequest(t *testing.T) {
	if plainNasAllowed(uint8(fgs.MsgServiceRequest)) {
		t.Fatal("ServiceRequest must NOT be on the plain-NAS whitelist (TS 24.501)")
	}
}

// TestPlainNasAllowed_RejectsULNasTransport pins ULNasTransport off the
// plain-NAS whitelist.
func TestPlainNasAllowed_RejectsULNasTransport(t *testing.T) {
	if plainNasAllowed(uint8(fgs.MsgULNASTransport)) {
		t.Fatal("ULNASTransport must NOT be on the plain-NAS whitelist (TS 24.501)")
	}
}
