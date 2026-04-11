// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package decode_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapType"
)

func validPDUSessionResourceReleaseResponse() *ngapType.PDUSessionResourceReleaseResponse {
	msg := &ngapType.PDUSessionResourceReleaseResponse{}

	msg.ProtocolIEs.List = []ngapType.PDUSessionResourceReleaseResponseIEs{
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDAMFUENGAPID},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
			Value: ngapType.PDUSessionResourceReleaseResponseIEsValue{
				Present:     ngapType.PDUSessionResourceReleaseResponseIEsPresentAMFUENGAPID,
				AMFUENGAPID: &ngapType.AMFUENGAPID{Value: 7},
			},
		},
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDRANUENGAPID},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
			Value: ngapType.PDUSessionResourceReleaseResponseIEsValue{
				Present:     ngapType.PDUSessionResourceReleaseResponseIEsPresentRANUENGAPID,
				RANUENGAPID: &ngapType.RANUENGAPID{Value: 8},
			},
		},
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDPDUSessionResourceReleasedListRelRes},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
			Value: ngapType.PDUSessionResourceReleaseResponseIEsValue{
				Present: ngapType.PDUSessionResourceReleaseResponseIEsPresentPDUSessionResourceReleasedListRelRes,
				PDUSessionResourceReleasedListRelRes: &ngapType.PDUSessionResourceReleasedListRelRes{
					List: []ngapType.PDUSessionResourceReleasedItemRelRes{
						{
							PDUSessionID: ngapType.PDUSessionID{Value: 1},
							PDUSessionResourceReleaseResponseTransfer: aper.OctetString{0xAA},
						},
					},
				},
			},
		},
	}

	return msg
}

func TestDecodePDUSessionResourceReleaseResponse_Happy(t *testing.T) {
	out, report := decode.DecodePDUSessionResourceReleaseResponse(validPDUSessionResourceReleaseResponse())
	if report != nil {
		t.Fatalf("expected nil report, got %+v", report)
	}

	if out.AMFUENGAPID == nil || *out.AMFUENGAPID != 7 {
		t.Errorf("AMFUENGAPID = %v, want 7", out.AMFUENGAPID)
	}

	if out.RANUENGAPID == nil || *out.RANUENGAPID != 8 {
		t.Errorf("RANUENGAPID = %v, want 8", out.RANUENGAPID)
	}

	if len(out.PDUSessionResourceReleasedItems) != 1 {
		t.Fatalf("expected 1 released item, got %d", len(out.PDUSessionResourceReleasedItems))
	}

	if out.PDUSessionResourceReleasedItems[0].PDUSessionID.Value != 1 {
		t.Errorf("PDUSessionID = %d, want 1", out.PDUSessionResourceReleasedItems[0].PDUSessionID.Value)
	}
}

func TestDecodePDUSessionResourceReleaseResponse_NilBody(t *testing.T) {
	_, report := decode.DecodePDUSessionResourceReleaseResponse(nil)
	if report == nil || !report.Fatal() {
		t.Fatalf("expected fatal report, got %+v", report)
	}

	if !report.ProcedureRejected {
		t.Error("expected ProcedureRejected to be set")
	}
}

func TestDecodePDUSessionResourceReleaseResponse_EmptyIEsNonFatal(t *testing.T) {
	_, report := decode.DecodePDUSessionResourceReleaseResponse(&ngapType.PDUSessionResourceReleaseResponse{})
	if report == nil {
		t.Fatal("expected non-nil report")
	}

	if report.Fatal() {
		t.Errorf("expected non-fatal report, got %+v", report)
	}

	wantIEs := map[int64]bool{
		ngapType.ProtocolIEIDAMFUENGAPID:                          true,
		ngapType.ProtocolIEIDRANUENGAPID:                          true,
		ngapType.ProtocolIEIDPDUSessionResourceReleasedListRelRes: true,
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

func TestDecodePDUSessionResourceReleaseResponse_NilAMFUENGAPIDValueNonFatal(t *testing.T) {
	msg := validPDUSessionResourceReleaseResponse()
	for i := range msg.ProtocolIEs.List {
		if msg.ProtocolIEs.List[i].Id.Value == ngapType.ProtocolIEIDAMFUENGAPID {
			msg.ProtocolIEs.List[i].Value.AMFUENGAPID = nil
		}
	}

	out, report := decode.DecodePDUSessionResourceReleaseResponse(msg)
	if report == nil {
		t.Fatal("expected non-nil report")
	}

	if report.Fatal() {
		t.Errorf("expected non-fatal report, got %+v", report)
	}

	if out.AMFUENGAPID != nil {
		t.Errorf("expected nil AMFUENGAPID on malformed input, got %v", out.AMFUENGAPID)
	}

	if len(report.Items) != 1 {
		t.Fatalf("expected 1 report item (no double-report), got %d", len(report.Items))
	}
}

func TestDecodePDUSessionResourceReleaseResponse_DuplicateIELastWins(t *testing.T) {
	msg := validPDUSessionResourceReleaseResponse()

	msg.ProtocolIEs.List = append(msg.ProtocolIEs.List, ngapType.PDUSessionResourceReleaseResponseIEs{
		Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDRANUENGAPID},
		Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
		Value: ngapType.PDUSessionResourceReleaseResponseIEsValue{
			Present:     ngapType.PDUSessionResourceReleaseResponseIEsPresentRANUENGAPID,
			RANUENGAPID: &ngapType.RANUENGAPID{Value: 4242},
		},
	})

	out, report := decode.DecodePDUSessionResourceReleaseResponse(msg)
	if report != nil {
		t.Fatalf("expected nil report, got %+v", report)
	}

	if out.RANUENGAPID == nil || *out.RANUENGAPID != 4242 {
		t.Errorf("RANUENGAPID = %v, want 4242 (last-wins)", out.RANUENGAPID)
	}
}
