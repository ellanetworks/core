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

func TestHandleLocationReport_EmptyIEs(t *testing.T) {
	ran := newTestRadio()
	amf := newTestAMF()
	msg := &ngapType.LocationReport{}

	assertNoPanic(t, "HandleLocationReport(empty IEs)", func() {
		ngap.HandleLocationReport(context.Background(), amf, ran, msg)
	})
}

func TestHandleLocationReport_MissingLocationReportingRequestType(t *testing.T) {
	ran := newTestRadio()
	amf := newTestAMF()
	ranUe := &amfContext.RanUe{
		RanUeNgapID: 1,
		AmfUeNgapID: 1,
		Radio:       ran,
		Log:         logger.AmfLog,
	}
	ran.RanUEs[1] = ranUe
	msg := &ngapType.LocationReport{}
	msg.ProtocolIEs.List = append(msg.ProtocolIEs.List, ngapType.LocationReportIEs{
		Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDRANUENGAPID},
		Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentReject},
		Value: ngapType.LocationReportIEsValue{
			Present:     ngapType.LocationReportIEsPresentRANUENGAPID,
			RANUENGAPID: &ngapType.RANUENGAPID{Value: 1},
		},
	})

	assertNoPanic(t, "HandleLocationReport(missing LocationReportingRequestType)", func() {
		ngap.HandleLocationReport(context.Background(), amf, ran, msg)
	})
}
