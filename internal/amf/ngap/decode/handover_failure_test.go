// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package decode_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/free5gc/ngap/ngapType"
)

func validHandoverFailure() *ngapType.HandoverFailure {
	msg := &ngapType.HandoverFailure{}

	msg.ProtocolIEs.List = []ngapType.HandoverFailureIEs{
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDAMFUENGAPID},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentReject},
			Value: ngapType.HandoverFailureIEsValue{
				Present:     ngapType.HandoverFailureIEsPresentAMFUENGAPID,
				AMFUENGAPID: &ngapType.AMFUENGAPID{Value: 5},
			},
		},
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDCause},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
			Value: ngapType.HandoverFailureIEsValue{
				Present: ngapType.HandoverFailureIEsPresentCause,
				Cause: &ngapType.Cause{
					Present:      ngapType.CausePresentRadioNetwork,
					RadioNetwork: &ngapType.CauseRadioNetwork{Value: ngapType.CauseRadioNetworkPresentHoFailureInTarget5GCNgranNodeOrTargetSystem},
				},
			},
		},
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDCriticalityDiagnostics},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
			Value: ngapType.HandoverFailureIEsValue{
				Present:                ngapType.HandoverFailureIEsPresentCriticalityDiagnostics,
				CriticalityDiagnostics: &ngapType.CriticalityDiagnostics{},
			},
		},
	}

	return msg
}

func TestDecodeHandoverFailure_Happy(t *testing.T) {
	out, report := decode.DecodeHandoverFailure(validHandoverFailure())
	if report != nil {
		t.Fatalf("expected nil report, got %+v", report)
	}

	if out.AMFUENGAPID != 5 {
		t.Errorf("AMFUENGAPID = %d, want 5", out.AMFUENGAPID)
	}

	if out.Cause == nil || out.Cause.Present != ngapType.CausePresentRadioNetwork {
		t.Errorf("Cause = %+v, want RadioNetwork", out.Cause)
	}

	if out.CriticalityDiagnostics == nil {
		t.Error("expected non-nil CriticalityDiagnostics")
	}
}

func TestDecodeHandoverFailure_NilBody(t *testing.T) {
	_, report := decode.DecodeHandoverFailure(nil)
	if report == nil || !report.Fatal() {
		t.Fatalf("expected fatal report, got %+v", report)
	}

	if !report.ProcedureRejected {
		t.Error("expected ProcedureRejected to be set")
	}
}

func TestDecodeHandoverFailure_EmptyIEs(t *testing.T) {
	_, report := decode.DecodeHandoverFailure(&ngapType.HandoverFailure{})
	if report == nil {
		t.Fatal("expected non-nil report")
	}

	// All IEs in HandoverFailure are ignore criticality, so missing
	// mandatories yield a non-fatal report.
	if report.Fatal() {
		t.Fatalf("expected non-fatal report, got %+v", report)
	}

	wantIEs := map[int64]bool{
		ngapType.ProtocolIEIDAMFUENGAPID: true,
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

func TestDecodeHandoverFailure_OptionalAbsent(t *testing.T) {
	msg := validHandoverFailure()

	filtered := msg.ProtocolIEs.List[:0]
	for _, ie := range msg.ProtocolIEs.List {
		if ie.Id.Value != ngapType.ProtocolIEIDCriticalityDiagnostics {
			filtered = append(filtered, ie)
		}
	}

	msg.ProtocolIEs.List = filtered

	out, report := decode.DecodeHandoverFailure(msg)
	if report != nil {
		t.Fatalf("expected nil report, got %+v", report)
	}

	if out.CriticalityDiagnostics != nil {
		t.Error("expected nil CriticalityDiagnostics when absent")
	}
}

func TestDecodeHandoverFailure_NilCauseValueNonFatal(t *testing.T) {
	msg := validHandoverFailure()
	for i := range msg.ProtocolIEs.List {
		if msg.ProtocolIEs.List[i].Id.Value == ngapType.ProtocolIEIDCause {
			msg.ProtocolIEs.List[i].Value.Cause = nil
		}
	}

	out, report := decode.DecodeHandoverFailure(msg)
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

func TestDecodeHandoverFailure_NilAMFUENGAPIDValueNonFatal(t *testing.T) {
	msg := validHandoverFailure()
	for i := range msg.ProtocolIEs.List {
		if msg.ProtocolIEs.List[i].Id.Value == ngapType.ProtocolIEIDAMFUENGAPID {
			msg.ProtocolIEs.List[i].Value.AMFUENGAPID = nil
		}
	}

	out, report := decode.DecodeHandoverFailure(msg)
	if report == nil {
		t.Fatal("expected non-nil report")
	}

	// AMF-UE-NGAP-ID criticality is ignore; a malformed value yields a
	// non-fatal report with AMFUENGAPID left at zero.
	if report.Fatal() {
		t.Fatalf("expected non-fatal report, got %+v", report)
	}

	if len(report.Items) != 1 {
		t.Fatalf("expected 1 item (no double-report), got %d", len(report.Items))
	}

	if out.AMFUENGAPID != 0 {
		t.Errorf("AMFUENGAPID = %d, want 0", out.AMFUENGAPID)
	}
}

func TestDecodeHandoverFailure_DuplicateIELastWins(t *testing.T) {
	msg := validHandoverFailure()

	msg.ProtocolIEs.List = append(msg.ProtocolIEs.List, ngapType.HandoverFailureIEs{
		Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDAMFUENGAPID},
		Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentReject},
		Value: ngapType.HandoverFailureIEsValue{
			Present:     ngapType.HandoverFailureIEsPresentAMFUENGAPID,
			AMFUENGAPID: &ngapType.AMFUENGAPID{Value: 42},
		},
	})

	out, report := decode.DecodeHandoverFailure(msg)
	if report != nil {
		t.Fatalf("expected nil report, got %+v", report)
	}

	if out.AMFUENGAPID != 42 {
		t.Errorf("AMFUENGAPID = %d, want 42 (last-wins)", out.AMFUENGAPID)
	}
}
