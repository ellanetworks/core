// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/ellanetworks/core/internal/logger"
	ngaplib "github.com/ellanetworks/core/ngap"
	"github.com/free5gc/ngap/ngapType"
)

func TestHandleLocationReport_MissingLocationReportingRequestType(t *testing.T) {
	amfInstance := newTestAMF()
	ran := newTestRadio(amfInstance)

	amf.NewUeConnForTest(ran, 1, 1, logger.AmfLog)

	msg := &ngaplib.LocationReport{
		AMFUENGAPID: 1,
		RANUENGAPID: 1,
	}

	ngap.HandleLocationReport(context.Background(), amfInstance, ran, msg)

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

	msg := &ngaplib.LocationReport{
		AMFUENGAPID: 999,
		RANUENGAPID: 99,
		LocationReportingRequestType: &ngaplib.LocationReportingRequestType{
			EventType:  ngaplib.EventTypeDirect,
			ReportArea: ngaplib.ReportAreaCell,
		},
	}

	ngap.HandleLocationReport(context.Background(), amfInstance, ran, msg)

	errInd := assertSingleErrorIndication(t, sender, ngapType.CauseRadioNetworkPresentUnknownLocalUENGAPID)
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

	msg := &ngaplib.LocationReport{
		AMFUENGAPID: 1,
		RANUENGAPID: 1,
		LocationReportingRequestType: &ngaplib.LocationReportingRequestType{
			EventType:  ngaplib.EventTypeUEPresenceInAreaOfInterest,
			ReportArea: ngaplib.ReportAreaCell,
		},
	}

	ngap.HandleLocationReport(context.Background(), amfInstance, ran, msg)

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

	msg := &ngaplib.LocationReport{
		AMFUENGAPID: 1,
		RANUENGAPID: 1,
		LocationReportingRequestType: &ngaplib.LocationReportingRequestType{
			EventType:  ngaplib.EventTypeStopUEPresenceInAreaOfInterest,
			ReportArea: ngaplib.ReportAreaCell,
		},
	}

	ngap.HandleLocationReport(context.Background(), amfInstance, ran, msg)

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

	msg := &ngaplib.LocationReport{
		AMFUENGAPID: 1,
		RANUENGAPID: 1,
		LocationReportingRequestType: &ngaplib.LocationReportingRequestType{
			EventType:  ngaplib.EventTypeUEPresenceInAreaOfInterest,
			ReportArea: ngaplib.ReportAreaCell,
		},
		UEPresenceInAreaOfInterestList: ngaplib.UEPresenceInAreaOfInterestList{
			{LocationReportingReferenceID: 1, UEPresence: ngaplib.UEPresenceIn},
		},
	}

	ngap.HandleLocationReport(context.Background(), amfInstance, ran, msg)

	sender := ran.Conn.(*fakeNGAPSender)
	if len(sender.SentErrorIndications) != 0 {
		t.Fatalf("expected no ErrorIndication, got %d", len(sender.SentErrorIndications))
	}
}
