// Copyright 2026 Ella Networks

package ngap_test

import (
	"testing"

	amfContext "github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/free5gc/ngap/ngapType"
)

func TestHandleUEContextModificationFailure_MissingAMFUENGAPID(t *testing.T) {
	ran := newTestRadio()
	amf := &amfContext.AMF{}
	msg := &ngapType.UEContextModificationFailure{}
	msg.ProtocolIEs.List = append(msg.ProtocolIEs.List, ngapType.UEContextModificationFailureIEs{
		Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDRANUENGAPID},
		Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
		Value: ngapType.UEContextModificationFailureIEsValue{
			Present:     ngapType.UEContextModificationFailureIEsPresentRANUENGAPID,
			RANUENGAPID: &ngapType.RANUENGAPID{Value: 1},
		},
	})

	assertNoPanic(t, "HandleUEContextModificationFailure(missing AMFUENGAPID)", func() {
		ngap.HandleUEContextModificationFailure(amf, ran, msg)
	})
}
