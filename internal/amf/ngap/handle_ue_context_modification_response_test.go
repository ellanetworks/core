// Copyright 2026 Ella Networks

package ngap_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/amf/sctp"
	"github.com/ellanetworks/core/internal/logger"
)

func TestHandleUEContextModificationResponse_UnknownRanUeNgapID(t *testing.T) {
	ran := newTestRadio()
	sender := ran.NGAPSender.(*FakeNGAPSender)
	amfInstance := newTestAMF()

	ranID := int64(1)
	msg := decode.UEContextModificationResponse{
		RANUENGAPID: &ranID,
	}

	ngap.HandleUEContextModificationResponse(context.Background(), amfInstance, ran, msg)

	if len(sender.SentErrorIndications) != 1 {
		t.Fatalf("expected 1 ErrorIndication (TS 38.413 §10.6), got %d", len(sender.SentErrorIndications))
	}
}

func TestHandleUEContextModificationResponse_UEFound(t *testing.T) {
	ran := newTestRadio()
	amfInstance := newTestAMF()
	amfInstance.Radios[new(sctp.SCTPConn)] = ran

	ranUe := amf.NewRanUeForTest(ran, 1, 10, logger.AmfLog)

	amfUeNgapID := int64(10)
	msg := decode.UEContextModificationResponse{
		AMFUENGAPID: &amfUeNgapID,
	}

	ngap.HandleUEContextModificationResponse(context.Background(), amfInstance, ran, msg)

	// ranUe was already created on 'ran', so Radio() should still be 'ran'.
	if ranUe.Radio() != ran {
		t.Error("expected ranUe.Radio to still be ran")
	}
}
