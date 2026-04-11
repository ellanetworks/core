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

func TestHandleLocationReport_MissingLocationReportingRequestType(t *testing.T) {
	ran := newTestRadio()
	amfInstance := newTestAMF()
	ranUe := &amf.RanUe{
		RanUeNgapID: 1,
		AmfUeNgapID: 1,
		Radio:       ran,
		Log:         logger.AmfLog,
	}
	ran.RanUEs[1] = ranUe

	msg := decode.LocationReport{
		AMFUENGAPID: 1,
		RANUENGAPID: 1,
	}

	assertNoPanic(t, "HandleLocationReport(missing LocationReportingRequestType)", func() {
		ngap.HandleLocationReport(context.Background(), amfInstance, ran, msg)
	})
}

// TestHandleLocationReport_UePresenceInAreaOfInterest_NilList verifies that
// a LocationReport with EventType=UePresenceInAreaOfInterest but without the
// optional UEPresenceInAreaOfInterestList IE does NOT panic.
// This is a regression test for a nil pointer dereference (CVE-like DoS).
func TestHandleLocationReport_UePresenceInAreaOfInterest_NilList(t *testing.T) {
	ran := newTestRadio()
	amfInstance := newTestAMF()
	ranUe := &amf.RanUe{
		RanUeNgapID: 1,
		AmfUeNgapID: 1,
		Radio:       ran,
		Log:         logger.AmfLog,
	}
	ran.RanUEs[1] = ranUe

	msg := decode.LocationReport{
		AMFUENGAPID: 1,
		RANUENGAPID: 1,
		LocationReportingRequestType: &ngapType.LocationReportingRequestType{
			EventType: ngapType.EventType{
				Value: ngapType.EventTypePresentUePresenceInAreaOfInterest,
			},
			ReportArea: ngapType.ReportArea{
				Value: ngapType.ReportAreaPresentCell,
			},
		},
	}

	assertNoPanic(t, "HandleLocationReport(UePresenceInAreaOfInterest with nil list)", func() {
		ngap.HandleLocationReport(context.Background(), amfInstance, ran, msg)
	})
}

// TestHandleLocationReport_StopUePresence_NilReferenceIDToBeCancelled verifies
// that a LocationReport with EventType=StopUePresenceInAreaOfInterest but
// without LocationReportingReferenceIDToBeCancelled does NOT panic.
// Reproduces GHSA-f2f3-9cx3-wcmf Bug 1.
func TestHandleLocationReport_StopUePresence_NilReferenceIDToBeCancelled(t *testing.T) {
	ran := newTestRadio()
	amfInstance := newTestAMF()
	ranUe := &amf.RanUe{
		RanUeNgapID: 1,
		AmfUeNgapID: 1,
		Radio:       ran,
		Log:         logger.AmfLog,
	}
	ran.RanUEs[1] = ranUe

	msg := decode.LocationReport{
		AMFUENGAPID: 1,
		RANUENGAPID: 1,
		LocationReportingRequestType: &ngapType.LocationReportingRequestType{
			EventType: ngapType.EventType{
				Value: ngapType.EventTypePresentStopUePresenceInAreaOfInterest,
			},
			ReportArea: ngapType.ReportArea{
				Value: ngapType.ReportAreaPresentCell,
			},
		},
	}

	assertNoPanic(t, "HandleLocationReport(StopUePresence with nil ReferenceIDToBeCancelled)", func() {
		ngap.HandleLocationReport(context.Background(), amfInstance, ran, msg)
	})
}

// TestHandleLocationReport_UePresence_NilAreaOfInterestList verifies that
// a LocationReport with EventType=UePresenceInAreaOfInterest and a non-nil
// UEPresenceInAreaOfInterestList but nil AreaOfInterestList does NOT panic.
// Reproduces GHSA-f2f3-9cx3-wcmf Bug 2.
func TestHandleLocationReport_UePresence_NilAreaOfInterestList(t *testing.T) {
	ran := newTestRadio()
	amfInstance := newTestAMF()
	ranUe := &amf.RanUe{
		RanUeNgapID: 1,
		AmfUeNgapID: 1,
		Radio:       ran,
		Log:         logger.AmfLog,
	}
	ran.RanUEs[1] = ranUe

	msg := decode.LocationReport{
		AMFUENGAPID: 1,
		RANUENGAPID: 1,
		LocationReportingRequestType: &ngapType.LocationReportingRequestType{
			EventType: ngapType.EventType{
				Value: ngapType.EventTypePresentUePresenceInAreaOfInterest,
			},
			ReportArea: ngapType.ReportArea{
				Value: ngapType.ReportAreaPresentCell,
			},
		},
		UEPresenceInAreaOfInterestList: &ngapType.UEPresenceInAreaOfInterestList{
			List: []ngapType.UEPresenceInAreaOfInterestItem{
				{
					LocationReportingReferenceID: ngapType.LocationReportingReferenceID{Value: 1},
					UEPresence:                   ngapType.UEPresence{Value: ngapType.UEPresencePresentIn},
				},
			},
		},
	}

	assertNoPanic(t, "HandleLocationReport(UePresence with nil AreaOfInterestList)", func() {
		ngap.HandleLocationReport(context.Background(), amfInstance, ran, msg)
	})
}
