// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/nas/fgs"
)

func TestHandleSecurityModeReject_NotSecurityMode(t *testing.T) {
	testcases := []struct {
		name  string
		setup func(*amf.UeContext)
	}{
		{"Deregistered", func(ue *amf.UeContext) { ue.ForceStateForTest(amf.Deregistered) }},
		{"Registered", func(ue *amf.UeContext) { ue.ForceStateForTest(amf.Registered) }},
		{"Authenticating", func(ue *amf.UeContext) { ue.ForceRegStepForTest(amf.RegStepAuthenticating) }},
		{"ContextSetup", func(ue *amf.UeContext) { ue.ForceRegStepForTest(amf.RegStepContextSetup) }},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			ue, ngapSender, err := buildUeAndRadio()
			if err != nil {
				t.Fatalf("could not build test UE and radio: %v", err)
			}

			tc.setup(ue)

			handleSecurityModeReject(t.Context(), ue, nil)

			if len(ngapSender.SentUEContextReleaseCommand) != 0 {
				t.Fatalf("expected Security Mode Reject outside the security mode exchange to be ignored, but a UE Context Release Command was sent")
			}
		})
	}
}

func TestHandleSecurityModeReject_T3560Stopped_UEContextReleased(t *testing.T) {
	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build test UE and radio: %v", err)
	}

	ue.SetSecuredForTest(true)
	ue.Conn().ReleaseAction = amf.UeContextN2NormalRelease
	ue.ForceRegStepForTest(amf.RegStepSecurityMode)
	conn := ue.Conn()
	conn.NASGuardForTest().Arm(5*time.Minute, 5, func(expireTimes int32) {}, func() {})

	handleSecurityModeReject(t.Context(), ue, buildTestSecurityModeReject(t))

	if conn.NASGuardForTest().Active() {
		t.Fatal("expected timer T3560 to be stopped and cleared")
	}

	if ue.State() != amf.Deregistered {
		t.Fatalf("expected UE to be deregistered but was: %v", ue.State())
	}

	if ue.SecuredForTest() {
		t.Fatal("expected UE security context available to be reset to false")
	}

	if len(ngapSender.SentUEContextReleaseCommand) != 1 {
		t.Fatalf("should have sent a UE Context Release Command message")
	}
}

func buildTestSecurityModeReject(_ *testing.T) []byte {
	// Plain SECURITY MODE REJECT: EPD, security-header-type+spare, message type, cause.
	return []byte{fgs.EPD5GMM, 0x00, uint8(fgs.MsgSecurityModeReject), 0x18}
}
