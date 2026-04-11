// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package decode_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/free5gc/ngap/ngapType"
)

func validUEContextReleaseComplete() *ngapType.UEContextReleaseComplete {
	msg := &ngapType.UEContextReleaseComplete{}

	msg.ProtocolIEs.List = []ngapType.UEContextReleaseCompleteIEs{
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDAMFUENGAPID},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
			Value: ngapType.UEContextReleaseCompleteIEsValue{
				Present:     ngapType.UEContextReleaseCompleteIEsPresentAMFUENGAPID,
				AMFUENGAPID: &ngapType.AMFUENGAPID{Value: 11},
			},
		},
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDRANUENGAPID},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
			Value: ngapType.UEContextReleaseCompleteIEsValue{
				Present:     ngapType.UEContextReleaseCompleteIEsPresentRANUENGAPID,
				RANUENGAPID: &ngapType.RANUENGAPID{Value: 22},
			},
		},
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDPDUSessionResourceListCxtRelCpl},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentReject},
			Value: ngapType.UEContextReleaseCompleteIEsValue{
				Present: ngapType.UEContextReleaseCompleteIEsPresentPDUSessionResourceListCxtRelCpl,
				PDUSessionResourceListCxtRelCpl: &ngapType.PDUSessionResourceListCxtRelCpl{
					List: []ngapType.PDUSessionResourceItemCxtRelCpl{
						{PDUSessionID: ngapType.PDUSessionID{Value: 1}},
					},
				},
			},
		},
	}

	return msg
}

func TestDecodeUEContextReleaseComplete_Happy(t *testing.T) {
	out, report := decode.DecodeUEContextReleaseComplete(validUEContextReleaseComplete())
	if report != nil {
		t.Fatalf("expected nil report, got %+v", report)
	}

	if out.AMFUENGAPID == nil || *out.AMFUENGAPID != 11 {
		t.Errorf("AMFUENGAPID = %v, want 11", out.AMFUENGAPID)
	}

	if out.RANUENGAPID == nil || *out.RANUENGAPID != 22 {
		t.Errorf("RANUENGAPID = %v, want 22", out.RANUENGAPID)
	}

	if out.PDUSessionResourceList == nil || len(out.PDUSessionResourceList.List) != 1 {
		t.Errorf("expected 1 PDU session resource item, got %+v", out.PDUSessionResourceList)
	}
}

func TestDecodeUEContextReleaseComplete_NilBody(t *testing.T) {
	_, report := decode.DecodeUEContextReleaseComplete(nil)
	if report == nil || !report.Fatal() {
		t.Fatalf("expected fatal report, got %+v", report)
	}

	if !report.ProcedureRejected {
		t.Error("expected ProcedureRejected to be set")
	}
}

func TestDecodeUEContextReleaseComplete_EmptyIEsNonFatal(t *testing.T) {
	_, report := decode.DecodeUEContextReleaseComplete(&ngapType.UEContextReleaseComplete{})
	if report == nil {
		t.Fatal("expected non-nil report")
	}

	if report.Fatal() {
		t.Errorf("expected non-fatal report, got %+v", report)
	}

	wantIEs := map[int64]bool{
		ngapType.ProtocolIEIDAMFUENGAPID: true,
		ngapType.ProtocolIEIDRANUENGAPID: true,
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

func TestDecodeUEContextReleaseComplete_NilAMFUENGAPIDValueNonFatal(t *testing.T) {
	msg := validUEContextReleaseComplete()
	for i := range msg.ProtocolIEs.List {
		if msg.ProtocolIEs.List[i].Id.Value == ngapType.ProtocolIEIDAMFUENGAPID {
			msg.ProtocolIEs.List[i].Value.AMFUENGAPID = nil
		}
	}

	out, report := decode.DecodeUEContextReleaseComplete(msg)
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

func TestDecodeUEContextReleaseComplete_DuplicateIELastWins(t *testing.T) {
	msg := validUEContextReleaseComplete()

	msg.ProtocolIEs.List = append(msg.ProtocolIEs.List, ngapType.UEContextReleaseCompleteIEs{
		Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDAMFUENGAPID},
		Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
		Value: ngapType.UEContextReleaseCompleteIEsValue{
			Present:     ngapType.UEContextReleaseCompleteIEsPresentAMFUENGAPID,
			AMFUENGAPID: &ngapType.AMFUENGAPID{Value: 999},
		},
	})

	out, report := decode.DecodeUEContextReleaseComplete(msg)
	if report != nil {
		t.Fatalf("expected nil report, got %+v", report)
	}

	if out.AMFUENGAPID == nil || *out.AMFUENGAPID != 999 {
		t.Errorf("AMFUENGAPID = %v, want 999 (last-wins)", out.AMFUENGAPID)
	}
}
