// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package decode_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/free5gc/ngap/ngapType"
)

func validLocationReport() *ngapType.LocationReport {
	msg := &ngapType.LocationReport{}

	msg.ProtocolIEs.List = []ngapType.LocationReportIEs{
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDAMFUENGAPID},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentReject},
			Value: ngapType.LocationReportIEsValue{
				Present:     ngapType.LocationReportIEsPresentAMFUENGAPID,
				AMFUENGAPID: &ngapType.AMFUENGAPID{Value: 11},
			},
		},
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDRANUENGAPID},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentReject},
			Value: ngapType.LocationReportIEsValue{
				Present:     ngapType.LocationReportIEsPresentRANUENGAPID,
				RANUENGAPID: &ngapType.RANUENGAPID{Value: 22},
			},
		},
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDUserLocationInformation},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
			Value: ngapType.LocationReportIEsValue{
				Present:                 ngapType.LocationReportIEsPresentUserLocationInformation,
				UserLocationInformation: &ngapType.UserLocationInformation{Present: ngapType.UserLocationInformationPresentUserLocationInformationNR},
			},
		},
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDLocationReportingRequestType},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
			Value: ngapType.LocationReportIEsValue{
				Present: ngapType.LocationReportIEsPresentLocationReportingRequestType,
				LocationReportingRequestType: &ngapType.LocationReportingRequestType{
					EventType:  ngapType.EventType{Value: ngapType.EventTypePresentDirect},
					ReportArea: ngapType.ReportArea{Value: ngapType.ReportAreaPresentCell},
				},
			},
		},
	}

	return msg
}

func TestDecodeLocationReport_Happy(t *testing.T) {
	out, report := decode.DecodeLocationReport(validLocationReport())
	if report != nil {
		t.Fatalf("expected nil report, got %+v", report)
	}

	if out.AMFUENGAPID != 11 {
		t.Errorf("AMFUENGAPID = %d, want 11", out.AMFUENGAPID)
	}

	if out.RANUENGAPID != 22 {
		t.Errorf("RANUENGAPID = %d, want 22", out.RANUENGAPID)
	}

	if out.UserLocationInformation == nil {
		t.Error("expected non-nil UserLocationInformation")
	}

	if out.LocationReportingRequestType == nil {
		t.Error("expected non-nil LocationReportingRequestType")
	}
}

func TestDecodeLocationReport_NilBody(t *testing.T) {
	_, report := decode.DecodeLocationReport(nil)
	if report == nil || !report.Fatal() {
		t.Fatalf("expected fatal report; got %+v", report)
	}

	if !report.ProcedureRejected {
		t.Error("expected ProcedureRejected to be set")
	}
}

func TestDecodeLocationReport_EmptyIEs(t *testing.T) {
	_, report := decode.DecodeLocationReport(&ngapType.LocationReport{})
	if report == nil || !report.Fatal() {
		t.Fatalf("expected fatal report; got %+v", report)
	}

	wantIEs := map[int64]bool{
		ngapType.ProtocolIEIDAMFUENGAPID:                  true,
		ngapType.ProtocolIEIDRANUENGAPID:                  true,
		ngapType.ProtocolIEIDUserLocationInformation:      true,
		ngapType.ProtocolIEIDLocationReportingRequestType: true,
	}

	gotIEs := make(map[int64]bool)
	for _, item := range report.Items {
		gotIEs[item.IEID] = true
	}

	for id := range wantIEs {
		if !gotIEs[id] {
			t.Errorf("missing report item for IE %d", id)
		}
	}
}

func TestDecodeLocationReport_NilRANUENGAPIDValue(t *testing.T) {
	msg := validLocationReport()
	for i := range msg.ProtocolIEs.List {
		if msg.ProtocolIEs.List[i].Id.Value == ngapType.ProtocolIEIDRANUENGAPID {
			msg.ProtocolIEs.List[i].Value.RANUENGAPID = nil
		}
	}

	_, report := decode.DecodeLocationReport(msg)
	if report == nil || !report.Fatal() {
		t.Fatalf("expected fatal report; got %+v", report)
	}

	if len(report.Items) != 1 {
		t.Fatalf("expected 1 report item, got %d", len(report.Items))
	}
}

func TestDecodeLocationReport_NilLRRTValueNonFatal(t *testing.T) {
	msg := validLocationReport()
	for i := range msg.ProtocolIEs.List {
		if msg.ProtocolIEs.List[i].Id.Value == ngapType.ProtocolIEIDLocationReportingRequestType {
			msg.ProtocolIEs.List[i].Value.LocationReportingRequestType = nil
		}
	}

	out, report := decode.DecodeLocationReport(msg)
	if report == nil {
		t.Fatal("expected non-nil report")
	}

	if report.Fatal() {
		t.Errorf("expected non-fatal report; got %+v", report)
	}

	if out.LocationReportingRequestType != nil {
		t.Error("expected nil LocationReportingRequestType")
	}
}

func TestDecodeLocationReport_DuplicateIELastWins(t *testing.T) {
	msg := validLocationReport()
	msg.ProtocolIEs.List = append(msg.ProtocolIEs.List, ngapType.LocationReportIEs{
		Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDRANUENGAPID},
		Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentReject},
		Value: ngapType.LocationReportIEsValue{
			Present:     ngapType.LocationReportIEsPresentRANUENGAPID,
			RANUENGAPID: &ngapType.RANUENGAPID{Value: 999},
		},
	})

	out, report := decode.DecodeLocationReport(msg)
	if report != nil {
		t.Fatalf("expected nil report, got %+v", report)
	}

	if out.RANUENGAPID != 999 {
		t.Errorf("expected last-wins RANUENGAPID=999, got %d", out.RANUENGAPID)
	}
}
