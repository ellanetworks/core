// Copyright 2026 Ella Networks

package ngap_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/ngap/ngapType"
)

func TestHandleLocationReport_EmptyIEs(t *testing.T) {
	ran := newTestRadio()
	amfInstance := newTestAMF()
	msg := &ngapType.LocationReport{}

	assertNoPanic(t, "HandleLocationReport(empty IEs)", func() {
		ngap.HandleLocationReport(context.Background(), amfInstance, ran, msg)
	})
}

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

	msg := &ngapType.LocationReport{}

	// Add mandatory RANUENGAPID IE
	msg.ProtocolIEs.List = append(msg.ProtocolIEs.List, ngapType.LocationReportIEs{
		Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDRANUENGAPID},
		Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentReject},
		Value: ngapType.LocationReportIEsValue{
			Present:     ngapType.LocationReportIEsPresentRANUENGAPID,
			RANUENGAPID: &ngapType.RANUENGAPID{Value: 1},
		},
	})

	// Add LocationReportingRequestType with EventType = UePresenceInAreaOfInterest
	// but deliberately omit the UEPresenceInAreaOfInterestList IE.
	msg.ProtocolIEs.List = append(msg.ProtocolIEs.List, ngapType.LocationReportIEs{
		Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDLocationReportingRequestType},
		Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
		Value: ngapType.LocationReportIEsValue{
			Present: ngapType.LocationReportIEsPresentLocationReportingRequestType,
			LocationReportingRequestType: &ngapType.LocationReportingRequestType{
				EventType: ngapType.EventType{
					Value: ngapType.EventTypePresentUePresenceInAreaOfInterest,
				},
				ReportArea: ngapType.ReportArea{
					Value: ngapType.ReportAreaPresentCell,
				},
			},
		},
	})

	// No UEPresenceInAreaOfInterestList IE is added — it is optional.
	// The handler must not panic when it is absent.
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

	msg := &ngapType.LocationReport{}

	msg.ProtocolIEs.List = append(msg.ProtocolIEs.List, ngapType.LocationReportIEs{
		Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDRANUENGAPID},
		Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentReject},
		Value: ngapType.LocationReportIEsValue{
			Present:     ngapType.LocationReportIEsPresentRANUENGAPID,
			RANUENGAPID: &ngapType.RANUENGAPID{Value: 1},
		},
	})

	// EventType = StopUePresenceInAreaOfInterest, but
	// LocationReportingReferenceIDToBeCancelled is nil (optional, omitted).
	msg.ProtocolIEs.List = append(msg.ProtocolIEs.List, ngapType.LocationReportIEs{
		Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDLocationReportingRequestType},
		Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
		Value: ngapType.LocationReportIEsValue{
			Present: ngapType.LocationReportIEsPresentLocationReportingRequestType,
			LocationReportingRequestType: &ngapType.LocationReportingRequestType{
				EventType: ngapType.EventType{
					Value: ngapType.EventTypePresentStopUePresenceInAreaOfInterest,
				},
				ReportArea: ngapType.ReportArea{
					Value: ngapType.ReportAreaPresentCell,
				},
				// LocationReportingReferenceIDToBeCancelled deliberately nil
			},
		},
	})

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

	msg := &ngapType.LocationReport{}

	msg.ProtocolIEs.List = append(msg.ProtocolIEs.List, ngapType.LocationReportIEs{
		Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDRANUENGAPID},
		Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentReject},
		Value: ngapType.LocationReportIEsValue{
			Present:     ngapType.LocationReportIEsPresentRANUENGAPID,
			RANUENGAPID: &ngapType.RANUENGAPID{Value: 1},
		},
	})

	// EventType = UePresenceInAreaOfInterest with AreaOfInterestList nil (omitted).
	msg.ProtocolIEs.List = append(msg.ProtocolIEs.List, ngapType.LocationReportIEs{
		Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDLocationReportingRequestType},
		Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
		Value: ngapType.LocationReportIEsValue{
			Present: ngapType.LocationReportIEsPresentLocationReportingRequestType,
			LocationReportingRequestType: &ngapType.LocationReportingRequestType{
				EventType: ngapType.EventType{
					Value: ngapType.EventTypePresentUePresenceInAreaOfInterest,
				},
				ReportArea: ngapType.ReportArea{
					Value: ngapType.ReportAreaPresentCell,
				},
				// AreaOfInterestList deliberately nil
			},
		},
	})

	// Provide a non-nil UEPresenceInAreaOfInterestList so the outer loop
	// executes and the inner loop hits the nil AreaOfInterestList.
	msg.ProtocolIEs.List = append(msg.ProtocolIEs.List, ngapType.LocationReportIEs{
		Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDUEPresenceInAreaOfInterestList},
		Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
		Value: ngapType.LocationReportIEsValue{
			Present: ngapType.LocationReportIEsPresentUEPresenceInAreaOfInterestList,
			UEPresenceInAreaOfInterestList: &ngapType.UEPresenceInAreaOfInterestList{
				List: []ngapType.UEPresenceInAreaOfInterestItem{
					{
						LocationReportingReferenceID: ngapType.LocationReportingReferenceID{Value: 1},
						UEPresence:                   ngapType.UEPresence{Value: ngapType.UEPresencePresentIn},
					},
				},
			},
		},
	})

	assertNoPanic(t, "HandleLocationReport(UePresence with nil AreaOfInterestList)", func() {
		ngap.HandleLocationReport(context.Background(), amfInstance, ran, msg)
	})
}
