// Copyright 2026 Ella Networks

package ngap_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/free5gc/ngap/ngapType"
)

func TestHandleUEContextModificationResponse_MissingAMFUENGAPID(t *testing.T) {
	ran := newTestRadio()
	amf := newTestAMF()
	msg := &ngapType.UEContextModificationResponse{}
	msg.ProtocolIEs.List = append(msg.ProtocolIEs.List, ngapType.UEContextModificationResponseIEs{
		Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDRANUENGAPID},
		Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
		Value: ngapType.UEContextModificationResponseIEsValue{
			Present:     ngapType.UEContextModificationResponseIEsPresentRANUENGAPID,
			RANUENGAPID: &ngapType.RANUENGAPID{Value: 1},
		},
	})

	assertNoPanic(t, "HandleUEContextModificationResponse(missing AMFUENGAPID)", func() {
		ngap.HandleUEContextModificationResponse(context.Background(), amf, ran, msg)
	})
}
