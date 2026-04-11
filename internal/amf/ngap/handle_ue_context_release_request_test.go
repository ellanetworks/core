// Copyright 2026 Ella Networks

package ngap_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/free5gc/ngap/ngapType"
)

// TestHandleUEContextReleaseRequest_UnknownUENGAPIDs verifies that a
// UEContextReleaseRequest with AMF_UE_NGAP_ID and RAN_UE_NGAP_ID values that
// don't match any existing UE context is handled gracefully — no panic, and an
// ErrorIndication with UnknownLocalUENGAPID cause is sent back.
// Regression test inspired by open5gs/open5gs#4371.
func TestHandleUEContextReleaseRequest_UnknownUENGAPIDs(t *testing.T) {
	ran := newTestRadio()
	amfInstance := newTestAMF()
	sender := ran.NGAPSender.(*FakeNGAPSender)

	msg := decode.UEContextReleaseRequest{
		AMFUENGAPID: 999999,
		RANUENGAPID: 888888,
		Cause: &ngapType.Cause{
			Present:      ngapType.CausePresentRadioNetwork,
			RadioNetwork: &ngapType.CauseRadioNetwork{Value: ngapType.CauseRadioNetworkPresentUserInactivity},
		},
	}

	assertNoPanic(t, "HandleUEContextReleaseRequest(unknown IDs)", func() {
		ngap.HandleUEContextReleaseRequest(context.Background(), amfInstance, ran, msg)
	})

	if len(sender.SentErrorIndications) != 1 {
		t.Fatalf("expected 1 ErrorIndication, got %d", len(sender.SentErrorIndications))
	}

	errInd := sender.SentErrorIndications[0]
	if errInd.Cause == nil || errInd.Cause.Present != ngapType.CausePresentRadioNetwork {
		t.Fatal("expected RadioNetwork cause in ErrorIndication")
	}

	if errInd.Cause.RadioNetwork.Value != ngapType.CauseRadioNetworkPresentUnknownLocalUENGAPID {
		t.Fatalf("expected UnknownLocalUENGAPID, got %d", errInd.Cause.RadioNetwork.Value)
	}

	if len(sender.SentUEContextReleaseCommands) != 0 {
		t.Fatalf("expected no UEContextReleaseCommand, got %d", len(sender.SentUEContextReleaseCommands))
	}
}
