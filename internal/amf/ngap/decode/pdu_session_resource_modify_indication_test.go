// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package decode_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/free5gc/ngap/ngapType"
)

func validPDUSessionResourceModifyIndication() *ngapType.PDUSessionResourceModifyIndication {
	msg := &ngapType.PDUSessionResourceModifyIndication{}

	msg.ProtocolIEs.List = []ngapType.PDUSessionResourceModifyIndicationIEs{
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDAMFUENGAPID},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentReject},
			Value: ngapType.PDUSessionResourceModifyIndicationIEsValue{
				Present:     ngapType.PDUSessionResourceModifyIndicationIEsPresentAMFUENGAPID,
				AMFUENGAPID: &ngapType.AMFUENGAPID{Value: 3},
			},
		},
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDRANUENGAPID},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentReject},
			Value: ngapType.PDUSessionResourceModifyIndicationIEsValue{
				Present:     ngapType.PDUSessionResourceModifyIndicationIEsPresentRANUENGAPID,
				RANUENGAPID: &ngapType.RANUENGAPID{Value: 4},
			},
		},
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDPDUSessionResourceModifyListModInd},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentReject},
			Value: ngapType.PDUSessionResourceModifyIndicationIEsValue{
				Present: ngapType.PDUSessionResourceModifyIndicationIEsPresentPDUSessionResourceModifyListModInd,
				PDUSessionResourceModifyListModInd: &ngapType.PDUSessionResourceModifyListModInd{
					List: []ngapType.PDUSessionResourceModifyItemModInd{
						{PDUSessionID: ngapType.PDUSessionID{Value: 1}},
					},
				},
			},
		},
	}

	return msg
}

func TestDecodePDUSessionResourceModifyIndication_Happy(t *testing.T) {
	out, report := decode.DecodePDUSessionResourceModifyIndication(validPDUSessionResourceModifyIndication())
	if report != nil {
		t.Fatalf("expected nil report, got %+v", report)
	}

	if out.AMFUENGAPID != 3 {
		t.Errorf("AMFUENGAPID = %d, want 3", out.AMFUENGAPID)
	}

	if out.RANUENGAPID != 4 {
		t.Errorf("RANUENGAPID = %d, want 4", out.RANUENGAPID)
	}
}

func TestDecodePDUSessionResourceModifyIndication_NilBody(t *testing.T) {
	_, report := decode.DecodePDUSessionResourceModifyIndication(nil)
	if report == nil || !report.Fatal() {
		t.Fatalf("expected fatal report; got %+v", report)
	}

	if !report.ProcedureRejected {
		t.Error("expected ProcedureRejected to be set")
	}
}

func TestDecodePDUSessionResourceModifyIndication_EmptyIEsFatal(t *testing.T) {
	_, report := decode.DecodePDUSessionResourceModifyIndication(&ngapType.PDUSessionResourceModifyIndication{})
	if report == nil || !report.Fatal() {
		t.Fatalf("expected fatal report; got %+v", report)
	}

	wantIEs := map[int64]bool{
		ngapType.ProtocolIEIDAMFUENGAPID:                        true,
		ngapType.ProtocolIEIDRANUENGAPID:                        true,
		ngapType.ProtocolIEIDPDUSessionResourceModifyListModInd: true,
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

func TestDecodePDUSessionResourceModifyIndication_MissingModifyListFatal(t *testing.T) {
	msg := validPDUSessionResourceModifyIndication()

	filtered := msg.ProtocolIEs.List[:0]
	for _, ie := range msg.ProtocolIEs.List {
		if ie.Id.Value == ngapType.ProtocolIEIDPDUSessionResourceModifyListModInd {
			continue
		}

		filtered = append(filtered, ie)
	}

	msg.ProtocolIEs.List = filtered

	_, report := decode.DecodePDUSessionResourceModifyIndication(msg)
	if report == nil || !report.Fatal() {
		t.Fatalf("expected fatal report; got %+v", report)
	}
}

func TestDecodePDUSessionResourceModifyIndication_NilAMFUENGAPIDValue(t *testing.T) {
	msg := validPDUSessionResourceModifyIndication()
	for i := range msg.ProtocolIEs.List {
		if msg.ProtocolIEs.List[i].Id.Value == ngapType.ProtocolIEIDAMFUENGAPID {
			msg.ProtocolIEs.List[i].Value.AMFUENGAPID = nil
		}
	}

	_, report := decode.DecodePDUSessionResourceModifyIndication(msg)
	if report == nil || !report.Fatal() {
		t.Fatalf("expected fatal report; got %+v", report)
	}

	if len(report.Items) != 1 {
		t.Fatalf("expected 1 report item, got %d", len(report.Items))
	}
}

func TestDecodePDUSessionResourceModifyIndication_DuplicateIELastWins(t *testing.T) {
	msg := validPDUSessionResourceModifyIndication()
	msg.ProtocolIEs.List = append(msg.ProtocolIEs.List, ngapType.PDUSessionResourceModifyIndicationIEs{
		Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDRANUENGAPID},
		Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentReject},
		Value: ngapType.PDUSessionResourceModifyIndicationIEsValue{
			Present:     ngapType.PDUSessionResourceModifyIndicationIEsPresentRANUENGAPID,
			RANUENGAPID: &ngapType.RANUENGAPID{Value: 999},
		},
	})

	out, report := decode.DecodePDUSessionResourceModifyIndication(msg)
	if report != nil {
		t.Fatalf("expected nil report, got %+v", report)
	}

	if out.RANUENGAPID != 999 {
		t.Errorf("expected last-wins RANUENGAPID=999, got %d", out.RANUENGAPID)
	}
}
