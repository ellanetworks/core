// Copyright 2026 Ella Networks

package ngap_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/free5gc/ngap/ngapType"
)

func TestHandleInitialUEMessage_NoRanID_SendsErrorIndication(t *testing.T) {
	ran := newTestRadio()
	amfInstance := newTestAMF()

	ngap.HandleInitialUEMessage(context.Background(), amfInstance, ran, decode.InitialUEMessage{})

	sender, ok := ran.NGAPSender.(*FakeNGAPSender)
	if !ok {
		t.Fatalf("ran.NGAPSender is %T, want *FakeNGAPSender", ran.NGAPSender)
	}

	if got := len(sender.SentErrorIndications); got != 1 {
		t.Fatalf("len(SentErrorIndications) = %d, want 1", got)
	}

	ei := sender.SentErrorIndications[0]
	if ei.Cause == nil {
		t.Fatal("ErrorIndication.Cause is nil")
	}

	if ei.Cause.Present != ngapType.CausePresentProtocol {
		t.Errorf("cause.Present = %d, want CausePresentProtocol", ei.Cause.Present)
	}

	if ei.Cause.Protocol == nil ||
		ei.Cause.Protocol.Value != ngapType.CauseProtocolPresentMessageNotCompatibleWithReceiverState {
		t.Errorf("cause.Protocol = %+v, want MessageNotCompatibleWithReceiverState", ei.Cause.Protocol)
	}

	if ei.CriticalityDiagnostics == nil {
		t.Error("CriticalityDiagnostics is nil")
	}
}
