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

func validNASNonDeliveryIndication() *ngapType.NASNonDeliveryIndication {
	msg := &ngapType.NASNonDeliveryIndication{}

	msg.ProtocolIEs.List = []ngapType.NASNonDeliveryIndicationIEs{
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDAMFUENGAPID},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentReject},
			Value: ngapType.NASNonDeliveryIndicationIEsValue{
				Present:     ngapType.NASNonDeliveryIndicationIEsPresentAMFUENGAPID,
				AMFUENGAPID: &ngapType.AMFUENGAPID{Value: 5},
			},
		},
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDRANUENGAPID},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentReject},
			Value: ngapType.NASNonDeliveryIndicationIEsValue{
				Present:     ngapType.NASNonDeliveryIndicationIEsPresentRANUENGAPID,
				RANUENGAPID: &ngapType.RANUENGAPID{Value: 9},
			},
		},
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDNASPDU},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
			Value: ngapType.NASNonDeliveryIndicationIEsValue{
				Present: ngapType.NASNonDeliveryIndicationIEsPresentNASPDU,
				NASPDU:  &ngapType.NASPDU{Value: aper.OctetString{0xDE, 0xAD, 0xBE, 0xEF}},
			},
		},
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDCause},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
			Value: ngapType.NASNonDeliveryIndicationIEsValue{
				Present: ngapType.NASNonDeliveryIndicationIEsPresentCause,
				Cause: &ngapType.Cause{
					Present:   ngapType.CausePresentTransport,
					Transport: &ngapType.CauseTransport{Value: ngapType.CauseTransportPresentTransportResourceUnavailable},
				},
			},
		},
	}

	return msg
}

func TestDecodeNASNonDeliveryIndication_Happy(t *testing.T) {
	out, report := decode.DecodeNASNonDeliveryIndication(validNASNonDeliveryIndication())
	if report != nil {
		t.Fatalf("expected nil report, got %+v", report)
	}

	if out.AMFUENGAPID != 5 || out.RANUENGAPID != 9 {
		t.Errorf("UE NGAP IDs = (%d, %d), want (5, 9)", out.AMFUENGAPID, out.RANUENGAPID)
	}

	if !bytes.Equal(out.NASPDU, []byte{0xDE, 0xAD, 0xBE, 0xEF}) {
		t.Errorf("NASPDU = %x, want DEADBEEF", out.NASPDU)
	}

	if out.Cause.Present != ngapType.CausePresentTransport {
		t.Errorf("Cause.Present = %d, want Transport", out.Cause.Present)
	}
}

func TestDecodeNASNonDeliveryIndication_NASPDUCopiedNotAliased(t *testing.T) {
	msg := validNASNonDeliveryIndication()

	out, report := decode.DecodeNASNonDeliveryIndication(msg)
	if report != nil {
		t.Fatalf("expected nil report, got %+v", report)
	}

	for i := range msg.ProtocolIEs.List {
		if msg.ProtocolIEs.List[i].Id.Value == ngapType.ProtocolIEIDNASPDU {
			msg.ProtocolIEs.List[i].Value.NASPDU.Value[0] = 0xFF
		}
	}

	if out.NASPDU[0] != 0xDE {
		t.Errorf("NASPDU was aliased, expected copy: out[0]=%#x", out.NASPDU[0])
	}
}

func TestDecodeNASNonDeliveryIndication_NilBody(t *testing.T) {
	_, report := decode.DecodeNASNonDeliveryIndication(nil)
	if report == nil || !report.Fatal() {
		t.Fatalf("expected fatal report, got %+v", report)
	}

	if !report.ProcedureRejected {
		t.Error("expected ProcedureRejected to be set")
	}
}

func TestDecodeNASNonDeliveryIndication_EmptyIEs(t *testing.T) {
	_, report := decode.DecodeNASNonDeliveryIndication(&ngapType.NASNonDeliveryIndication{})
	if report == nil || !report.Fatal() {
		t.Fatalf("expected fatal report (missing UE NGAP IDs), got %+v", report)
	}

	wantIEs := map[int64]bool{
		ngapType.ProtocolIEIDAMFUENGAPID: true,
		ngapType.ProtocolIEIDRANUENGAPID: true,
		ngapType.ProtocolIEIDNASPDU:      true,
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

func TestDecodeNASNonDeliveryIndication_EmptyNASPDUNonFatal(t *testing.T) {
	msg := validNASNonDeliveryIndication()
	for i := range msg.ProtocolIEs.List {
		if msg.ProtocolIEs.List[i].Id.Value == ngapType.ProtocolIEIDNASPDU {
			msg.ProtocolIEs.List[i].Value.NASPDU = &ngapType.NASPDU{Value: aper.OctetString{}}
		}
	}

	out, report := decode.DecodeNASNonDeliveryIndication(msg)
	if report == nil {
		t.Fatal("expected non-nil report")
	}

	if report.Fatal() {
		t.Errorf("expected non-fatal report (NASPDU criticality ignore), got %+v", report)
	}

	if len(out.NASPDU) != 0 {
		t.Errorf("expected empty NASPDU, got %x", out.NASPDU)
	}
}

func TestDecodeNASNonDeliveryIndication_NilAMFUENGAPIDValueIsFatal(t *testing.T) {
	msg := validNASNonDeliveryIndication()
	for i := range msg.ProtocolIEs.List {
		if msg.ProtocolIEs.List[i].Id.Value == ngapType.ProtocolIEIDAMFUENGAPID {
			msg.ProtocolIEs.List[i].Value.AMFUENGAPID = nil
		}
	}

	_, report := decode.DecodeNASNonDeliveryIndication(msg)
	if report == nil || !report.Fatal() {
		t.Fatalf("expected fatal report, got %+v", report)
	}

	if len(report.Items) != 1 {
		t.Fatalf("expected 1 item (no double-report), got %d", len(report.Items))
	}
}

func TestDecodeNASNonDeliveryIndication_DuplicateIELastWins(t *testing.T) {
	msg := validNASNonDeliveryIndication()

	msg.ProtocolIEs.List = append(msg.ProtocolIEs.List, ngapType.NASNonDeliveryIndicationIEs{
		Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDNASPDU},
		Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
		Value: ngapType.NASNonDeliveryIndicationIEsValue{
			Present: ngapType.NASNonDeliveryIndicationIEsPresentNASPDU,
			NASPDU:  &ngapType.NASPDU{Value: aper.OctetString{0x11, 0x22}},
		},
	})

	out, report := decode.DecodeNASNonDeliveryIndication(msg)
	if report != nil {
		t.Fatalf("expected nil report, got %+v", report)
	}

	if !bytes.Equal(out.NASPDU, []byte{0x11, 0x22}) {
		t.Errorf("NASPDU = %x, want 1122 (last-wins)", out.NASPDU)
	}
}
