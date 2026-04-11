// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package decode_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/free5gc/ngap/ngapType"
)

func validUEContextModificationResponse() *ngapType.UEContextModificationResponse {
	msg := &ngapType.UEContextModificationResponse{}

	msg.ProtocolIEs.List = []ngapType.UEContextModificationResponseIEs{
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDAMFUENGAPID},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
			Value: ngapType.UEContextModificationResponseIEsValue{
				Present:     ngapType.UEContextModificationResponseIEsPresentAMFUENGAPID,
				AMFUENGAPID: &ngapType.AMFUENGAPID{Value: 3},
			},
		},
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDRANUENGAPID},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
			Value: ngapType.UEContextModificationResponseIEsValue{
				Present:     ngapType.UEContextModificationResponseIEsPresentRANUENGAPID,
				RANUENGAPID: &ngapType.RANUENGAPID{Value: 4},
			},
		},
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDRRCState},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
			Value: ngapType.UEContextModificationResponseIEsValue{
				Present:  ngapType.UEContextModificationResponseIEsPresentRRCState,
				RRCState: &ngapType.RRCState{Value: ngapType.RRCStatePresentConnected},
			},
		},
	}

	return msg
}

func TestDecodeUEContextModificationResponse_Happy(t *testing.T) {
	out, report := decode.DecodeUEContextModificationResponse(validUEContextModificationResponse())
	if report != nil {
		t.Fatalf("expected nil report, got %+v", report)
	}

	if out.AMFUENGAPID == nil || *out.AMFUENGAPID != 3 {
		t.Errorf("AMFUENGAPID = %v, want 3", out.AMFUENGAPID)
	}

	if out.RANUENGAPID == nil || *out.RANUENGAPID != 4 {
		t.Errorf("RANUENGAPID = %v, want 4", out.RANUENGAPID)
	}

	if out.RRCState == nil || out.RRCState.Value != ngapType.RRCStatePresentConnected {
		t.Errorf("RRCState = %v, want Connected", out.RRCState)
	}
}

func TestDecodeUEContextModificationResponse_NilBody(t *testing.T) {
	_, report := decode.DecodeUEContextModificationResponse(nil)
	if report == nil || !report.Fatal() {
		t.Fatalf("expected fatal report, got %+v", report)
	}

	if !report.ProcedureRejected {
		t.Error("expected ProcedureRejected to be set")
	}
}

func TestDecodeUEContextModificationResponse_EmptyIEsNonFatal(t *testing.T) {
	_, report := decode.DecodeUEContextModificationResponse(&ngapType.UEContextModificationResponse{})
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

func TestDecodeUEContextModificationResponse_NilAMFUENGAPIDValueNonFatal(t *testing.T) {
	msg := validUEContextModificationResponse()
	for i := range msg.ProtocolIEs.List {
		if msg.ProtocolIEs.List[i].Id.Value == ngapType.ProtocolIEIDAMFUENGAPID {
			msg.ProtocolIEs.List[i].Value.AMFUENGAPID = nil
		}
	}

	out, report := decode.DecodeUEContextModificationResponse(msg)
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

func TestDecodeUEContextModificationResponse_DuplicateIELastWins(t *testing.T) {
	msg := validUEContextModificationResponse()

	msg.ProtocolIEs.List = append(msg.ProtocolIEs.List, ngapType.UEContextModificationResponseIEs{
		Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDAMFUENGAPID},
		Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
		Value: ngapType.UEContextModificationResponseIEsValue{
			Present:     ngapType.UEContextModificationResponseIEsPresentAMFUENGAPID,
			AMFUENGAPID: &ngapType.AMFUENGAPID{Value: 777},
		},
	})

	out, report := decode.DecodeUEContextModificationResponse(msg)
	if report != nil {
		t.Fatalf("expected nil report, got %+v", report)
	}

	if out.AMFUENGAPID == nil || *out.AMFUENGAPID != 777 {
		t.Errorf("AMFUENGAPID = %v, want 777 (last-wins)", out.AMFUENGAPID)
	}
}
