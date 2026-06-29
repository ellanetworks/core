// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"fmt"
	"testing"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
)

func TestHandleConfigurationUpdateComplete_NotRegisteredError(t *testing.T) {
	testcases := []amf.StateType{amf.Authentication, amf.Deregistered, amf.ContextSetup, amf.SecurityMode}

	for _, tc := range testcases {
		t.Run(string(tc), func(t *testing.T) {
			ue := amf.NewUeContext()
			ue.ForceState(tc)

			expected := fmt.Sprintf("state mismatch: receive Configuration Update Complete message in state %s", tc)

			amfInstance := amf.New(nil, nil, nil)

			err := handleConfigurationUpdateComplete(amfInstance, ue, false)
			if err == nil || err.Error() != expected {
				t.Fatalf("expected error: %s, got %v", expected, err)
			}
		})
	}
}

func TestHandleConfigurationUpdateComplete_MacFailed(t *testing.T) {
	ue := amf.NewUeContext()
	ue.ForceState(amf.Registered)

	expected := "NAS message integrity check failed"

	amfInstance := amf.New(nil, nil, nil)

	err := handleConfigurationUpdateComplete(amfInstance, ue, false)
	if err == nil || err.Error() != expected {
		t.Fatalf("expected error: %s, got %v", expected, err)
	}
}

func TestHandleConfigurationUpdateComplete_T3555Stopped_OldGutiFreed(t *testing.T) {
	ue := amf.NewUeContext()
	ue.ForceState(amf.Registered)
	ue.NasConn().T3555.Arm(5*time.Minute, 5, func(expireTimes int32) {}, func() {})
	ue.OldGuti = mustTestGuti("001", "01", "cafe42", 0x12345678)
	ue.OldTmsi = mustValidTestTmsi(0x12345678)

	amfInstance := amf.New(nil, nil, nil)

	err := handleConfigurationUpdateComplete(amfInstance, ue, true)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if ue.NasConn().T3555.Active() {
		t.Fatal("expected timer T3555 to be stopped and cleared")
	}

	if ue.OldGuti != etsi.InvalidGUTI || ue.OldTmsi != etsi.InvalidTMSI {
		t.Fatal("expected old GUTI and TMSI to be invalidated")
	}
}
