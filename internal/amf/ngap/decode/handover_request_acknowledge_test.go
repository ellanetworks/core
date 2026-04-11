// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package decode_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapType"
)

func validHandoverRequestAcknowledge() *ngapType.HandoverRequestAcknowledge {
	msg := &ngapType.HandoverRequestAcknowledge{}

	msg.ProtocolIEs.List = []ngapType.HandoverRequestAcknowledgeIEs{
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDAMFUENGAPID},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
			Value: ngapType.HandoverRequestAcknowledgeIEsValue{
				Present:     ngapType.HandoverRequestAcknowledgeIEsPresentAMFUENGAPID,
				AMFUENGAPID: &ngapType.AMFUENGAPID{Value: 1},
			},
		},
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDRANUENGAPID},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
			Value: ngapType.HandoverRequestAcknowledgeIEsValue{
				Present:     ngapType.HandoverRequestAcknowledgeIEsPresentRANUENGAPID,
				RANUENGAPID: &ngapType.RANUENGAPID{Value: 2},
			},
		},
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDPDUSessionResourceAdmittedList},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
			Value: ngapType.HandoverRequestAcknowledgeIEsValue{
				Present: ngapType.HandoverRequestAcknowledgeIEsPresentPDUSessionResourceAdmittedList,
				PDUSessionResourceAdmittedList: &ngapType.PDUSessionResourceAdmittedList{
					List: []ngapType.PDUSessionResourceAdmittedItem{
						{
							PDUSessionID:                       ngapType.PDUSessionID{Value: 1},
							HandoverRequestAcknowledgeTransfer: aper.OctetString{0xAA},
						},
					},
				},
			},
		},
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDTargetToSourceTransparentContainer},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentReject},
			Value: ngapType.HandoverRequestAcknowledgeIEsValue{
				Present:                            ngapType.HandoverRequestAcknowledgeIEsPresentTargetToSourceTransparentContainer,
				TargetToSourceTransparentContainer: &ngapType.TargetToSourceTransparentContainer{Value: aper.OctetString{0xBB, 0xCC}},
			},
		},
	}

	return msg
}

func TestDecodeHandoverRequestAcknowledge_Happy(t *testing.T) {
	out, report := decode.DecodeHandoverRequestAcknowledge(validHandoverRequestAcknowledge())
	if report != nil {
		t.Fatalf("expected nil report, got %+v", report)
	}

	if out.AMFUENGAPID == nil || *out.AMFUENGAPID != 1 {
		t.Errorf("AMFUENGAPID = %v, want 1", out.AMFUENGAPID)
	}

	if out.RANUENGAPID == nil || *out.RANUENGAPID != 2 {
		t.Errorf("RANUENGAPID = %v, want 2", out.RANUENGAPID)
	}

	if len(out.AdmittedItems) != 1 {
		t.Fatalf("expected 1 admitted item, got %d", len(out.AdmittedItems))
	}

	if len(out.TargetToSourceTransparentContainer.Value) != 2 {
		t.Errorf("expected 2-byte container, got %d", len(out.TargetToSourceTransparentContainer.Value))
	}
}

func TestDecodeHandoverRequestAcknowledge_NilBody(t *testing.T) {
	_, report := decode.DecodeHandoverRequestAcknowledge(nil)
	if report == nil || !report.Fatal() {
		t.Fatalf("expected fatal report, got %+v", report)
	}

	if !report.ProcedureRejected {
		t.Error("expected ProcedureRejected to be set")
	}
}

func TestDecodeHandoverRequestAcknowledge_EmptyIEsFatal(t *testing.T) {
	_, report := decode.DecodeHandoverRequestAcknowledge(&ngapType.HandoverRequestAcknowledge{})
	if report == nil || !report.Fatal() {
		t.Fatalf("expected fatal report (missing TargetToSourceTransparentContainer), got %+v", report)
	}

	wantIEs := map[int64]bool{
		ngapType.ProtocolIEIDAMFUENGAPID:                        true,
		ngapType.ProtocolIEIDRANUENGAPID:                        true,
		ngapType.ProtocolIEIDPDUSessionResourceAdmittedList:     true,
		ngapType.ProtocolIEIDTargetToSourceTransparentContainer: true,
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

func TestDecodeHandoverRequestAcknowledge_MissingTargetToSourceContainerFatal(t *testing.T) {
	msg := validHandoverRequestAcknowledge()

	filtered := msg.ProtocolIEs.List[:0]
	for _, ie := range msg.ProtocolIEs.List {
		if ie.Id.Value != ngapType.ProtocolIEIDTargetToSourceTransparentContainer {
			filtered = append(filtered, ie)
		}
	}

	msg.ProtocolIEs.List = filtered

	_, report := decode.DecodeHandoverRequestAcknowledge(msg)
	if report == nil || !report.Fatal() {
		t.Fatalf("expected fatal report, got %+v", report)
	}
}

func TestDecodeHandoverRequestAcknowledge_NilTargetToSourceContainerValueFatal(t *testing.T) {
	msg := validHandoverRequestAcknowledge()
	for i := range msg.ProtocolIEs.List {
		if msg.ProtocolIEs.List[i].Id.Value == ngapType.ProtocolIEIDTargetToSourceTransparentContainer {
			msg.ProtocolIEs.List[i].Value.TargetToSourceTransparentContainer = nil
		}
	}

	_, report := decode.DecodeHandoverRequestAcknowledge(msg)
	if report == nil || !report.Fatal() {
		t.Fatalf("expected fatal report, got %+v", report)
	}

	if len(report.Items) != 1 {
		t.Fatalf("expected 1 item (no double-report), got %d", len(report.Items))
	}
}

func TestDecodeHandoverRequestAcknowledge_NilAMFUENGAPIDValueNonFatal(t *testing.T) {
	msg := validHandoverRequestAcknowledge()
	for i := range msg.ProtocolIEs.List {
		if msg.ProtocolIEs.List[i].Id.Value == ngapType.ProtocolIEIDAMFUENGAPID {
			msg.ProtocolIEs.List[i].Value.AMFUENGAPID = nil
		}
	}

	out, report := decode.DecodeHandoverRequestAcknowledge(msg)
	if report == nil {
		t.Fatal("expected non-nil report")
	}

	if report.Fatal() {
		t.Errorf("expected non-fatal report (AMFUENGAPID criticality ignore), got %+v", report)
	}

	if out.AMFUENGAPID != nil {
		t.Errorf("expected nil AMFUENGAPID, got %v", out.AMFUENGAPID)
	}
}

func TestDecodeHandoverRequestAcknowledge_DuplicateIELastWins(t *testing.T) {
	msg := validHandoverRequestAcknowledge()

	msg.ProtocolIEs.List = append(msg.ProtocolIEs.List, ngapType.HandoverRequestAcknowledgeIEs{
		Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDAMFUENGAPID},
		Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
		Value: ngapType.HandoverRequestAcknowledgeIEsValue{
			Present:     ngapType.HandoverRequestAcknowledgeIEsPresentAMFUENGAPID,
			AMFUENGAPID: &ngapType.AMFUENGAPID{Value: 4242},
		},
	})

	out, report := decode.DecodeHandoverRequestAcknowledge(msg)
	if report != nil {
		t.Fatalf("expected nil report, got %+v", report)
	}

	if out.AMFUENGAPID == nil || *out.AMFUENGAPID != 4242 {
		t.Errorf("AMFUENGAPID = %v, want 4242 (last-wins)", out.AMFUENGAPID)
	}
}
