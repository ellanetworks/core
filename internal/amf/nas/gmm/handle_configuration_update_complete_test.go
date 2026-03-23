package gmm

import (
	"fmt"
	"testing"
	"time"

	"github.com/ellanetworks/core/etsi"
	amfContext "github.com/ellanetworks/core/internal/amf"
)

func TestHandleConfigurationUpdateComplete_NotRegisteredError(t *testing.T) {
	testcases := []amfContext.StateType{amfContext.Authentication, amfContext.Deregistered, amfContext.ContextSetup, amfContext.SecurityMode}

	for _, tc := range testcases {
		t.Run(string(tc), func(t *testing.T) {
			ue := amfContext.NewAmfUe()
			ue.ForceState(tc)

			expected := fmt.Sprintf("state mismatch: receive Configuration Update Complete message in state %s", tc)

			amf := amfContext.New(nil, nil, nil)

			err := handleConfigurationUpdateComplete(amf, ue)
			if err == nil || err.Error() != expected {
				t.Fatalf("expected error: %s, got %v", expected, err)
			}
		})
	}
}

func TestHandleConfigurationUpdateComplete_MacFailed(t *testing.T) {
	ue := amfContext.NewAmfUe()
	ue.ForceState(amfContext.Registered)
	ue.MacFailed = true

	expected := "NAS message integrity check failed"

	amf := amfContext.New(nil, nil, nil)

	err := handleConfigurationUpdateComplete(amf, ue)
	if err == nil || err.Error() != expected {
		t.Fatalf("expected error: %s, got %v", expected, err)
	}
}

func TestHandleConfigurationUpdateComplete_T3555Stopped_OldGutiFreed(t *testing.T) {
	ue := amfContext.NewAmfUe()
	ue.ForceState(amfContext.Registered)
	ue.T3555 = amfContext.NewTimer(5*time.Minute, 5, func(expireTimes int32) {}, func() {})
	ue.OldGuti = mustTestGuti("001", "01", "cafe42", 0x12345678)
	ue.OldTmsi = mustValidTestTmsi(0x12345678)

	amf := amfContext.New(nil, nil, nil)

	err := handleConfigurationUpdateComplete(amf, ue)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if ue.T3555 != nil {
		t.Fatal("expected timer T3555 to be stopped and cleared")
	}

	if ue.OldGuti != etsi.InvalidGUTI || ue.OldTmsi != etsi.InvalidTMSI {
		t.Fatal("expected old GUTI and TMSI to be invalidated")
	}
}
