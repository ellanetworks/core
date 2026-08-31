// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/ngap"
)

func TestHandleLocationReport_MissingLocationReportingRequestType(t *testing.T) {
	amfInstance := newTestAMF()
	ran := newTestRadio(amfInstance)

	amf.NewUeConnForTest(ran, 1, 1, logger.AmfLog)

	msg := &ngap.LocationReport{
		AMFUENGAPID: 1,
		RANUENGAPID: 1,
	}

	HandleLocationReport(context.Background(), amfInstance, ran, msg)

	sender := ran.Conn.(*fakeNGAPSender)
	if len(sender.SentErrorIndications) != 0 {
		t.Fatalf("expected no ErrorIndication, got %d", len(sender.SentErrorIndications))
	}
}

func TestHandleLocationReport_UnknownAmfUeNgapID(t *testing.T) {
	amfInstance := newTestAMF()
	ran := newTestRadio(amfInstance)
	sender := ran.Conn.(*fakeNGAPSender)

	msg := &ngap.LocationReport{
		AMFUENGAPID: 999,
		RANUENGAPID: 99,
		LocationReportingRequestType: &ngap.LocationReportingRequestType{
			EventType:  ngap.EventTypeDirect,
			ReportArea: ngap.ReportAreaCell,
		},
	}

	HandleLocationReport(context.Background(), amfInstance, ran, msg)

	errInd := assertSingleErrorIndication(t, sender, ngap.CauseRadioNetworkUnknownLocalUENGAPID)
	assertErrorIndicationEchoesIDs(t, errInd, 999, 99)
}

func TestHandleLocationReport_UePresenceInAreaOfInterest_NilList(t *testing.T) {
	amfInstance := newTestAMF()
	ran := newTestRadio(amfInstance)

	amf.NewUeConnForTest(ran, 1, 1, logger.AmfLog)

	msg := &ngap.LocationReport{
		AMFUENGAPID: 1,
		RANUENGAPID: 1,
		LocationReportingRequestType: &ngap.LocationReportingRequestType{
			EventType:  ngap.EventTypeUEPresenceInAreaOfInterest,
			ReportArea: ngap.ReportAreaCell,
		},
	}

	HandleLocationReport(context.Background(), amfInstance, ran, msg)

	sender := ran.Conn.(*fakeNGAPSender)
	if len(sender.SentErrorIndications) != 0 {
		t.Fatalf("expected no ErrorIndication, got %d", len(sender.SentErrorIndications))
	}
}

func TestHandleLocationReport_StopUePresence_NilReferenceIDToBeCancelled(t *testing.T) {
	amfInstance := newTestAMF()
	ran := newTestRadio(amfInstance)

	amf.NewUeConnForTest(ran, 1, 1, logger.AmfLog)

	msg := &ngap.LocationReport{
		AMFUENGAPID: 1,
		RANUENGAPID: 1,
		LocationReportingRequestType: &ngap.LocationReportingRequestType{
			EventType:  ngap.EventTypeStopUEPresenceInAreaOfInterest,
			ReportArea: ngap.ReportAreaCell,
		},
	}

	HandleLocationReport(context.Background(), amfInstance, ran, msg)

	sender := ran.Conn.(*fakeNGAPSender)
	if len(sender.SentErrorIndications) != 0 {
		t.Fatalf("expected no ErrorIndication, got %d", len(sender.SentErrorIndications))
	}
}

func TestHandleLocationReport_UEPresenceWithoutRequestedArea(t *testing.T) {
	amfInstance := newTestAMF()
	ran := newTestRadio(amfInstance)

	amf.NewUeConnForTest(ran, 1, 1, logger.AmfLog)

	msg := &ngap.LocationReport{
		AMFUENGAPID: 1,
		RANUENGAPID: 1,
		LocationReportingRequestType: &ngap.LocationReportingRequestType{
			EventType:  ngap.EventTypeUEPresenceInAreaOfInterest,
			ReportArea: ngap.ReportAreaCell,
		},
		UEPresenceInAreaOfInterestList: ngap.UEPresenceInAreaOfInterestList{
			{LocationReportingReferenceID: 1, UEPresence: ngap.UEPresenceIn},
		},
	}

	HandleLocationReport(context.Background(), amfInstance, ran, msg)

	sender := ran.Conn.(*fakeNGAPSender)
	if len(sender.SentErrorIndications) != 0 {
		t.Fatalf("expected no ErrorIndication, got %d", len(sender.SentErrorIndications))
	}
}
