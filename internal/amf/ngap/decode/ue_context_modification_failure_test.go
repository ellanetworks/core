// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package decode_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/free5gc/ngap/ngapType"
)

func validUEContextModificationFailure() *ngapType.UEContextModificationFailure {
	msg := &ngapType.UEContextModificationFailure{}

	msg.ProtocolIEs.List = []ngapType.UEContextModificationFailureIEs{
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDAMFUENGAPID},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
			Value: ngapType.UEContextModificationFailureIEsValue{
				Present:     ngapType.UEContextModificationFailureIEsPresentAMFUENGAPID,
				AMFUENGAPID: &ngapType.AMFUENGAPID{Value: 5},
			},
		},
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDRANUENGAPID},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
			Value: ngapType.UEContextModificationFailureIEsValue{
				Present:     ngapType.UEContextModificationFailureIEsPresentRANUENGAPID,
				RANUENGAPID: &ngapType.RANUENGAPID{Value: 9},
			},
		},
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDCause},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
			Value: ngapType.UEContextModificationFailureIEsValue{
				Present: ngapType.UEContextModificationFailureIEsPresentCause,
				Cause: &ngapType.Cause{
					Present:      ngapType.CausePresentRadioNetwork,
					RadioNetwork: &ngapType.CauseRadioNetwork{Value: ngapType.CauseRadioNetworkPresentUnspecified},
				},
			},
		},
	}

	return msg
}

func TestDecodeUEContextModificationFailure_Happy(t *testing.T) {
	out, report := decode.DecodeUEContextModificationFailure(validUEContextModificationFailure())
	if report != nil {
		t.Fatalf("expected nil report, got %+v", report)
	}

	if out.AMFUENGAPID == nil || *out.AMFUENGAPID != 5 {
		t.Errorf("AMFUENGAPID = %v, want 5", out.AMFUENGAPID)
	}

	if out.RANUENGAPID == nil || *out.RANUENGAPID != 9 {
		t.Errorf("RANUENGAPID = %v, want 9", out.RANUENGAPID)
	}

	if out.Cause == nil || out.Cause.Present != ngapType.CausePresentRadioNetwork {
		t.Errorf("Cause = %+v, want RadioNetwork", out.Cause)
	}
}

func TestDecodeUEContextModificationFailure_NilBody(t *testing.T) {
	_, report := decode.DecodeUEContextModificationFailure(nil)
	if report == nil || !report.Fatal() {
		t.Fatalf("expected fatal report, got %+v", report)
	}

	if !report.ProcedureRejected {
		t.Error("expected ProcedureRejected to be set")
	}
}

func TestDecodeUEContextModificationFailure_EmptyIEsNonFatal(t *testing.T) {
	out, report := decode.DecodeUEContextModificationFailure(&ngapType.UEContextModificationFailure{})
	if report == nil {
		t.Fatal("expected non-nil report (missing-mandatory diagnostics)")
	}

	if report.Fatal() {
		t.Errorf("expected non-fatal report (all IEs ignore), got %+v", report)
	}

	if out.AMFUENGAPID != nil || out.RANUENGAPID != nil || out.Cause != nil {
		t.Error("expected zero-value output for empty IEs")
	}
}

func TestDecodeUEContextModificationFailure_ZeroIDIsPresent(t *testing.T) {
	msg := validUEContextModificationFailure()
	for i := range msg.ProtocolIEs.List {
		if msg.ProtocolIEs.List[i].Id.Value == ngapType.ProtocolIEIDAMFUENGAPID {
			msg.ProtocolIEs.List[i].Value.AMFUENGAPID = &ngapType.AMFUENGAPID{Value: 0}
		}
	}

	out, report := decode.DecodeUEContextModificationFailure(msg)
	if report != nil {
		t.Fatalf("expected nil report, got %+v", report)
	}

	if out.AMFUENGAPID == nil {
		t.Fatal("expected non-nil AMFUENGAPID for zero value")
	}

	if *out.AMFUENGAPID != 0 {
		t.Errorf("expected 0, got %d", *out.AMFUENGAPID)
	}
}

func TestDecodeUEContextModificationFailure_NilAMFUENGAPIDValueNonFatal(t *testing.T) {
	msg := validUEContextModificationFailure()
	for i := range msg.ProtocolIEs.List {
		if msg.ProtocolIEs.List[i].Id.Value == ngapType.ProtocolIEIDAMFUENGAPID {
			msg.ProtocolIEs.List[i].Value.AMFUENGAPID = nil
		}
	}

	out, report := decode.DecodeUEContextModificationFailure(msg)
	if report == nil {
		t.Fatal("expected non-nil report")
	}

	if report.Fatal() {
		t.Errorf("expected non-fatal report (criticality ignore), got %+v", report)
	}

	if out.AMFUENGAPID != nil {
		t.Error("expected nil AMFUENGAPID when malformed")
	}
}

func TestDecodeUEContextModificationFailure_DuplicateIELastWins(t *testing.T) {
	msg := validUEContextModificationFailure()

	msg.ProtocolIEs.List = append(msg.ProtocolIEs.List, ngapType.UEContextModificationFailureIEs{
		Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDRANUENGAPID},
		Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
		Value: ngapType.UEContextModificationFailureIEsValue{
			Present:     ngapType.UEContextModificationFailureIEsPresentRANUENGAPID,
			RANUENGAPID: &ngapType.RANUENGAPID{Value: 42},
		},
	})

	out, report := decode.DecodeUEContextModificationFailure(msg)
	if report != nil {
		t.Fatalf("expected nil report, got %+v", report)
	}

	if out.RANUENGAPID == nil || *out.RANUENGAPID != 42 {
		t.Errorf("RANUENGAPID = %v, want 42 (last-wins)", out.RANUENGAPID)
	}
}
