// Copyright 2026 Ella Networks

package ngap_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/logger"
)

func TestHandleUplinkNasTransport_UnknownRanUe(t *testing.T) {
	ran := newTestRadio()
	sender := ran.NGAPSender.(*FakeNGAPSender)
	amf := newTestAMF()

	ngap.HandleUplinkNasTransport(context.Background(), amf, ran, decode.UplinkNASTransport{
		AMFUENGAPID: 1,
		RANUENGAPID: 1,
		NASPDU:      []byte{0x7E, 0x00, 0x55},
	})

	if len(sender.SentErrorIndications) != 0 {
		t.Fatalf("expected no ErrorIndication, got %d", len(sender.SentErrorIndications))
	}
}

func TestHandleUplinkNasTransport_NilAmfUe_RemovesRanUe(t *testing.T) {
	ran := newTestRadio()
	amfInstance := newTestAMF()

	ranUe := &amf.RanUe{
		RanUeNgapID: 1,
		AmfUeNgapID: 10,
		Radio:       ran,
		Log:         logger.AmfLog,
	}
	ran.RanUEs[1] = ranUe

	ngap.HandleUplinkNasTransport(context.Background(), amfInstance, ran, decode.UplinkNASTransport{
		AMFUENGAPID: 10,
		RANUENGAPID: 1,
		NASPDU:      []byte{0x7E, 0x00, 0x55},
	})

	if _, exists := ran.RanUEs[1]; exists {
		t.Error("expected RanUe to be removed from radio's map")
	}
}
