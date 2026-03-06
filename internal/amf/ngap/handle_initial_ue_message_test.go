// Copyright 2026 Ella Networks

package ngap_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/ngap/ngapType"
)

func TestHandleInitialUEMessage_EmptyIEs(t *testing.T) {
	ran := newTestRadio()
	ran.RanID = &models.GlobalRanNodeID{}
	amf := newTestAMF()
	msg := &ngapType.InitialUEMessage{}

	assertNoPanic(t, "HandleInitialUEMessage(empty IEs)", func() {
		ngap.HandleInitialUEMessage(context.Background(), amf, ran, msg)
	})
}

func TestHandleInitialUEMessage_NilValueIEs(t *testing.T) {
	ran := newTestRadio()
	ran.RanID = &models.GlobalRanNodeID{}
	amf := newTestAMF()
	msg := &ngapType.InitialUEMessage{}
	msg.ProtocolIEs.List = append(msg.ProtocolIEs.List, ngapType.InitialUEMessageIEs{
		Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDRANUENGAPID},
		Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentReject},
		Value: ngapType.InitialUEMessageIEsValue{
			Present:     ngapType.InitialUEMessageIEsPresentRANUENGAPID,
			RANUENGAPID: nil,
		},
	})
	msg.ProtocolIEs.List = append(msg.ProtocolIEs.List, ngapType.InitialUEMessageIEs{
		Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDNASPDU},
		Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentReject},
		Value: ngapType.InitialUEMessageIEsValue{
			Present: ngapType.InitialUEMessageIEsPresentNASPDU,
			NASPDU:  nil,
		},
	})

	assertNoPanic(t, "HandleInitialUEMessage(nil value IEs)", func() {
		ngap.HandleInitialUEMessage(context.Background(), amf, ran, msg)
	})
}
