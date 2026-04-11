// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package decode_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/free5gc/ngap/ngapType"
)

func validErrorIndication() *ngapType.ErrorIndication {
	msg := &ngapType.ErrorIndication{}

	msg.ProtocolIEs.List = []ngapType.ErrorIndicationIEs{
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDAMFUENGAPID},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
			Value: ngapType.ErrorIndicationIEsValue{
				Present:     ngapType.ErrorIndicationIEsPresentAMFUENGAPID,
				AMFUENGAPID: &ngapType.AMFUENGAPID{Value: 7},
			},
		},
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDRANUENGAPID},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
			Value: ngapType.ErrorIndicationIEsValue{
				Present:     ngapType.ErrorIndicationIEsPresentRANUENGAPID,
				RANUENGAPID: &ngapType.RANUENGAPID{Value: 11},
			},
		},
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDCause},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
			Value: ngapType.ErrorIndicationIEsValue{
				Present: ngapType.ErrorIndicationIEsPresentCause,
				Cause: &ngapType.Cause{
					Present:      ngapType.CausePresentRadioNetwork,
					RadioNetwork: &ngapType.CauseRadioNetwork{Value: ngapType.CauseRadioNetworkPresentUnspecified},
				},
			},
		},
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDCriticalityDiagnostics},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
			Value: ngapType.ErrorIndicationIEsValue{
				Present:                ngapType.ErrorIndicationIEsPresentCriticalityDiagnostics,
				CriticalityDiagnostics: &ngapType.CriticalityDiagnostics{},
			},
		},
	}

	return msg
}

func TestDecodeErrorIndication_Happy(t *testing.T) {
	out, report := decode.DecodeErrorIndication(validErrorIndication())
	if report != nil {
		t.Fatalf("expected nil report, got %+v", report)
	}

	if out.Cause == nil {
		t.Fatal("expected non-nil Cause")
	}

	if out.Cause.Present != ngapType.CausePresentRadioNetwork {
		t.Errorf("Cause.Present = %d, want RadioNetwork", out.Cause.Present)
	}

	if out.CriticalityDiagnostics == nil {
		t.Error("expected non-nil CriticalityDiagnostics")
	}
}

func TestDecodeErrorIndication_NilBody(t *testing.T) {
	_, report := decode.DecodeErrorIndication(nil)
	if report == nil || !report.Fatal() {
		t.Fatalf("expected fatal report, got %+v", report)
	}

	if !report.ProcedureRejected {
		t.Error("expected ProcedureRejected to be set")
	}
}

func TestDecodeErrorIndication_EmptyIEsNonFatal(t *testing.T) {
	out, report := decode.DecodeErrorIndication(&ngapType.ErrorIndication{})
	if report != nil {
		t.Fatalf("expected nil report (all IEs optional), got %+v", report)
	}

	if out.Cause != nil || out.CriticalityDiagnostics != nil {
		t.Error("expected zero-value output for empty IEs")
	}
}

func TestDecodeErrorIndication_MalformedCauseNonFatal(t *testing.T) {
	msg := validErrorIndication()
	for i := range msg.ProtocolIEs.List {
		if msg.ProtocolIEs.List[i].Id.Value == ngapType.ProtocolIEIDCause {
			msg.ProtocolIEs.List[i].Value.Cause = nil
		}
	}

	out, report := decode.DecodeErrorIndication(msg)
	if report == nil {
		t.Fatal("expected non-nil report")
	}

	if report.Fatal() {
		t.Errorf("expected non-fatal report (Cause optional-ignore), got %+v", report)
	}

	if out.Cause != nil {
		t.Error("expected nil Cause when malformed")
	}
}

func TestDecodeErrorIndication_DuplicateIELastWins(t *testing.T) {
	msg := validErrorIndication()

	msg.ProtocolIEs.List = append(msg.ProtocolIEs.List, ngapType.ErrorIndicationIEs{
		Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDCause},
		Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
		Value: ngapType.ErrorIndicationIEsValue{
			Present: ngapType.ErrorIndicationIEsPresentCause,
			Cause: &ngapType.Cause{
				Present: ngapType.CausePresentMisc,
				Misc:    &ngapType.CauseMisc{Value: ngapType.CauseMiscPresentUnspecified},
			},
		},
	})

	out, report := decode.DecodeErrorIndication(msg)
	if report != nil {
		t.Fatalf("expected nil report, got %+v", report)
	}

	if out.Cause == nil || out.Cause.Present != ngapType.CausePresentMisc {
		t.Errorf("expected last-wins Misc cause, got %+v", out.Cause)
	}
}
