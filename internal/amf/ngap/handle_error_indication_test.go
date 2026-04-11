// Copyright 2026 Ella Networks

package ngap_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/free5gc/ngap/ngapType"
)

func TestHandleErrorIndication_EmptyIEs(t *testing.T) {
	ran := newTestRadio()
	sender := ran.NGAPSender.(*FakeNGAPSender)

	ngap.HandleErrorIndication(context.Background(), ran, decode.ErrorIndication{})

	if len(sender.SentErrorIndications) != 0 {
		t.Fatalf("expected no ErrorIndication, got %d", len(sender.SentErrorIndications))
	}
}

func TestHandleErrorIndication_WithCause(t *testing.T) {
	ran := newTestRadio()
	sender := ran.NGAPSender.(*FakeNGAPSender)

	msg := decode.ErrorIndication{
		Cause: &ngapType.Cause{
			Present:      ngapType.CausePresentRadioNetwork,
			RadioNetwork: &ngapType.CauseRadioNetwork{Value: ngapType.CauseRadioNetworkPresentUnspecified},
		},
	}

	ngap.HandleErrorIndication(context.Background(), ran, msg)

	if len(sender.SentErrorIndications) != 0 {
		t.Fatalf("expected no ErrorIndication sent back, got %d", len(sender.SentErrorIndications))
	}
}

func TestHandleErrorIndication_WithCriticalityDiagnostics(t *testing.T) {
	ran := newTestRadio()
	sender := ran.NGAPSender.(*FakeNGAPSender)

	msg := decode.ErrorIndication{
		CriticalityDiagnostics: &ngapType.CriticalityDiagnostics{},
	}

	ngap.HandleErrorIndication(context.Background(), ran, msg)

	if len(sender.SentErrorIndications) != 0 {
		t.Fatalf("expected no ErrorIndication sent back, got %d", len(sender.SentErrorIndications))
	}
}
