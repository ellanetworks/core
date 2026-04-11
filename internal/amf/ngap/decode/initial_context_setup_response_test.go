// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package decode_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/free5gc/ngap/ngapType"
)

func validInitialContextSetupResponse() *ngapType.InitialContextSetupResponse {
	msg := &ngapType.InitialContextSetupResponse{}

	msg.ProtocolIEs.List = []ngapType.InitialContextSetupResponseIEs{
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDAMFUENGAPID},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
			Value: ngapType.InitialContextSetupResponseIEsValue{
				Present:     ngapType.InitialContextSetupResponseIEsPresentAMFUENGAPID,
				AMFUENGAPID: &ngapType.AMFUENGAPID{Value: 12},
			},
		},
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDRANUENGAPID},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
			Value: ngapType.InitialContextSetupResponseIEsValue{
				Present:     ngapType.InitialContextSetupResponseIEsPresentRANUENGAPID,
				RANUENGAPID: &ngapType.RANUENGAPID{Value: 34},
			},
		},
	}

	return msg
}

func TestDecodeInitialContextSetupResponse_Happy(t *testing.T) {
	msg := validInitialContextSetupResponse()

	out, report := decode.DecodeInitialContextSetupResponse(msg)
	if report != nil {
		t.Fatalf("expected nil report, got %+v", report)
	}

	if out.AMFUENGAPID != 12 {
		t.Errorf("AMFUENGAPID = %d, want 12", out.AMFUENGAPID)
	}

	if out.RANUENGAPID != 34 {
		t.Errorf("RANUENGAPID = %d, want 34", out.RANUENGAPID)
	}

	if out.SetupItems != nil {
		t.Errorf("SetupItems should be nil when IE absent, got %+v", out.SetupItems)
	}

	if out.FailedToSetupItems != nil {
		t.Errorf("FailedToSetupItems should be nil when IE absent, got %+v", out.FailedToSetupItems)
	}
}

func TestDecodeInitialContextSetupResponse_NilBody(t *testing.T) {
	out, report := decode.DecodeInitialContextSetupResponse(nil)
	if report == nil {
		t.Fatal("expected non-nil report")
	}

	if !report.Fatal() {
		t.Error("expected fatal report for nil body")
	}

	if !report.ProcedureRejected {
		t.Error("expected ProcedureRejected to be set")
	}

	if out.AMFUENGAPID != 0 {
		t.Errorf("expected zero value, got %d", out.AMFUENGAPID)
	}
}

func TestDecodeInitialContextSetupResponse_EmptyIEsNonFatal(t *testing.T) {
	// All IEs in InitialContextSetupResponse are mandatory-ignore. An
	// empty container should produce a non-fatal report (handler still
	// runs) listing both missing mandatory IEs.
	_, report := decode.DecodeInitialContextSetupResponse(&ngapType.InitialContextSetupResponse{})
	if report == nil {
		t.Fatal("expected non-nil report")
	}

	if report.Fatal() {
		t.Errorf("expected non-fatal report; got %+v", report)
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

func TestDecodeInitialContextSetupResponse_NilAMFUENGAPIDValueNonFatal(t *testing.T) {
	msg := validInitialContextSetupResponse()
	for i := range msg.ProtocolIEs.List {
		if msg.ProtocolIEs.List[i].Id.Value == ngapType.ProtocolIEIDAMFUENGAPID {
			msg.ProtocolIEs.List[i].Value.AMFUENGAPID = nil
		}
	}

	out, report := decode.DecodeInitialContextSetupResponse(msg)
	if report == nil {
		t.Fatal("expected non-nil report")
	}

	if report.Fatal() {
		t.Errorf("expected non-fatal report for nil AMFUENGAPID, got %+v", report)
	}

	if len(report.Items) != 1 {
		t.Fatalf("expected 1 report item, got %d (%+v)", len(report.Items), report.Items)
	}

	if out.AMFUENGAPID != 0 {
		t.Errorf("expected zero AMFUENGAPID on malformed input, got %d", out.AMFUENGAPID)
	}
}

func TestDecodeInitialContextSetupResponse_OptionalSetupLists(t *testing.T) {
	msg := validInitialContextSetupResponse()

	msg.ProtocolIEs.List = append(msg.ProtocolIEs.List,
		ngapType.InitialContextSetupResponseIEs{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDPDUSessionResourceSetupListCxtRes},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
			Value: ngapType.InitialContextSetupResponseIEsValue{
				Present: ngapType.InitialContextSetupResponseIEsPresentPDUSessionResourceSetupListCxtRes,
				PDUSessionResourceSetupListCxtRes: &ngapType.PDUSessionResourceSetupListCxtRes{
					List: []ngapType.PDUSessionResourceSetupItemCxtRes{
						{
							PDUSessionID:                            ngapType.PDUSessionID{Value: 1},
							PDUSessionResourceSetupResponseTransfer: []byte{0xDE, 0xAD},
						},
					},
				},
			},
		},
		ngapType.InitialContextSetupResponseIEs{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDPDUSessionResourceFailedToSetupListCxtRes},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
			Value: ngapType.InitialContextSetupResponseIEsValue{
				Present: ngapType.InitialContextSetupResponseIEsPresentPDUSessionResourceFailedToSetupListCxtRes,
				PDUSessionResourceFailedToSetupListCxtRes: &ngapType.PDUSessionResourceFailedToSetupListCxtRes{
					List: []ngapType.PDUSessionResourceFailedToSetupItemCxtRes{
						{
							PDUSessionID: ngapType.PDUSessionID{Value: 2},
							PDUSessionResourceSetupUnsuccessfulTransfer: []byte{0xBE, 0xEF},
						},
					},
				},
			},
		},
	)

	out, report := decode.DecodeInitialContextSetupResponse(msg)
	if report != nil {
		t.Fatalf("expected nil report, got %+v", report)
	}

	if len(out.SetupItems) != 1 {
		t.Fatalf("SetupItems len = %d, want 1", len(out.SetupItems))
	}

	if out.SetupItems[0].PDUSessionID.Value != 1 {
		t.Errorf("SetupItem PDUSessionID = %d, want 1", out.SetupItems[0].PDUSessionID.Value)
	}

	if len(out.FailedToSetupItems) != 1 {
		t.Fatalf("FailedToSetupItems len = %d, want 1", len(out.FailedToSetupItems))
	}

	if out.FailedToSetupItems[0].PDUSessionID.Value != 2 {
		t.Errorf("FailedToSetupItem PDUSessionID = %d, want 2", out.FailedToSetupItems[0].PDUSessionID.Value)
	}
}

func TestDecodeInitialContextSetupResponse_DuplicateIELastWins(t *testing.T) {
	msg := validInitialContextSetupResponse()

	msg.ProtocolIEs.List = append(msg.ProtocolIEs.List, ngapType.InitialContextSetupResponseIEs{
		Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDRANUENGAPID},
		Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
		Value: ngapType.InitialContextSetupResponseIEsValue{
			Present:     ngapType.InitialContextSetupResponseIEsPresentRANUENGAPID,
			RANUENGAPID: &ngapType.RANUENGAPID{Value: 7777},
		},
	})

	out, report := decode.DecodeInitialContextSetupResponse(msg)
	if report != nil {
		t.Fatalf("expected nil report, got %+v", report)
	}

	if out.RANUENGAPID != 7777 {
		t.Errorf("expected last-wins RANUENGAPID=7777, got %d", out.RANUENGAPID)
	}
}
