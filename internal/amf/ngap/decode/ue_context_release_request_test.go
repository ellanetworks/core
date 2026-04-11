// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package decode_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/free5gc/ngap/ngapType"
)

func validUEContextReleaseRequest() *ngapType.UEContextReleaseRequest {
	msg := &ngapType.UEContextReleaseRequest{}

	msg.ProtocolIEs.List = []ngapType.UEContextReleaseRequestIEs{
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDAMFUENGAPID},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentReject},
			Value: ngapType.UEContextReleaseRequestIEsValue{
				Present:     ngapType.UEContextReleaseRequestIEsPresentAMFUENGAPID,
				AMFUENGAPID: &ngapType.AMFUENGAPID{Value: 11},
			},
		},
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDRANUENGAPID},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentReject},
			Value: ngapType.UEContextReleaseRequestIEsValue{
				Present:     ngapType.UEContextReleaseRequestIEsPresentRANUENGAPID,
				RANUENGAPID: &ngapType.RANUENGAPID{Value: 22},
			},
		},
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDPDUSessionResourceListCxtRelReq},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentReject},
			Value: ngapType.UEContextReleaseRequestIEsValue{
				Present: ngapType.UEContextReleaseRequestIEsPresentPDUSessionResourceListCxtRelReq,
				PDUSessionResourceListCxtRelReq: &ngapType.PDUSessionResourceListCxtRelReq{
					List: []ngapType.PDUSessionResourceItemCxtRelReq{
						{PDUSessionID: ngapType.PDUSessionID{Value: 5}},
					},
				},
			},
		},
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDCause},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
			Value: ngapType.UEContextReleaseRequestIEsValue{
				Present: ngapType.UEContextReleaseRequestIEsPresentCause,
				Cause: &ngapType.Cause{
					Present:      ngapType.CausePresentRadioNetwork,
					RadioNetwork: &ngapType.CauseRadioNetwork{Value: ngapType.CauseRadioNetworkPresentUserInactivity},
				},
			},
		},
	}

	return msg
}

func TestDecodeUEContextReleaseRequest_Happy(t *testing.T) {
	out, report := decode.DecodeUEContextReleaseRequest(validUEContextReleaseRequest())
	if report != nil {
		t.Fatalf("expected nil report, got %+v", report)
	}

	if out.AMFUENGAPID != 11 {
		t.Errorf("AMFUENGAPID = %d, want 11", out.AMFUENGAPID)
	}

	if out.RANUENGAPID != 22 {
		t.Errorf("RANUENGAPID = %d, want 22", out.RANUENGAPID)
	}

	if len(out.PDUSessionResourceList) != 1 || out.PDUSessionResourceList[0].PDUSessionID.Value != 5 {
		t.Errorf("PDUSessionResourceList = %+v, want one item with id 5", out.PDUSessionResourceList)
	}

	if out.Cause == nil || out.Cause.Present != ngapType.CausePresentRadioNetwork {
		t.Fatalf("expected RadioNetwork cause, got %+v", out.Cause)
	}

	if out.Cause.RadioNetwork.Value != ngapType.CauseRadioNetworkPresentUserInactivity {
		t.Errorf("cause radio = %d, want UserInactivity", out.Cause.RadioNetwork.Value)
	}
}

func TestDecodeUEContextReleaseRequest_NilBody(t *testing.T) {
	out, report := decode.DecodeUEContextReleaseRequest(nil)
	if report == nil || !report.Fatal() {
		t.Fatalf("expected fatal report, got %+v", report)
	}

	if !report.ProcedureRejected {
		t.Error("expected ProcedureRejected to be set")
	}

	if out.AMFUENGAPID != 0 {
		t.Errorf("expected zero value, got %d", out.AMFUENGAPID)
	}
}

