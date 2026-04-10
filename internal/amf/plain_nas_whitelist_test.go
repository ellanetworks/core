// Copyright 2026 Ella Networks
//
// SPDX-License-Identifier: Apache-2.0

package amf

import (
	"testing"

	"github.com/free5gc/nas"
)

// TestPlainNasAllowed pins down the TS 24.501 §4.4.4.3 whitelist used by
// the NAS decoder for plain NAS PDUs. Any change to this list is a
// security boundary change.
func TestPlainNasAllowed(t *testing.T) {
	allowed := map[uint8]bool{
		nas.MsgTypeRegistrationRequest:                              true,
		nas.MsgTypeIdentityResponse:                                 true,
		nas.MsgTypeAuthenticationResponse:                           true,
		nas.MsgTypeAuthenticationFailure:                            true,
		nas.MsgTypeSecurityModeReject:                               true,
		nas.MsgTypeDeregistrationRequestUEOriginatingDeregistration: true,
		nas.MsgTypeDeregistrationAcceptUETerminatedDeregistration:   true,
	}

	allTypes := []uint8{
		nas.MsgTypeRegistrationRequest,
		nas.MsgTypeRegistrationAccept,
		nas.MsgTypeRegistrationComplete,
		nas.MsgTypeRegistrationReject,
		nas.MsgTypeDeregistrationRequestUEOriginatingDeregistration,
		nas.MsgTypeDeregistrationAcceptUEOriginatingDeregistration,
		nas.MsgTypeDeregistrationRequestUETerminatedDeregistration,
		nas.MsgTypeDeregistrationAcceptUETerminatedDeregistration,
		nas.MsgTypeServiceRequest,
		nas.MsgTypeServiceReject,
		nas.MsgTypeServiceAccept,
		nas.MsgTypeConfigurationUpdateCommand,
		nas.MsgTypeConfigurationUpdateComplete,
		nas.MsgTypeAuthenticationRequest,
		nas.MsgTypeAuthenticationResponse,
		nas.MsgTypeAuthenticationReject,
		nas.MsgTypeAuthenticationFailure,
		nas.MsgTypeAuthenticationResult,
		nas.MsgTypeIdentityRequest,
		nas.MsgTypeIdentityResponse,
		nas.MsgTypeSecurityModeCommand,
		nas.MsgTypeSecurityModeComplete,
		nas.MsgTypeSecurityModeReject,
		nas.MsgTypeStatus5GMM,
		nas.MsgTypeNotification,
		nas.MsgTypeNotificationResponse,
		nas.MsgTypeULNASTransport,
		nas.MsgTypeDLNASTransport,
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
	if plainNasAllowed(nas.MsgTypeServiceRequest) {
		t.Fatal("ServiceRequest must NOT be on the plain-NAS whitelist (TS 24.501 §4.4.4.3)")
	}
}

// TestPlainNasAllowed_RejectsULNasTransport pins ULNasTransport off the
// plain-NAS whitelist.
func TestPlainNasAllowed_RejectsULNasTransport(t *testing.T) {
	if plainNasAllowed(nas.MsgTypeULNASTransport) {
		t.Fatal("ULNASTransport must NOT be on the plain-NAS whitelist (TS 24.501 §4.4.4.3)")
	}
}

// TestMacFailedAllowed pins down the macFailedAllowed whitelist
// (plainNasAllowed plus ServiceRequest, per TS 33.501 §6.4.6 step 3).
func TestMacFailedAllowed(t *testing.T) {
	allowed := map[uint8]bool{
		nas.MsgTypeRegistrationRequest:                              true,
		nas.MsgTypeIdentityResponse:                                 true,
		nas.MsgTypeAuthenticationResponse:                           true,
		nas.MsgTypeAuthenticationFailure:                            true,
		nas.MsgTypeSecurityModeReject:                               true,
		nas.MsgTypeServiceRequest:                                   true,
		nas.MsgTypeDeregistrationRequestUEOriginatingDeregistration: true,
		nas.MsgTypeDeregistrationAcceptUETerminatedDeregistration:   true,
	}

	allTypes := []uint8{
		nas.MsgTypeRegistrationRequest,
		nas.MsgTypeRegistrationAccept,
		nas.MsgTypeRegistrationComplete,
		nas.MsgTypeRegistrationReject,
		nas.MsgTypeDeregistrationRequestUEOriginatingDeregistration,
		nas.MsgTypeDeregistrationAcceptUEOriginatingDeregistration,
		nas.MsgTypeDeregistrationRequestUETerminatedDeregistration,
		nas.MsgTypeDeregistrationAcceptUETerminatedDeregistration,
		nas.MsgTypeServiceRequest,
		nas.MsgTypeServiceReject,
		nas.MsgTypeServiceAccept,
		nas.MsgTypeConfigurationUpdateCommand,
		nas.MsgTypeConfigurationUpdateComplete,
		nas.MsgTypeAuthenticationRequest,
		nas.MsgTypeAuthenticationResponse,
		nas.MsgTypeAuthenticationReject,
		nas.MsgTypeAuthenticationFailure,
		nas.MsgTypeAuthenticationResult,
		nas.MsgTypeIdentityRequest,
		nas.MsgTypeIdentityResponse,
		nas.MsgTypeSecurityModeCommand,
		nas.MsgTypeSecurityModeComplete,
		nas.MsgTypeSecurityModeReject,
		nas.MsgTypeStatus5GMM,
		nas.MsgTypeNotification,
		nas.MsgTypeNotificationResponse,
		nas.MsgTypeULNASTransport,
		nas.MsgTypeDLNASTransport,
	}

	for _, mt := range allTypes {
		want := allowed[mt]
		if got := macFailedAllowed(mt); got != want {
			t.Errorf("macFailedAllowed(%d): got %v, want %v", mt, got, want)
		}
	}
}
