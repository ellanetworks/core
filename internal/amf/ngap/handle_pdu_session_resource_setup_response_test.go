// Copyright 2026 Ella Networks

package ngap_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
)

func TestHandlePDUSessionResourceSetupResponse_EmptyMessage(t *testing.T) {
	ran := newTestRadio()
	sender := ran.NGAPSender.(*FakeNGAPSender)
	amfInstance := newTestAMF()

	msg := decode.PDUSessionResourceSetupResponse{}

	ngap.HandlePDUSessionResourceSetupResponse(context.Background(), amfInstance, ran, msg)

	if len(sender.SentErrorIndications) != 0 {
		t.Fatalf("expected no ErrorIndication, got %d", len(sender.SentErrorIndications))
	}
}

// TestHandlePDUSessionResourceSetupResponse_UnknownAMFUENGAPID verifies that a
// PDUSessionResourceSetupResponse with an AMF_UE_NGAP_ID that doesn't match
// any existing UE context is handled gracefully — no panic, no further
// processing.
// Regression test inspired by open5gs/open5gs#4377.
func TestHandlePDUSessionResourceSetupResponse_UnknownAMFUENGAPID(t *testing.T) {
	ran := newTestRadio()
	amfInstance := newTestAMF()

	amfID := int64(1379640380095)
	ranID := int64(99)
	msg := decode.PDUSessionResourceSetupResponse{
		AMFUENGAPID: &amfID,
		RANUENGAPID: &ranID,
	}

	ngap.HandlePDUSessionResourceSetupResponse(context.Background(), amfInstance, ran, msg)

	sender := ran.NGAPSender.(*FakeNGAPSender)
	if len(sender.SentErrorIndications) != 0 {
		t.Fatalf("expected no ErrorIndication, got %d", len(sender.SentErrorIndications))
	}
}

// TestHandlePDUSessionResourceSetupResponse_OnlyUnknownRANUENGAPID verifies
// that when only the RAN_UE_NGAP_ID is present but unknown, the handler does
// not crash or process further.
func TestHandlePDUSessionResourceSetupResponse_OnlyUnknownRANUENGAPID(t *testing.T) {
	ran := newTestRadio()
	amfInstance := newTestAMF()

	ranID := int64(42)
	msg := decode.PDUSessionResourceSetupResponse{
		RANUENGAPID: &ranID,
	}

	ngap.HandlePDUSessionResourceSetupResponse(context.Background(), amfInstance, ran, msg)

	sender := ran.NGAPSender.(*FakeNGAPSender)
	if len(sender.SentErrorIndications) != 0 {
		t.Fatalf("expected no ErrorIndication, got %d", len(sender.SentErrorIndications))
	}
}