func TestDecodeUEContextReleaseRequest_EmptyIEs(t *testing.T) {
	_, report := decode.DecodeUEContextReleaseRequest(&ngapType.UEContextReleaseRequest{})
	if report == nil || !report.Fatal() {
		t.Fatalf("expected fatal report, got %+v", report)
	}

	wantIEs := map[int64]bool{
		ngapType.ProtocolIEIDAMFUENGAPID: true,
		ngapType.ProtocolIEIDRANUENGAPID: true,
		ngapType.ProtocolIEIDCause:       true,
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

func TestDecodeUEContextReleaseRequest_OptionalAbsent(t *testing.T) {
	msg := validUEContextReleaseRequest()

	// drop the PDUSessionResourceListCxtRelReq IE
	out := msg.ProtocolIEs.List[:0]

	for _, ie := range msg.ProtocolIEs.List {
		if ie.Id.Value == ngapType.ProtocolIEIDPDUSessionResourceListCxtRelReq {
			continue
		}

		out = append(out, ie)
	}

	msg.ProtocolIEs.List = out

	decoded, report := decode.DecodeUEContextReleaseRequest(msg)
	if report != nil {
		t.Fatalf("expected nil report, got %+v", report)
	}

	if decoded.PDUSessionResourceList != nil {
		t.Errorf("expected nil PDU list when IE absent, got %+v", decoded.PDUSessionResourceList)
	}
}

func TestDecodeUEContextReleaseRequest_PresentEmptyListIsNonNil(t *testing.T) {
	msg := validUEContextReleaseRequest()

	for i := range msg.ProtocolIEs.List {
		if msg.ProtocolIEs.List[i].Id.Value == ngapType.ProtocolIEIDPDUSessionResourceListCxtRelReq {
			msg.ProtocolIEs.List[i].Value.PDUSessionResourceListCxtRelReq = &ngapType.PDUSessionResourceListCxtRelReq{}
		}
	}

	decoded, report := decode.DecodeUEContextReleaseRequest(msg)
	if report != nil {
		t.Fatalf("expected nil report, got %+v", report)
	}

	if decoded.PDUSessionResourceList == nil {
		t.Error("expected non-nil empty slice when IE present with no items, got nil")
	}

	if len(decoded.PDUSessionResourceList) != 0 {
		t.Errorf("expected zero items, got %d", len(decoded.PDUSessionResourceList))
	}
}

func TestDecodeUEContextReleaseRequest_NilCauseValueNonFatal(t *testing.T) {
	msg := validUEContextReleaseRequest()
	for i := range msg.ProtocolIEs.List {
		if msg.ProtocolIEs.List[i].Id.Value == ngapType.ProtocolIEIDCause {
			msg.ProtocolIEs.List[i].Value.Cause = nil
		}
	}

	out, report := decode.DecodeUEContextReleaseRequest(msg)
	if report == nil {
		t.Fatal("expected non-nil report")
	}

	// Cause is mandatory-ignore: malformed must not be fatal.
	if report.Fatal() {
		t.Errorf("expected non-fatal report, got %+v", report)
	}

	if out.Cause != nil {
		t.Errorf("expected nil Cause on malformed input, got %+v", out.Cause)
	}
}

func TestDecodeUEContextReleaseRequest_NilAMFUENGAPIDValueIsFatal(t *testing.T) {
	msg := validUEContextReleaseRequest()
	for i := range msg.ProtocolIEs.List {
		if msg.ProtocolIEs.List[i].Id.Value == ngapType.ProtocolIEIDAMFUENGAPID {
			msg.ProtocolIEs.List[i].Value.AMFUENGAPID = nil
		}
	}

	_, report := decode.DecodeUEContextReleaseRequest(msg)
	if report == nil || !report.Fatal() {
		t.Fatalf("expected fatal report, got %+v", report)
	}

	if len(report.Items) != 1 {
		t.Fatalf("expected 1 item (no double-report), got %d (%+v)", len(report.Items), report.Items)
	}
}

func TestDecodeUEContextReleaseRequest_DuplicateIELastWins(t *testing.T) {
	msg := validUEContextReleaseRequest()

	msg.ProtocolIEs.List = append(msg.ProtocolIEs.List, ngapType.UEContextReleaseRequestIEs{
		Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDRANUENGAPID},
		Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentReject},
		Value: ngapType.UEContextReleaseRequestIEsValue{
			Present:     ngapType.UEContextReleaseRequestIEsPresentRANUENGAPID,
			RANUENGAPID: &ngapType.RANUENGAPID{Value: 7777},
		},
	})

	out, report := decode.DecodeUEContextReleaseRequest(msg)
	if report != nil {
		t.Fatalf("expected nil report, got %+v", report)
	}

	if out.RANUENGAPID != 7777 {
		t.Errorf("expected last-wins RANUENGAPID=7777, got %d", out.RANUENGAPID)
	}
}
