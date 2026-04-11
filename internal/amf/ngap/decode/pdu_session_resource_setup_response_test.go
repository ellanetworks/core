// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package decode_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapType"
)

func validPDUSessionResourceSetupResponse() *ngapType.PDUSessionResourceSetupResponse {
	msg := &ngapType.PDUSessionResourceSetupResponse{}

	msg.ProtocolIEs.List = []ngapType.PDUSessionResourceSetupResponseIEs{
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDAMFUENGAPID},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
			Value: ngapType.PDUSessionResourceSetupResponseIEsValue{
				Present:     ngapType.PDUSessionResourceSetupResponseIEsPresentAMFUENGAPID,
				AMFUENGAPID: &ngapType.AMFUENGAPID{Value: 1},
			},
		},
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDRANUENGAPID},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
			Value: ngapType.PDUSessionResourceSetupResponseIEsValue{
				Present:     ngapType.PDUSessionResourceSetupResponseIEsPresentRANUENGAPID,
				RANUENGAPID: &ngapType.RANUENGAPID{Value: 2},
			},
		},
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDPDUSessionResourceSetupListSURes},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
			Value: ngapType.PDUSessionResourceSetupResponseIEsValue{
				Present: ngapType.PDUSessionResourceSetupResponseIEsPresentPDUSessionResourceSetupListSURes,
				PDUSessionResourceSetupListSURes: &ngapType.PDUSessionResourceSetupListSURes{
					List: []ngapType.PDUSessionResourceSetupItemSURes{
						{
							PDUSessionID:                            ngapType.PDUSessionID{Value: 1},
							PDUSessionResourceSetupResponseTransfer: aper.OctetString{0xDE},
						},
					},
				},
			},
		},
	}

	return msg
}

func TestDecodePDUSessionResourceSetupResponse_Happy(t *testing.T) {
	out, report := decode.DecodePDUSessionResourceSetupResponse(validPDUSessionResourceSetupResponse())
	if report != nil {
		t.Fatalf("expected nil report, got %+v", report)
	}

	if out.AMFUENGAPID == nil || *out.AMFUENGAPID != 1 {
		t.Errorf("AMFUENGAPID = %v, want 1", out.AMFUENGAPID)
	}

	if out.RANUENGAPID == nil || *out.RANUENGAPID != 2 {
		t.Errorf("RANUENGAPID = %v, want 2", out.RANUENGAPID)
	}

	if len(out.SetupItems) != 1 {
		t.Fatalf("expected 1 setup item, got %d", len(out.SetupItems))
	}
}

func TestDecodePDUSessionResourceSetupResponse_NilBody(t *testing.T) {
	_, report := decode.DecodePDUSessionResourceSetupResponse(nil)
	if report == nil || !report.Fatal() {
		t.Fatalf("expected fatal report, got %+v", report)
	}

	if !report.ProcedureRejected {
		t.Error("expected ProcedureRejected to be set")
	}
}

func TestDecodePDUSessionResourceSetupResponse_EmptyIEsNonFatal(t *testing.T) {
	_, report := decode.DecodePDUSessionResourceSetupResponse(&ngapType.PDUSessionResourceSetupResponse{})
	if report == nil {
		t.Fatal("expected non-nil report")
	}

	if report.Fatal() {
		t.Errorf("expected non-fatal report, got %+v", report)
	}
}

func TestDecodePDUSessionResourceSetupResponse_NilAMFUENGAPIDValueNonFatal(t *testing.T) {
	msg := validPDUSessionResourceSetupResponse()
	for i := range msg.ProtocolIEs.List {
		if msg.ProtocolIEs.List[i].Id.Value == ngapType.ProtocolIEIDAMFUENGAPID {
			msg.ProtocolIEs.List[i].Value.AMFUENGAPID = nil
		}
	}

	out, report := decode.DecodePDUSessionResourceSetupResponse(msg)
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

func TestDecodePDUSessionResourceSetupResponse_DuplicateIELastWins(t *testing.T) {
	msg := validPDUSessionResourceSetupResponse()

	msg.ProtocolIEs.List = append(msg.ProtocolIEs.List, ngapType.PDUSessionResourceSetupResponseIEs{
		Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDAMFUENGAPID},
		Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
		Value: ngapType.PDUSessionResourceSetupResponseIEsValue{
			Present:     ngapType.PDUSessionResourceSetupResponseIEsPresentAMFUENGAPID,
			AMFUENGAPID: &ngapType.AMFUENGAPID{Value: 5151},
		},
	})

	out, report := decode.DecodePDUSessionResourceSetupResponse(msg)
	if report != nil {
		t.Fatalf("expected nil report, got %+v", report)
	}

	if out.AMFUENGAPID == nil || *out.AMFUENGAPID != 5151 {
		t.Errorf("AMFUENGAPID = %v, want 5151 (last-wins)", out.AMFUENGAPID)
	}
}
