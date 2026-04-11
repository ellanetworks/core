// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package decode_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapType"
)

func validRANConfigurationUpdate() *ngapType.RANConfigurationUpdate {
	msg := &ngapType.RANConfigurationUpdate{}

	msg.ProtocolIEs.List = []ngapType.RANConfigurationUpdateIEs{
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDRANNodeName},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
			Value: ngapType.RANConfigurationUpdateIEsValue{
				Present:     ngapType.RANConfigurationUpdateIEsPresentRANNodeName,
				RANNodeName: &ngapType.RANNodeName{Value: "test-gnb"},
			},
		},
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDSupportedTAList},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentReject},
			Value: ngapType.RANConfigurationUpdateIEsValue{
				Present: ngapType.RANConfigurationUpdateIEsPresentSupportedTAList,
				SupportedTAList: &ngapType.SupportedTAList{
					List: []ngapType.SupportedTAItem{
						{
							TAC: ngapType.TAC{Value: aper.OctetString{0x00, 0x00, 0x01}},
							BroadcastPLMNList: ngapType.BroadcastPLMNList{
								List: []ngapType.BroadcastPLMNItem{
									{
										PLMNIdentity: ngapType.PLMNIdentity{Value: aper.OctetString{0x00, 0xF1, 0x10}},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDDefaultPagingDRX},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
			Value: ngapType.RANConfigurationUpdateIEsValue{
				Present:          ngapType.RANConfigurationUpdateIEsPresentDefaultPagingDRX,
				DefaultPagingDRX: &ngapType.PagingDRX{Value: ngapType.PagingDRXPresentV128},
			},
		},
	}

	return msg
}

func TestDecodeRANConfigurationUpdate_Happy(t *testing.T) {
	out, report := decode.DecodeRANConfigurationUpdate(validRANConfigurationUpdate())
	if report != nil {
		t.Fatalf("expected nil report, got %+v", report)
	}

	if len(out.SupportedTAItems) != 1 {
		t.Fatalf("expected 1 supported TA item, got %d", len(out.SupportedTAItems))
	}

	if len(out.SupportedTAItems[0].BroadcastPLMNList.List) != 1 {
		t.Errorf("expected 1 broadcast PLMN, got %d", len(out.SupportedTAItems[0].BroadcastPLMNList.List))
	}
}

func TestDecodeRANConfigurationUpdate_NilBody(t *testing.T) {
	_, report := decode.DecodeRANConfigurationUpdate(nil)
	if report == nil || !report.Fatal() {
		t.Fatalf("expected fatal report, got %+v", report)
	}

	if !report.ProcedureRejected {
		t.Error("expected ProcedureRejected to be set")
	}
}

func TestDecodeRANConfigurationUpdate_EmptyIEsIsNonFatal(t *testing.T) {
	// All IEs are optional — an empty body is structurally valid.
	out, report := decode.DecodeRANConfigurationUpdate(&ngapType.RANConfigurationUpdate{})
	if report != nil {
		t.Fatalf("expected nil report (all IEs optional), got %+v", report)
	}

	if out.SupportedTAItems != nil {
		t.Errorf("expected nil SupportedTAItems on absent IE, got %+v", out.SupportedTAItems)
	}
}

func TestDecodeRANConfigurationUpdate_PresentEmptyTAListIsNonNil(t *testing.T) {
	msg := validRANConfigurationUpdate()
	for i := range msg.ProtocolIEs.List {
		if msg.ProtocolIEs.List[i].Id.Value == ngapType.ProtocolIEIDSupportedTAList {
			msg.ProtocolIEs.List[i].Value.SupportedTAList = &ngapType.SupportedTAList{}
		}
	}

	out, report := decode.DecodeRANConfigurationUpdate(msg)
	if report != nil {
		t.Fatalf("expected nil report, got %+v", report)
	}

	if out.SupportedTAItems == nil {
		t.Error("expected non-nil empty slice when IE present with no items, got nil")
	}

	if len(out.SupportedTAItems) != 0 {
		t.Errorf("expected zero items, got %d", len(out.SupportedTAItems))
	}
}

func TestDecodeRANConfigurationUpdate_NilSupportedTAListIsFatal(t *testing.T) {
	msg := validRANConfigurationUpdate()
	for i := range msg.ProtocolIEs.List {
		if msg.ProtocolIEs.List[i].Id.Value == ngapType.ProtocolIEIDSupportedTAList {
			msg.ProtocolIEs.List[i].Value.SupportedTAList = nil
		}
	}

	_, report := decode.DecodeRANConfigurationUpdate(msg)
	if report == nil || !report.Fatal() {
		t.Fatalf("expected fatal report (SupportedTAList malformed, criticality reject), got %+v", report)
	}
}

func TestDecodeRANConfigurationUpdate_NilRANNodeNameIsNonFatal(t *testing.T) {
	msg := validRANConfigurationUpdate()
	for i := range msg.ProtocolIEs.List {
		if msg.ProtocolIEs.List[i].Id.Value == ngapType.ProtocolIEIDRANNodeName {
			msg.ProtocolIEs.List[i].Value.RANNodeName = nil
		}
	}

	out, report := decode.DecodeRANConfigurationUpdate(msg)
	if report == nil {
		t.Fatal("expected non-nil report for malformed RANNodeName")
	}

	if report.Fatal() {
		t.Errorf("expected non-fatal report (RANNodeName criticality ignore), got %+v", report)
	}

	// SupportedTAItems should still come through.
	if len(out.SupportedTAItems) != 1 {
		t.Errorf("expected SupportedTAItems still populated, got %+v", out.SupportedTAItems)
	}
}

func TestDecodeRANConfigurationUpdate_NilDefaultPagingDRXIsNonFatal(t *testing.T) {
	msg := validRANConfigurationUpdate()
	for i := range msg.ProtocolIEs.List {
		if msg.ProtocolIEs.List[i].Id.Value == ngapType.ProtocolIEIDDefaultPagingDRX {
			msg.ProtocolIEs.List[i].Value.DefaultPagingDRX = nil
		}
	}

	_, report := decode.DecodeRANConfigurationUpdate(msg)
	if report == nil {
		t.Fatal("expected non-nil report")
	}

	if report.Fatal() {
		t.Errorf("expected non-fatal report (DefaultPagingDRX criticality ignore), got %+v", report)
	}
}

func TestDecodeRANConfigurationUpdate_DuplicateIELastWins(t *testing.T) {
	msg := validRANConfigurationUpdate()

	msg.ProtocolIEs.List = append(msg.ProtocolIEs.List, ngapType.RANConfigurationUpdateIEs{
		Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDSupportedTAList},
		Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentReject},
		Value: ngapType.RANConfigurationUpdateIEsValue{
			Present: ngapType.RANConfigurationUpdateIEsPresentSupportedTAList,
			SupportedTAList: &ngapType.SupportedTAList{
				List: []ngapType.SupportedTAItem{
					{TAC: ngapType.TAC{Value: aper.OctetString{0x12, 0x34, 0x56}}},
					{TAC: ngapType.TAC{Value: aper.OctetString{0x78, 0x9A, 0xBC}}},
				},
			},
		},
	})

	out, report := decode.DecodeRANConfigurationUpdate(msg)
	if report != nil {
		t.Fatalf("expected nil report, got %+v", report)
	}

	if len(out.SupportedTAItems) != 2 {
		t.Errorf("expected last-wins 2 TA items, got %d", len(out.SupportedTAItems))
	}
}
