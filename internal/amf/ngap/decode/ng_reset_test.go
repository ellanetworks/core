// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package decode_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/free5gc/ngap/ngapType"
)

func validNGReset() *ngapType.NGReset {
	msg := &ngapType.NGReset{}

	msg.ProtocolIEs.List = []ngapType.NGResetIEs{
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDCause},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
			Value: ngapType.NGResetIEsValue{
				Present: ngapType.NGResetIEsPresentCause,
				Cause: &ngapType.Cause{
					Present:      ngapType.CausePresentRadioNetwork,
					RadioNetwork: &ngapType.CauseRadioNetwork{Value: ngapType.CauseRadioNetworkPresentUnspecified},
				},
			},
		},
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDResetType},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentReject},
			Value: ngapType.NGResetIEsValue{
				Present: ngapType.NGResetIEsPresentResetType,
				ResetType: &ngapType.ResetType{
					Present:     ngapType.ResetTypePresentNGInterface,
					NGInterface: &ngapType.ResetAll{Value: ngapType.ResetAllPresentResetAll},
				},
			},
		},
	}

	return msg
}

func TestDecodeNGReset_Happy(t *testing.T) {
	out, report := decode.DecodeNGReset(validNGReset())
	if report != nil {
		t.Fatalf("expected nil report, got %+v", report)
	}

	if out.Cause.Present != ngapType.CausePresentRadioNetwork {
		t.Errorf("Cause.Present = %d, want RadioNetwork", out.Cause.Present)
	}

	if out.ResetType == nil {
		t.Fatal("expected non-nil ResetType")
	}

	if out.ResetType.Present != ngapType.ResetTypePresentNGInterface {
		t.Errorf("ResetType.Present = %d, want NGInterface", out.ResetType.Present)
	}
}

func TestDecodeNGReset_NilBody(t *testing.T) {
	_, report := decode.DecodeNGReset(nil)
	if report == nil || !report.Fatal() {
		t.Fatalf("expected fatal report, got %+v", report)
	}

	if !report.ProcedureRejected {
		t.Error("expected ProcedureRejected to be set")
	}
}

func TestDecodeNGReset_EmptyIEs(t *testing.T) {
	_, report := decode.DecodeNGReset(&ngapType.NGReset{})
	if report == nil || !report.Fatal() {
		t.Fatalf("expected fatal report (ResetType reject), got %+v", report)
	}

	wantIEs := map[int64]bool{
		ngapType.ProtocolIEIDCause:     true,
		ngapType.ProtocolIEIDResetType: true,
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

func TestDecodeNGReset_NilCauseValueNonFatal(t *testing.T) {
	msg := validNGReset()
	for i := range msg.ProtocolIEs.List {
		if msg.ProtocolIEs.List[i].Id.Value == ngapType.ProtocolIEIDCause {
			msg.ProtocolIEs.List[i].Value.Cause = nil
		}
	}

	out, report := decode.DecodeNGReset(msg)
	if report == nil {
		t.Fatal("expected non-nil report")
	}

	if report.Fatal() {
		t.Errorf("expected non-fatal report (Cause criticality ignore), got %+v", report)
	}

	if out.ResetType == nil {
		t.Error("expected ResetType still populated")
	}
}

func TestDecodeNGReset_NilResetTypeIsFatal(t *testing.T) {
	msg := validNGReset()
	for i := range msg.ProtocolIEs.List {
		if msg.ProtocolIEs.List[i].Id.Value == ngapType.ProtocolIEIDResetType {
			msg.ProtocolIEs.List[i].Value.ResetType = nil
		}
	}

	_, report := decode.DecodeNGReset(msg)
	if report == nil || !report.Fatal() {
		t.Fatalf("expected fatal report (ResetType criticality reject), got %+v", report)
	}

	if len(report.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(report.Items))
	}
}

func TestDecodeNGReset_DuplicateIELastWins(t *testing.T) {
	msg := validNGReset()

	msg.ProtocolIEs.List = append(msg.ProtocolIEs.List, ngapType.NGResetIEs{
		Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDCause},
		Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
		Value: ngapType.NGResetIEsValue{
			Present: ngapType.NGResetIEsPresentCause,
			Cause: &ngapType.Cause{
				Present: ngapType.CausePresentMisc,
				Misc:    &ngapType.CauseMisc{Value: ngapType.CauseMiscPresentUnspecified},
			},
		},
	})

	out, report := decode.DecodeNGReset(msg)
	if report != nil {
		t.Fatalf("expected nil report, got %+v", report)
	}

	if out.Cause.Present != ngapType.CausePresentMisc {
		t.Errorf("expected last-wins Misc cause, got Present=%d", out.Cause.Present)
	}
}
