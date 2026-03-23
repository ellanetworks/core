package gmm

import (
	"fmt"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
)

func TestHandleSecurityModeReject_NotSecurityMode(t *testing.T) {
	testcases := []amf.StateType{amf.Authentication, amf.Deregistered, amf.ContextSetup, amf.Registered}

	for _, tc := range testcases {
		t.Run(string(tc), func(t *testing.T) {
			ue := amf.NewAmfUe()
			ue.ForceState(tc)

			expected := fmt.Sprintf("state mismatch: receive Security Mode Reject message in state %s", tc)

			err := handleSecurityModeReject(t.Context(), ue, nil)
			if err == nil || err.Error() != expected {
				t.Fatalf("expected error: %s, got %v", expected, err)
			}
		})
	}
}

func TestHandleSecurityModeReject_T3560Stopped_UEContextReleased(t *testing.T) {
	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build test UE and radio: %v", err)
	}

	ue.SecurityContextAvailable = true
	ue.RanUe.ReleaseAction = amf.UeContextN2NormalRelease
	ue.ForceState(amf.SecurityMode)
	ue.T3560 = amf.NewTimer(5*time.Minute, 5, func(expireTimes int32) {}, func() {})

	m := buildTestSecurityModeReject()

	err = handleSecurityModeReject(t.Context(), ue, m.SecurityModeReject)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if ue.T3560 != nil {
		t.Fatal("expected timer T3560 to be stopped and cleared")
	}

	if ue.GetState() != amf.Deregistered {
		t.Fatalf("expected UE to be deregistered but was: %v", ue.GetState())
	}

	if ue.SecurityContextAvailable {
		t.Fatal("expected UE security context available to be reset to false")
	}

	if len(ngapSender.SentUEContextReleaseCommand) != 1 {
		t.Fatalf("should have sent a UE Context Release Command message")
	}
}

func buildTestSecurityModeReject() *nas.GmmMessage {
	m := nas.NewGmmMessage()

	securityModeReject := nasMessage.NewSecurityModeReject(0)
	securityModeReject.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSMobilityManagementMessage)
	securityModeReject.SetSpareHalfOctet(0x00)
	securityModeReject.SetMessageType(nas.MsgTypeSecurityModeReject)

	m.SecurityModeReject = securityModeReject
	m.SetMessageType(nas.MsgTypeSecurityModeReject)

	return m
}
