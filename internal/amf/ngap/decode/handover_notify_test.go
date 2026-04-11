// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package decode_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/free5gc/ngap/ngapType"
)

func validHandoverNotify() *ngapType.HandoverNotify {
	msg := &ngapType.HandoverNotify{}

	msg.ProtocolIEs.List = []ngapType.HandoverNotifyIEs{
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDAMFUENGAPID},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentReject},
			Value: ngapType.HandoverNotifyIEsValue{
				Present:     ngapType.HandoverNotifyIEsPresentAMFUENGAPID,
				AMFUENGAPID: &ngapType.AMFUENGAPID{Value: 42},
			},
		},
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDRANUENGAPID},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentReject},
			Value: ngapType.HandoverNotifyIEsValue{
				Present:     ngapType.HandoverNotifyIEsPresentRANUENGAPID,
				RANUENGAPID: &ngapType.RANUENGAPID{Value: 7},
			},
		},
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDUserLocationInformation},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
			Value: ngapType.HandoverNotifyIEsValue{
				Present:                 ngapType.HandoverNotifyIEsPresentUserLocationInformation,
				UserLocationInformation: &ngapType.UserLocationInformation{Present: ngapType.UserLocationInformationPresentUserLocationInformationNR},
			},
		},
	}

	return msg
}

func TestDecodeHandoverNotify_Happy(t *testing.T) {
	out, report := decode.DecodeHandoverNotify(validHandoverNotify())
	if report != nil {
		t.Fatalf("expected nil report, got %+v", report)
	}

	if out.AMFUENGAPID != 42 {
		t.Errorf("AMFUENGAPID = %d, want 42", out.AMFUENGAPID)
	}

	if out.RANUENGAPID != 7 {
		t.Errorf("RANUENGAPID = %d, want 7", out.RANUENGAPID)
	}

	if out.UserLocationInformation == nil {
		t.Error("expected non-nil UserLocationInformation")
	}
}

func TestDecodeHandoverNotify_NilBody(t *testing.T) {
	_, report := decode.DecodeHandoverNotify(nil)
	if report == nil || !report.Fatal() {
		t.Fatalf("expected fatal report; got %+v", report)
	}

	if !report.ProcedureRejected {
		t.Error("expected ProcedureRejected to be set")
	}
}

func TestDecodeHandoverNotify_EmptyIEs(t *testing.T) {
	_, report := decode.DecodeHandoverNotify(&ngapType.HandoverNotify{})
	if report == nil || !report.Fatal() {
		t.Fatalf("expected fatal report; got %+v", report)
	}

	wantIEs := map[int64]bool{
		ngapType.ProtocolIEIDAMFUENGAPID:             true,
		ngapType.ProtocolIEIDRANUENGAPID:             true,
		ngapType.ProtocolIEIDUserLocationInformation: true,
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

func TestDecodeHandoverNotify_NilAMFUENGAPIDValue(t *testing.T) {
	msg := validHandoverNotify()
	for i := range msg.ProtocolIEs.List {
		if msg.ProtocolIEs.List[i].Id.Value == ngapType.ProtocolIEIDAMFUENGAPID {
			msg.ProtocolIEs.List[i].Value.AMFUENGAPID = nil
		}
	}

	_, report := decode.DecodeHandoverNotify(msg)
	if report == nil || !report.Fatal() {
		t.Fatalf("expected fatal report; got %+v", report)
	}

	if len(report.Items) != 1 {
		t.Fatalf("expected 1 report item, got %d", len(report.Items))
	}

	if report.Items[0].IEID != ngapType.ProtocolIEIDAMFUENGAPID {
		t.Errorf("expected AMFUENGAPID item, got IE %d", report.Items[0].IEID)
	}
}

func TestDecodeHandoverNotify_NilULIValueNonFatal(t *testing.T) {
	msg := validHandoverNotify()
	for i := range msg.ProtocolIEs.List {
		if msg.ProtocolIEs.List[i].Id.Value == ngapType.ProtocolIEIDUserLocationInformation {
			msg.ProtocolIEs.List[i].Value.UserLocationInformation = nil
		}
	}

	out, report := decode.DecodeHandoverNotify(msg)
	if report == nil {
		t.Fatal("expected non-nil report")
	}

	if report.Fatal() {
		t.Errorf("expected non-fatal report for malformed ULI; got %+v", report)
	}

	if out.UserLocationInformation != nil {
		t.Error("expected nil UserLocationInformation")
	}
}

func TestDecodeHandoverNotify_DuplicateIELastWins(t *testing.T) {
	msg := validHandoverNotify()
	msg.ProtocolIEs.List = append(msg.ProtocolIEs.List, ngapType.HandoverNotifyIEs{
		Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDRANUENGAPID},
		Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentReject},
		Value: ngapType.HandoverNotifyIEsValue{
			Present:     ngapType.HandoverNotifyIEsPresentRANUENGAPID,
			RANUENGAPID: &ngapType.RANUENGAPID{Value: 999},
		},
	})

	out, report := decode.DecodeHandoverNotify(msg)
	if report != nil {
		t.Fatalf("expected nil report, got %+v", report)
	}

	if out.RANUENGAPID != 999 {
		t.Errorf("expected last-wins RANUENGAPID=999, got %d", out.RANUENGAPID)
	}
}
