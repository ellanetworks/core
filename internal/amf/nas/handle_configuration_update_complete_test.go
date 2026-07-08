// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"testing"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
)

func TestHandleConfigurationUpdateComplete_NotRegisteredIgnored(t *testing.T) {
	testcases := []amf.StateType{amf.Deregistered, amf.RegistrationInitiated, amf.DeregistrationInitiated}

	for _, tc := range testcases {
		t.Run(string(tc), func(t *testing.T) {
			ue, _, err := buildUeAndRadio()
			if err != nil {
				t.Fatalf("could not build test ue: %v", err)
			}

			ue.ForceStateForTest(tc)
			ue.Conn().NASGuardForTest().Arm(5*time.Minute, 5, func(expireTimes int32) {}, func() {})

			amfInstance := amf.New(nil, nil, nil)

			handleConfigurationUpdateComplete(amfInstance, ue)

			if !ue.Conn().NASGuardForTest().Active() {
				t.Fatal("expected out-of-state Configuration Update Complete to be ignored, leaving the NAS guard armed")
			}
		})
	}
}

func TestHandleConfigurationUpdateComplete_T3555Stopped_OldGutiFreed(t *testing.T) {
	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build test ue: %v", err)
	}

	ue.ForceStateForTest(amf.Registered)
	ue.Conn().NASGuardForTest().Arm(5*time.Minute, 5, func(expireTimes int32) {}, func() {})
	ue.SetOldTmsiForTest(mustTestGuti("001", "01", "cafe42", 0x12345678).Tmsi)
	ue.SetOldTmsiForTest(mustValidTestTmsi(0x12345678))

	amfInstance := amf.New(nil, nil, nil)

	handleConfigurationUpdateComplete(amfInstance, ue)

	if ue.Conn().NASGuardForTest().Active() {
		t.Fatal("expected timer T3555 to be stopped and cleared")
	}

	if ue.OldTmsi() != etsi.InvalidTMSI {
		t.Fatal("expected old GUTI and TMSI to be invalidated")
	}
}
