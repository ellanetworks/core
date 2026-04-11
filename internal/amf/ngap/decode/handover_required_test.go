// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package decode_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapType"
)

func validTargetID() *ngapType.TargetID {
	return &ngapType.TargetID{
		Present: ngapType.TargetIDPresentTargetRANNodeID,
		TargetRANNodeID: &ngapType.TargetRANNodeID{
			GlobalRANNodeID: ngapType.GlobalRANNodeID{
				Present: ngapType.GlobalRANNodeIDPresentGlobalGNBID,
				GlobalGNBID: &ngapType.GlobalGNBID{
					PLMNIdentity: ngapType.PLMNIdentity{Value: aper.OctetString{0x00, 0xF1, 0x10}},
					GNBID: ngapType.GNBID{
						Present: ngapType.GNBIDPresentGNBID,
						GNBID:   &aper.BitString{Bytes: []byte{0xAB, 0xCD, 0xE2}, BitLength: 24},
					},
				},
			},
			SelectedTAI: ngapType.TAI{
				PLMNIdentity: ngapType.PLMNIdentity{Value: aper.OctetString{0x00, 0xF1, 0x10}},
				TAC:          ngapType.TAC{Value: aper.OctetString{0x00, 0x00, 0x01}},
			},
		},
	}
}

func validHandoverRequired() *ngapType.HandoverRequired {
	msg := &ngapType.HandoverRequired{}

	msg.ProtocolIEs.List = []ngapType.HandoverRequiredIEs{
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDAMFUENGAPID},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentReject},
			Value: ngapType.HandoverRequiredIEsValue{
				Present:     ngapType.HandoverRequiredIEsPresentAMFUENGAPID,
				AMFUENGAPID: &ngapType.AMFUENGAPID{Value: 5},
			},
		},
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDRANUENGAPID},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentReject},
			Value: ngapType.HandoverRequiredIEsValue{
				Present:     ngapType.HandoverRequiredIEsPresentRANUENGAPID,
				RANUENGAPID: &ngapType.RANUENGAPID{Value: 9},
			},
		},
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDHandoverType},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentReject},
			Value: ngapType.HandoverRequiredIEsValue{
				Present:      ngapType.HandoverRequiredIEsPresentHandoverType,
				HandoverType: &ngapType.HandoverType{Value: ngapType.HandoverTypePresentIntra5gs},
			},
		},
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDCause},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
			Value: ngapType.HandoverRequiredIEsValue{
				Present: ngapType.HandoverRequiredIEsPresentCause,
				Cause: &ngapType.Cause{
					Present: ngapType.CausePresentRadioNetwork,
					RadioNetwork: &ngapType.CauseRadioNetwork{
						Value: ngapType.CauseRadioNetworkPresentHandoverDesirableForRadioReason,
					},
				},
			},
		},
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDTargetID},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentReject},
			Value: ngapType.HandoverRequiredIEsValue{
				Present:  ngapType.HandoverRequiredIEsPresentTargetID,
				TargetID: validTargetID(),
			},
		},
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDPDUSessionResourceListHORqd},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentReject},
			Value: ngapType.HandoverRequiredIEsValue{
				Present: ngapType.HandoverRequiredIEsPresentPDUSessionResourceListHORqd,
				PDUSessionResourceListHORqd: &ngapType.PDUSessionResourceListHORqd{
					List: []ngapType.PDUSessionResourceItemHORqd{
						{
							PDUSessionID:             ngapType.PDUSessionID{Value: 1},
							HandoverRequiredTransfer: []byte{0xCA, 0xFE},
						},
					},
				},
			},
		},
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDSourceToTargetTransparentContainer},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentReject},
			Value: ngapType.HandoverRequiredIEsValue{
				Present: ngapType.HandoverRequiredIEsPresentSourceToTargetTransparentContainer,
				SourceToTargetTransparentContainer: &ngapType.SourceToTargetTransparentContainer{
					Value: []byte{0x01, 0x02, 0x03},
				},
			},
		},
	}

	return msg
}

func TestDecodeHandoverRequired_Happy(t *testing.T) {
	msg := validHandoverRequired()

	out, report := decode.DecodeHandoverRequired(msg)
	if report != nil {
		t.Fatalf("expected nil report, got %+v", report)
	}

	if out.AMFUENGAPID != 5 {
		t.Errorf("AMFUENGAPID = %d, want 5", out.AMFUENGAPID)
	}

	if out.RANUENGAPID != 9 {
		t.Errorf("RANUENGAPID = %d, want 9", out.RANUENGAPID)
	}

	if out.HandoverType.Value != ngapType.HandoverTypePresentIntra5gs {
		t.Errorf("HandoverType = %d, want Intra5gs", out.HandoverType.Value)
	}

	if out.Cause.Present != ngapType.CausePresentRadioNetwork {
		t.Errorf("Cause.Present = %d, want RadioNetwork", out.Cause.Present)
	}

	if out.TargetID == nil {
		t.Fatal("TargetID should be non-nil after happy-path decode")
	}

	if len(out.PDUSessionResourceItems) != 1 {
		t.Fatalf("PDUSessionResourceItems len = %d, want 1", len(out.PDUSessionResourceItems))
	}

	if len(out.SourceToTargetTransparentContainer.Value) != 3 {
		t.Errorf("SourceToTargetTransparentContainer len = %d, want 3", len(out.SourceToTargetTransparentContainer.Value))
	}
}

