package gmm

import (
	"fmt"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
)

func TestHandleSecurityModeReject_NotSecurityMode(t *testing.T) {
	testcases := []context.StateType{context.Authentication, context.Deregistered, context.ContextSetup, context.Registered}

	for _, tc := range testcases {
		t.Run(string(tc), func(t *testing.T) {
			ue := context.NewAmfUe()
			ue.State = tc

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
	ue.RanUe.ReleaseAction = context.UeContextN2NormalRelease
	ue.State = context.SecurityMode
	ue.T3560 = context.NewTimer(5*time.Minute, 5, func(expireTimes int32) {}, func() {})

	m := buildTestSecurityModeReject()

	err = handleSecurityModeReject(t.Context(), ue, m.SecurityModeReject)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if ue.T3560 != nil {
		t.Fatal("expected timer T3560 to be stopped and cleared")
	}

	if ue.SecurityContextAvailable {
		t.Fatal("expected UE security context available to be reset to false")
	}

	if ue.RanUe.ReleaseAction != context.UeContextReleaseUeContext {
		t.Fatalf("expected RanUE release action to be set to UeContextReleaseUeContext, got: %v", ue.RanUe.ReleaseAction)
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
