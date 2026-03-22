package gmm

import (
	"testing"
	"time"

	amfContext "github.com/ellanetworks/core/internal/amf"
)

func TestHandleDeregistrationAccept_T3522Stopped_UEContextReleaseCommand(t *testing.T) {
	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build test UE and radio: %v", err)
	}

	ue.State = amfContext.Registered
	ue.T3522 = amfContext.NewTimer(5*time.Minute, 5, func(expireTimes int32) {}, func() {})

	err = handleDeregistrationAccept(t.Context(), ue)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if ue.State != amfContext.Deregistered {
		t.Fatalf("expected UE to be deregistered, but was: %s", ue.State)
	}

	if ue.T3522 != nil {
		t.Fatal("expected timer T3522 to be stopped and cleared")
	}

	if len(ngapSender.SentUEContextReleaseCommand) != 1 {
		t.Fatal("should have sent a UE Context Release Command message")
	}
}

func TestHandleDeregistrationAccept_NilRanUE_NoMessage(t *testing.T) {
	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build test UE and radio: %v", err)
	}

	ue.RanUe = nil

	err = handleDeregistrationAccept(t.Context(), ue)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(ngapSender.SentUEContextReleaseCommand) != 0 {
		t.Fatal("should not have sent a UE Context Release Command message")
	}
}
