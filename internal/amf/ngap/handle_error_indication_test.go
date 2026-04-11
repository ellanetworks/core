// Copyright 2026 Ella Networks

package ngap_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
)

func TestHandleErrorIndication_EmptyIEs(t *testing.T) {
	ran := newTestRadio()
	sender := ran.NGAPSender.(*FakeNGAPSender)

	ngap.HandleErrorIndication(context.Background(), ran, decode.ErrorIndication{})

	if len(sender.SentErrorIndications) != 0 {
		t.Fatalf("expected no ErrorIndication, got %d", len(sender.SentErrorIndications))
	}
}
