// Copyright 2026 Ella Networks
//
// SPDX-License-Identifier: Apache-2.0

package amf

import (
	"testing"

	"github.com/free5gc/nas"
)

// TestClassifyNasPdu_ExhaustiveTable is the sole authority on the NAS
// classification policy. Every GMM message type that may reach the AMF
// is enumerated and every relevant (securityHeader, macVerified) tuple
// is exercised. Any change to the whitelist must land here first.
func TestClassifyNasPdu_ExhaustiveTable(t *testing.T) {
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

	plain := map[uint8]bool{
		nas.MsgTypeRegistrationRequest:                              true,
		nas.MsgTypeIdentityResponse:                                 true,
		nas.MsgTypeAuthenticationResponse:                           true,
		nas.MsgTypeAuthenticationFailure:                            true,
		nas.MsgTypeSecurityModeReject:                               true,
		nas.MsgTypeDeregistrationRequestUEOriginatingDeregistration: true,
		nas.MsgTypeDeregistrationAcceptUETerminatedDeregistration:   true,
	}

	macFailed := map[uint8]bool{
		nas.MsgTypeRegistrationRequest:                              true,
		nas.MsgTypeIdentityResponse:                                 true,
		nas.MsgTypeAuthenticationResponse:                           true,
		nas.MsgTypeAuthenticationFailure:                            true,
		nas.MsgTypeSecurityModeReject:                               true,
		nas.MsgTypeServiceRequest:                                   true,
		nas.MsgTypeDeregistrationRequestUEOriginatingDeregistration: true,
		nas.MsgTypeDeregistrationAcceptUETerminatedDeregistration:   true,
	}

	protectedHeaders := []uint8{
		nas.SecurityHeaderTypeIntegrityProtected,
		nas.SecurityHeaderTypeIntegrityProtectedAndCiphered,
		nas.SecurityHeaderTypeIntegrityProtectedAndCipheredWithNew5gNasSecurityContext,
	}

	for _, mt := range allTypes {
		// Plain NAS.
		got := classifyNasPdu(mt, nas.SecurityHeaderTypePlainNas, false)

		want := VerdictReject
		if plain[mt] {
			want = VerdictPlainAllowed
		}

		if got != want {
			t.Errorf("plain mt=%d: got verdict %d, want %d", mt, got, want)
		}

		// Integrity-protected, MAC OK: always integrity-verified.
		for _, sh := range protectedHeaders {
			if got := classifyNasPdu(mt, sh, true); got != VerdictIntegrityVerified {
				t.Errorf("protected mt=%d sh=%d macOK: got %d, want VerdictIntegrityVerified", mt, sh, got)
			}
		}

		// Integrity-protected, MAC failed.
		for _, sh := range protectedHeaders {
			got := classifyNasPdu(mt, sh, false)

			want := VerdictReject
			if macFailed[mt] {
				want = VerdictMacFailedAllowed
			}

			if got != want {
				t.Errorf("protected mt=%d sh=%d macFail: got %d, want %d", mt, sh, got, want)
			}
		}
	}
}

// TestClassifyNasPdu_ServiceRequestOnlyMacFailed pins the
// ServiceRequest asymmetry: it is only acceptable on the MAC-failed
// path, never as plain NAS.
func TestClassifyNasPdu_ServiceRequestOnlyMacFailed(t *testing.T) {
	if v := classifyNasPdu(nas.MsgTypeServiceRequest, nas.SecurityHeaderTypePlainNas, false); v != VerdictReject {
		t.Errorf("plain ServiceRequest must be rejected; got verdict %d", v)
	}

	if v := classifyNasPdu(nas.MsgTypeServiceRequest, nas.SecurityHeaderTypeIntegrityProtected, false); v != VerdictMacFailedAllowed {
		t.Errorf("MAC-failed ServiceRequest must be VerdictMacFailedAllowed; got %d", v)
	}
}