func TestDecodeHandoverRequired_NilBody(t *testing.T) {
	out, report := decode.DecodeHandoverRequired(nil)
	if report == nil {
		t.Fatal("expected non-nil report")
	}

	if !report.Fatal() {
		t.Error("expected fatal report for nil body")
	}

	if !report.ProcedureRejected {
		t.Error("expected ProcedureRejected to be set")
	}

	if len(report.Items) != 0 {
		t.Errorf("expected no per-IE items for nil body, got %+v", report.Items)
	}

	if out.AMFUENGAPID != 0 {
		t.Errorf("expected zero value, got %d", out.AMFUENGAPID)
	}
}

func TestDecodeHandoverRequired_EmptyIEs(t *testing.T) {
	_, report := decode.DecodeHandoverRequired(&ngapType.HandoverRequired{})
	if report == nil || !report.Fatal() {
		t.Fatalf("expected fatal report; got %+v", report)
	}

	wantIEs := map[int64]bool{
		ngapType.ProtocolIEIDAMFUENGAPID:                        true,
		ngapType.ProtocolIEIDRANUENGAPID:                        true,
		ngapType.ProtocolIEIDHandoverType:                       true,
		ngapType.ProtocolIEIDCause:                              true,
		ngapType.ProtocolIEIDTargetID:                           true,
		ngapType.ProtocolIEIDPDUSessionResourceListHORqd:        true,
		ngapType.ProtocolIEIDSourceToTargetTransparentContainer: true,
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

func TestDecodeHandoverRequired_MissingTargetID(t *testing.T) {
	msg := validHandoverRequired()

	filtered := msg.ProtocolIEs.List[:0]
	for _, ie := range msg.ProtocolIEs.List {
		if ie.Id.Value == ngapType.ProtocolIEIDTargetID {
			continue
		}

		filtered = append(filtered, ie)
	}

	msg.ProtocolIEs.List = filtered

	_, report := decode.DecodeHandoverRequired(msg)
	if report == nil || !report.Fatal() {
		t.Fatalf("expected fatal report; got %+v", report)
	}
}

func TestDecodeHandoverRequired_NilAMFUENGAPIDValue(t *testing.T) {
	msg := validHandoverRequired()
	for i := range msg.ProtocolIEs.List {
		if msg.ProtocolIEs.List[i].Id.Value == ngapType.ProtocolIEIDAMFUENGAPID {
			msg.ProtocolIEs.List[i].Value.AMFUENGAPID = nil
		}
	}

	_, report := decode.DecodeHandoverRequired(msg)
	if report == nil || !report.Fatal() {
		t.Fatalf("expected fatal report; got %+v", report)
	}

	// Exactly one item should be present: a Malformed for AMFUENGAPID,
	// not also a MissingMandatory.
	if len(report.Items) != 1 {
		t.Fatalf("expected 1 report item, got %d (%+v)", len(report.Items), report.Items)
	}

	if report.Items[0].IEID != ngapType.ProtocolIEIDAMFUENGAPID {
		t.Errorf("expected AMFUENGAPID item, got IE %d", report.Items[0].IEID)
	}
}

func TestDecodeHandoverRequired_NilCauseValueNonFatal(t *testing.T) {
	msg := validHandoverRequired()
	for i := range msg.ProtocolIEs.List {
		if msg.ProtocolIEs.List[i].Id.Value == ngapType.ProtocolIEIDCause {
			msg.ProtocolIEs.List[i].Value.Cause = nil
		}
	}

	out, report := decode.DecodeHandoverRequired(msg)
	if report == nil {
		t.Fatal("expected non-nil report")
	}

	// Cause is mandatory-ignore: malformed must not be fatal so the
	// handler still gets invoked.
	if report.Fatal() {
		t.Errorf("expected non-fatal report for malformed Cause, got %+v", report)
	}

	if out.Cause.Present != 0 {
		t.Errorf("expected zero Cause on malformed input, got Present=%d", out.Cause.Present)
	}
}

func TestDecodeHandoverRequired_DuplicateIELastWins(t *testing.T) {
	msg := validHandoverRequired()

	msg.ProtocolIEs.List = append(msg.ProtocolIEs.List, ngapType.HandoverRequiredIEs{
		Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDRANUENGAPID},
		Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentReject},
		Value: ngapType.HandoverRequiredIEsValue{
			Present:     ngapType.HandoverRequiredIEsPresentRANUENGAPID,
			RANUENGAPID: &ngapType.RANUENGAPID{Value: 999},
		},
	})

	out, report := decode.DecodeHandoverRequired(msg)
	if report != nil {
		t.Fatalf("expected nil report, got %+v", report)
	}

	if out.RANUENGAPID != 999 {
		t.Errorf("expected last-wins RANUENGAPID=999, got %d", out.RANUENGAPID)
	}
}
