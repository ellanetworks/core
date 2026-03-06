// Copyright 2026 Ella Networks

package ngap_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/free5gc/ngap/ngapType"
)

func TestHandleUplinkNasTransport_EmptyIEs(t *testing.T) {
	ran := newTestRadio()
	amf := newTestAMF()
	msg := &ngapType.UplinkNASTransport{}

	assertNoPanic(t, "HandleUplinkNasTransport(empty IEs)", func() {
		ngap.HandleUplinkNasTransport(context.Background(), amf, ran, msg)
	})
}

func TestHandleUplinkNasTransport_OnlyRANUENGAPID(t *testing.T) {
	ran := newTestRadio()
	amf := newTestAMF()
	msg := &ngapType.UplinkNASTransport{}
	msg.ProtocolIEs.List = append(msg.ProtocolIEs.List, ngapType.UplinkNASTransportIEs{
		Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDRANUENGAPID},
		Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentReject},
		Value: ngapType.UplinkNASTransportIEsValue{
			Present:     ngapType.UplinkNASTransportIEsPresentRANUENGAPID,
			RANUENGAPID: &ngapType.RANUENGAPID{Value: 1},
		},
	})

	assertNoPanic(t, "HandleUplinkNasTransport(only RANUENGAPID)", func() {
		ngap.HandleUplinkNasTransport(context.Background(), amf, ran, msg)
	})
}
