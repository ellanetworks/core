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

// TestHandleLocationReport_UnknownAmfUeNgapID verifies that a Location Report
// whose AMF UE NGAP ID the AMF never allocated draws an Error Indication with the
// received AP IDs (TS 38.413).
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

// TestHandleLocationReport_UePresenceInAreaOfInterest_NilList verifies that
// a LocationReport with EventType=UePresenceInAreaOfInterest but without the
// optional UEPresenceInAreaOfInterestList IE does NOT panic.
// This is a regression test for a nil pointer dereference (CVE-like DoS).
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

// TestHandleLocationReport_StopUePresence_NilReferenceIDToBeCancelled verifies
// that a LocationReport with EventType=StopUePresenceInAreaOfInterest but
// without LocationReportingReferenceIDToBeCancelled does NOT panic.
// Reproduces GHSA-f2f3-9cx3-wcmf Bug 1.
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

// A LocationReport naming UE presences the AMF never asked about must not
// panic. Originally GHSA-f2f3-9cx3-wcmf Bug 2, a nil AreaOfInterestList deref;
// the library no longer models an areaOfInterestList at all, so the pairing
// that crashed cannot be built — this now guards the presence walk itself.
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
