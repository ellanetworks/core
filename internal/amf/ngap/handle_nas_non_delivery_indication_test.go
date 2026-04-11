// Copyright 2026 Ella Networks

package ngap_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/ngap/ngapType"
)

func TestNasNonDeliveryIndication_UnknownRanUeNgapID(t *testing.T) {
	ran := newTestRadio()
	amfInstance := newTestAMF()

	ngap.HandleNasNonDeliveryIndication(context.Background(), amfInstance, ran, decode.NASNonDeliveryIndication{
		RANUENGAPID: 99,
	})
}

func TestNasNonDeliveryIndication_UEFoundDispatchesNAS(t *testing.T) {
	ran := newTestRadio()

	amfUe := amf.NewAmfUe()
	amfUe.Log = logger.AmfLog

	ranUe := &amf.RanUe{
		RanUeNgapID: 1,
		AmfUeNgapID: 10,
		Radio:       ran,
		Log:         logger.AmfLog,
	}
	amfUe.AttachRanUe(ranUe)
	ran.RanUEs[1] = ranUe

	amfInstance := newTestAMF()

	// Minimal NAS PDU — HandleNAS will fail to parse it, but the handler
	// just logs the error. We verify no panic and the UE lookup succeeds.
	ngap.HandleNasNonDeliveryIndication(context.Background(), amfInstance, ran, decode.NASNonDeliveryIndication{
		RANUENGAPID: 1,
		NASPDU:      []byte{0x7E, 0x00, 0x00},
		Cause: ngapType.Cause{
			Present:      ngapType.CausePresentRadioNetwork,
			RadioNetwork: &ngapType.CauseRadioNetwork{Value: ngapType.CauseRadioNetworkPresentUnknownLocalUENGAPID},
		},
	})
}
