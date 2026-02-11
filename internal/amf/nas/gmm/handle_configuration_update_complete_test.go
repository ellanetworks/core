package gmm

import (
	"fmt"
	"testing"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf/context"
)

func TestHandleConfigurationUpdateComplete_NotRegisteredError(t *testing.T) {
	testcases := []context.StateType{context.Authentication, context.Deregistered, context.ContextSetup, context.SecurityMode}

	for _, tc := range testcases {
		t.Run(string(tc), func(t *testing.T) {
			ue := context.NewAmfUe()
			ue.State = tc

			expected := fmt.Sprintf("state mismatch: receive Configuration Update Complete message in state %s", tc)

			err := handleConfigurationUpdateComplete(ue)
			if err == nil || err.Error() != expected {
				t.Fatalf("expected error: %s, got %v", expected, err)
			}
		})
	}
}

func TestHandleConfigurationUpdateComplete_MacFailed(t *testing.T) {
	ue := context.NewAmfUe()
	ue.State = context.Registered
	ue.MacFailed = true

	expected := "NAS message integrity check failed"

	err := handleConfigurationUpdateComplete(ue)
	if err == nil || err.Error() != expected {
		t.Fatalf("expected error: %s, got %v", expected, err)
	}
}

func TestHandleConfigurationUpdateComplete_T3555Stopped_OldGutiFreed(t *testing.T) {
	ue := context.NewAmfUe()
	ue.State = context.Registered
	ue.T3555 = context.NewTimer(5*time.Minute, 5, func(expireTimes int32) {}, func() {})
	ue.OldGuti = mustTestGuti("001", "01", "cafe42", 0x12345678)
	ue.OldTmsi = mustValidTestTmsi(0x12345678)

	err := handleConfigurationUpdateComplete(ue)
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
