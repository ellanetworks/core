// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package decode_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/free5gc/ngap/ngapType"
)

func validHandoverCancel() *ngapType.HandoverCancel {
	msg := &ngapType.HandoverCancel{}

	msg.ProtocolIEs.List = []ngapType.HandoverCancelIEs{
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDAMFUENGAPID},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentReject},
			Value: ngapType.HandoverCancelIEsValue{
				Present:     ngapType.HandoverCancelIEsPresentAMFUENGAPID,
				AMFUENGAPID: &ngapType.AMFUENGAPID{Value: 5},
			},
		},
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDRANUENGAPID},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentReject},
			Value: ngapType.HandoverCancelIEsValue{
				Present:     ngapType.HandoverCancelIEsPresentRANUENGAPID,
				RANUENGAPID: &ngapType.RANUENGAPID{Value: 9},
			},
		},
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDCause},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
			Value: ngapType.HandoverCancelIEsValue{
				Present: ngapType.HandoverCancelIEsPresentCause,
				Cause: &ngapType.Cause{
					Present:      ngapType.CausePresentRadioNetwork,
					RadioNetwork: &ngapType.CauseRadioNetwork{Value: ngapType.CauseRadioNetworkPresentHandoverCancelled},
				},
			},
		},
	}

	return msg
}

func TestDecodeHandoverCancel_Happy(t *testing.T) {
	out, report := decode.DecodeHandoverCancel(validHandoverCancel())
	if report != nil {
		t.Fatalf("expected nil report, got %+v", report)
	}

	if out.AMFUENGAPID != 5 {
		t.Errorf("AMFUENGAPID = %d, want 5", out.AMFUENGAPID)
	}

	if out.RANUENGAPID != 9 {
		t.Errorf("RANUENGAPID = %d, want 9", out.RANUENGAPID)
	}

	if out.Cause == nil || out.Cause.Present != ngapType.CausePresentRadioNetwork {
		t.Errorf("Cause = %+v, want RadioNetwork", out.Cause)
	}
}

func TestDecodeHandoverCancel_NilBody(t *testing.T) {
	_, report := decode.DecodeHandoverCancel(nil)
	if report == nil || !report.Fatal() {
		t.Fatalf("expected fatal report, got %+v", report)
	}

	if !report.ProcedureRejected {
		t.Error("expected ProcedureRejected to be set")
	}
}

func TestDecodeHandoverCancel_EmptyIEs(t *testing.T) {
	_, report := decode.DecodeHandoverCancel(&ngapType.HandoverCancel{})
	if report == nil || !report.Fatal() {
		t.Fatalf("expected fatal report (missing mandatory-reject IEs), got %+v", report)
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

func TestDecodeHandoverCancel_NilCauseValueNonFatal(t *testing.T) {
	msg := validHandoverCancel()
	for i := range msg.ProtocolIEs.List {
		if msg.ProtocolIEs.List[i].Id.Value == ngapType.ProtocolIEIDCause {
			msg.ProtocolIEs.List[i].Value.Cause = nil
		}
	}

	out, report := decode.DecodeHandoverCancel(msg)
	if report == nil {
		t.Fatal("expected non-nil report")
	}

	if report.Fatal() {
		t.Errorf("expected non-fatal report (Cause criticality ignore), got %+v", report)
	}

	if out.Cause != nil {
		t.Error("expected nil Cause when malformed")
	}
}

func TestDecodeHandoverCancel_NilAMFUENGAPIDValueIsFatal(t *testing.T) {
	msg := validHandoverCancel()
	for i := range msg.ProtocolIEs.List {
		if msg.ProtocolIEs.List[i].Id.Value == ngapType.ProtocolIEIDAMFUENGAPID {
			msg.ProtocolIEs.List[i].Value.AMFUENGAPID = nil
		}
	}

	_, report := decode.DecodeHandoverCancel(msg)
	if report == nil || !report.Fatal() {
		t.Fatalf("expected fatal report (AMFUENGAPID criticality reject), got %+v", report)
	}

	if len(report.Items) != 1 {
		t.Fatalf("expected 1 item (no double-report), got %d", len(report.Items))
	}
}

func TestDecodeHandoverCancel_DuplicateIELastWins(t *testing.T) {
	msg := validHandoverCancel()

	msg.ProtocolIEs.List = append(msg.ProtocolIEs.List, ngapType.HandoverCancelIEs{
		Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDAMFUENGAPID},
		Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentReject},
		Value: ngapType.HandoverCancelIEsValue{
			Present:     ngapType.HandoverCancelIEsPresentAMFUENGAPID,
			AMFUENGAPID: &ngapType.AMFUENGAPID{Value: 42},
		},
	})

	out, report := decode.DecodeHandoverCancel(msg)
	if report != nil {
		t.Fatalf("expected nil report, got %+v", report)
	}

	if out.AMFUENGAPID != 42 {
		t.Errorf("AMFUENGAPID = %d, want 42 (last-wins)", out.AMFUENGAPID)
	}
}
