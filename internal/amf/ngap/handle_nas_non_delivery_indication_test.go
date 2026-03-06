// Copyright 2026 Ella Networks

package ngap_test

import (
	"context"
	"testing"

	amfContext "github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/ngap/ngapType"
)

func TestHandleNasNonDeliveryIndication_EmptyIEs(t *testing.T) {
	ran := newTestRadio()
	amf := newTestAMF()
	msg := &ngapType.NASNonDeliveryIndication{}

	assertNoPanic(t, "HandleNasNonDeliveryIndication(empty IEs)", func() {
		ngap.HandleNasNonDeliveryIndication(context.Background(), amf, ran, msg)
	})
}

func TestHandleNasNonDeliveryIndication_MissingCauseAndNASPDU(t *testing.T) {
	ran := newTestRadio()
	amf := newTestAMF()
	ranUe := &amfContext.RanUe{
		RanUeNgapID: 1,
		AmfUeNgapID: 1,
		Radio:       ran,
		Log:         logger.AmfLog,
	}
	ran.RanUEs[1] = ranUe
	msg := &ngapType.NASNonDeliveryIndication{}
	msg.ProtocolIEs.List = append(msg.ProtocolIEs.List, ngapType.NASNonDeliveryIndicationIEs{
		Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDRANUENGAPID},
		Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentReject},
		Value: ngapType.NASNonDeliveryIndicationIEsValue{
			Present:     ngapType.NASNonDeliveryIndicationIEsPresentRANUENGAPID,
			RANUENGAPID: &ngapType.RANUENGAPID{Value: 1},
		},
	})

	assertNoPanic(t, "HandleNasNonDeliveryIndication(missing cause+NASPDU)", func() {
		ngap.HandleNasNonDeliveryIndication(context.Background(), amf, ran, msg)
	})
}
