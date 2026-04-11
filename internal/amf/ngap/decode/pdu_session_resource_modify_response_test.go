// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package decode_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/free5gc/ngap/ngapType"
)

func validPDUSessionResourceModifyResponse() *ngapType.PDUSessionResourceModifyResponse {
	msg := &ngapType.PDUSessionResourceModifyResponse{}

	msg.ProtocolIEs.List = []ngapType.PDUSessionResourceModifyResponseIEs{
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDAMFUENGAPID},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
			Value: ngapType.PDUSessionResourceModifyResponseIEsValue{
				Present:     ngapType.PDUSessionResourceModifyResponseIEsPresentAMFUENGAPID,
				AMFUENGAPID: &ngapType.AMFUENGAPID{Value: 50},
			},
		},
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDRANUENGAPID},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
			Value: ngapType.PDUSessionResourceModifyResponseIEsValue{
				Present:     ngapType.PDUSessionResourceModifyResponseIEsPresentRANUENGAPID,
				RANUENGAPID: &ngapType.RANUENGAPID{Value: 60},
			},
		},
	}

	return msg
}

func TestDecodePDUSessionResourceModifyResponse_Happy(t *testing.T) {
	out, report := decode.DecodePDUSessionResourceModifyResponse(validPDUSessionResourceModifyResponse())
	if report != nil {
		t.Fatalf("expected nil report, got %+v", report)
	}

	if out.AMFUENGAPID == nil || *out.AMFUENGAPID != 50 {
		t.Errorf("AMFUENGAPID = %v, want 50", out.AMFUENGAPID)
	}

	if out.RANUENGAPID == nil || *out.RANUENGAPID != 60 {
		t.Errorf("RANUENGAPID = %v, want 60", out.RANUENGAPID)
	}
}

func TestDecodePDUSessionResourceModifyResponse_NilBody(t *testing.T) {
	_, report := decode.DecodePDUSessionResourceModifyResponse(nil)
	if report == nil || !report.Fatal() {
		t.Fatalf("expected fatal report, got %+v", report)
	}

	if !report.ProcedureRejected {
		t.Error("expected ProcedureRejected to be set")
	}
}

func TestDecodePDUSessionResourceModifyResponse_EmptyIEsNonFatal(t *testing.T) {
	_, report := decode.DecodePDUSessionResourceModifyResponse(&ngapType.PDUSessionResourceModifyResponse{})
	if report == nil {
		t.Fatal("expected non-nil report")
	}

	if report.Fatal() {
		t.Errorf("expected non-fatal report, got %+v", report)
	}
}

func TestDecodePDUSessionResourceModifyResponse_NilAMFUENGAPIDValueNonFatal(t *testing.T) {
	msg := validPDUSessionResourceModifyResponse()
	for i := range msg.ProtocolIEs.List {
		if msg.ProtocolIEs.List[i].Id.Value == ngapType.ProtocolIEIDAMFUENGAPID {
			msg.ProtocolIEs.List[i].Value.AMFUENGAPID = nil
		}
	}

	out, report := decode.DecodePDUSessionResourceModifyResponse(msg)
	if report == nil {
		t.Fatal("expected non-nil report")
	}

	if report.Fatal() {
		t.Errorf("expected non-fatal report, got %+v", report)
	}

	if out.AMFUENGAPID != nil {
		t.Errorf("expected nil AMFUENGAPID, got %v", out.AMFUENGAPID)
	}

	if len(report.Items) != 1 {
		t.Fatalf("expected 1 report item (no double-report), got %d", len(report.Items))
	}
}

func TestDecodePDUSessionResourceModifyResponse_DuplicateIELastWins(t *testing.T) {
	msg := validPDUSessionResourceModifyResponse()

	msg.ProtocolIEs.List = append(msg.ProtocolIEs.List, ngapType.PDUSessionResourceModifyResponseIEs{
		Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDRANUENGAPID},
		Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
		Value: ngapType.PDUSessionResourceModifyResponseIEsValue{
			Present:     ngapType.PDUSessionResourceModifyResponseIEsPresentRANUENGAPID,
			RANUENGAPID: &ngapType.RANUENGAPID{Value: 9999},
		},
	})

	out, report := decode.DecodePDUSessionResourceModifyResponse(msg)
	if report != nil {
		t.Fatalf("expected nil report, got %+v", report)
	}

	if out.RANUENGAPID == nil || *out.RANUENGAPID != 9999 {
		t.Errorf("RANUENGAPID = %v, want 9999 (last-wins)", out.RANUENGAPID)
	}
}
