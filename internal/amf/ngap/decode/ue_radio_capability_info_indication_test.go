// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package decode_test

import (
	"bytes"
	"testing"

	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapType"
)

func validUERadioCapabilityInfoIndication() *ngapType.UERadioCapabilityInfoIndication {
	msg := &ngapType.UERadioCapabilityInfoIndication{}

	msg.ProtocolIEs.List = []ngapType.UERadioCapabilityInfoIndicationIEs{
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDAMFUENGAPID},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentReject},
			Value: ngapType.UERadioCapabilityInfoIndicationIEsValue{
				Present:     ngapType.UERadioCapabilityInfoIndicationIEsPresentAMFUENGAPID,
				AMFUENGAPID: &ngapType.AMFUENGAPID{Value: 5},
			},
		},
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDRANUENGAPID},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentReject},
			Value: ngapType.UERadioCapabilityInfoIndicationIEsValue{
				Present:     ngapType.UERadioCapabilityInfoIndicationIEsPresentRANUENGAPID,
				RANUENGAPID: &ngapType.RANUENGAPID{Value: 9},
			},
		},
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDUERadioCapability},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
			Value: ngapType.UERadioCapabilityInfoIndicationIEsValue{
				Present:           ngapType.UERadioCapabilityInfoIndicationIEsPresentUERadioCapability,
				UERadioCapability: &ngapType.UERadioCapability{Value: aper.OctetString{0xAA, 0xBB, 0xCC}},
			},
		},
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDUERadioCapabilityForPaging},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
			Value: ngapType.UERadioCapabilityInfoIndicationIEsValue{
				Present:                    ngapType.UERadioCapabilityInfoIndicationIEsPresentUERadioCapabilityForPaging,
				UERadioCapabilityForPaging: &ngapType.UERadioCapabilityForPaging{},
			},
		},
	}

	return msg
}

func TestDecodeUERadioCapabilityInfoIndication_Happy(t *testing.T) {
	out, report := decode.DecodeUERadioCapabilityInfoIndication(validUERadioCapabilityInfoIndication())
	if report != nil {
		t.Fatalf("expected nil report, got %+v", report)
	}

	if out.AMFUENGAPID != 5 || out.RANUENGAPID != 9 {
		t.Errorf("UE NGAP IDs = (%d, %d), want (5, 9)", out.AMFUENGAPID, out.RANUENGAPID)
	}

	if !bytes.Equal(out.UERadioCapability, []byte{0xAA, 0xBB, 0xCC}) {
		t.Errorf("UERadioCapability = %x, want AABBCC", out.UERadioCapability)
	}

	if out.UERadioCapabilityForPaging == nil {
		t.Error("expected non-nil UERadioCapabilityForPaging")
	}
}

func TestDecodeUERadioCapabilityInfoIndication_NilBody(t *testing.T) {
	_, report := decode.DecodeUERadioCapabilityInfoIndication(nil)
	if report == nil || !report.Fatal() {
		t.Fatalf("expected fatal report, got %+v", report)
	}

	if !report.ProcedureRejected {
		t.Error("expected ProcedureRejected to be set")
	}
}

func TestDecodeUERadioCapabilityInfoIndication_EmptyIEs(t *testing.T) {
	_, report := decode.DecodeUERadioCapabilityInfoIndication(&ngapType.UERadioCapabilityInfoIndication{})
	if report == nil || !report.Fatal() {
		t.Fatalf("expected fatal report (missing UE NGAP IDs), got %+v", report)
	}

	wantIEs := map[int64]bool{
		ngapType.ProtocolIEIDAMFUENGAPID:       true,
		ngapType.ProtocolIEIDRANUENGAPID:       true,
		ngapType.ProtocolIEIDUERadioCapability: true,
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

func TestDecodeUERadioCapabilityInfoIndication_OptionalAbsent(t *testing.T) {
	msg := validUERadioCapabilityInfoIndication()

	filtered := msg.ProtocolIEs.List[:0]
	for _, ie := range msg.ProtocolIEs.List {
		if ie.Id.Value != ngapType.ProtocolIEIDUERadioCapabilityForPaging {
			filtered = append(filtered, ie)
		}
	}

	msg.ProtocolIEs.List = filtered

	out, report := decode.DecodeUERadioCapabilityInfoIndication(msg)
	if report != nil {
		t.Fatalf("expected nil report, got %+v", report)
	}

	if out.UERadioCapabilityForPaging != nil {
		t.Error("expected nil UERadioCapabilityForPaging when absent")
	}
}

func TestDecodeUERadioCapabilityInfoIndication_NilCapabilityValueNonFatal(t *testing.T) {
	msg := validUERadioCapabilityInfoIndication()
	for i := range msg.ProtocolIEs.List {
		if msg.ProtocolIEs.List[i].Id.Value == ngapType.ProtocolIEIDUERadioCapability {
			msg.ProtocolIEs.List[i].Value.UERadioCapability = nil
		}
	}

	out, report := decode.DecodeUERadioCapabilityInfoIndication(msg)
	if report == nil {
		t.Fatal("expected non-nil report")
	}

	if report.Fatal() {
		t.Errorf("expected non-fatal report, got %+v", report)
	}

	if out.UERadioCapability != nil {
		t.Error("expected nil UERadioCapability when malformed")
	}
}

func TestDecodeUERadioCapabilityInfoIndication_NilRANUENGAPIDValueIsFatal(t *testing.T) {
	msg := validUERadioCapabilityInfoIndication()
	for i := range msg.ProtocolIEs.List {
		if msg.ProtocolIEs.List[i].Id.Value == ngapType.ProtocolIEIDRANUENGAPID {
			msg.ProtocolIEs.List[i].Value.RANUENGAPID = nil
		}
	}

	_, report := decode.DecodeUERadioCapabilityInfoIndication(msg)
	if report == nil || !report.Fatal() {
		t.Fatalf("expected fatal report, got %+v", report)
	}

	if len(report.Items) != 1 {
		t.Fatalf("expected 1 item (no double-report), got %d", len(report.Items))
	}
}

func TestDecodeUERadioCapabilityInfoIndication_DuplicateIELastWins(t *testing.T) {
	msg := validUERadioCapabilityInfoIndication()

	msg.ProtocolIEs.List = append(msg.ProtocolIEs.List, ngapType.UERadioCapabilityInfoIndicationIEs{
		Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDUERadioCapability},
		Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
		Value: ngapType.UERadioCapabilityInfoIndicationIEsValue{
			Present:           ngapType.UERadioCapabilityInfoIndicationIEsPresentUERadioCapability,
			UERadioCapability: &ngapType.UERadioCapability{Value: aper.OctetString{0x11, 0x22}},
		},
	})

	out, report := decode.DecodeUERadioCapabilityInfoIndication(msg)
	if report != nil {
		t.Fatalf("expected nil report, got %+v", report)
	}

	if !bytes.Equal(out.UERadioCapability, []byte{0x11, 0x22}) {
		t.Errorf("UERadioCapability = %x, want 1122 (last-wins)", out.UERadioCapability)
	}
}
