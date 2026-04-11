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

func validUplinkNASTransport() *ngapType.UplinkNASTransport {
	msg := &ngapType.UplinkNASTransport{}

	msg.ProtocolIEs.List = []ngapType.UplinkNASTransportIEs{
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDAMFUENGAPID},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentReject},
			Value: ngapType.UplinkNASTransportIEsValue{
				Present:     ngapType.UplinkNASTransportIEsPresentAMFUENGAPID,
				AMFUENGAPID: &ngapType.AMFUENGAPID{Value: 21},
			},
		},
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDRANUENGAPID},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentReject},
			Value: ngapType.UplinkNASTransportIEsValue{
				Present:     ngapType.UplinkNASTransportIEsPresentRANUENGAPID,
				RANUENGAPID: &ngapType.RANUENGAPID{Value: 42},
			},
		},
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDNASPDU},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentReject},
			Value: ngapType.UplinkNASTransportIEsValue{
				Present: ngapType.UplinkNASTransportIEsPresentNASPDU,
				NASPDU:  &ngapType.NASPDU{Value: aper.OctetString{0x7E, 0x00, 0x55}},
			},
		},
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDUserLocationInformation},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
			Value: ngapType.UplinkNASTransportIEsValue{
				Present: ngapType.UplinkNASTransportIEsPresentUserLocationInformation,
				UserLocationInformation: &ngapType.UserLocationInformation{
					Present:                   ngapType.UserLocationInformationPresentUserLocationInformationNR,
					UserLocationInformationNR: &ngapType.UserLocationInformationNR{},
				},
			},
		},
	}

	return msg
}

func TestDecodeUplinkNASTransport_Happy(t *testing.T) {
	msg := validUplinkNASTransport()

	out, report := decode.DecodeUplinkNASTransport(msg)
	if report != nil {
		t.Fatalf("expected nil report, got %+v", report)
	}

	if out.AMFUENGAPID != 21 {
		t.Errorf("AMFUENGAPID = %d, want 21", out.AMFUENGAPID)
	}

	if out.RANUENGAPID != 42 {
		t.Errorf("RANUENGAPID = %d, want 42", out.RANUENGAPID)
	}

	if !bytes.Equal(out.NASPDU, []byte{0x7E, 0x00, 0x55}) {
		t.Errorf("NASPDU = % x, want 7e 00 55", out.NASPDU)
	}

	if out.UserLocationInformation.Kind() != decode.UserLocationKindNR {
		t.Errorf("ULI kind = %d, want NR", out.UserLocationInformation.Kind())
	}
}

func TestDecodeUplinkNASTransport_NASPDUIsCopied(t *testing.T) {
	// The decoder must not alias the source PDU buffer for NASPDU
	// because the handler may stash it past the synchronous handler
	// invocation (e.g. when forwarded to NAS processing).
	msg := validUplinkNASTransport()

	out, report := decode.DecodeUplinkNASTransport(msg)
	if report != nil {
		t.Fatalf("expected nil report, got %+v", report)
	}

	// Mutate the source bytes; the decoded copy must not change.
	for i := range msg.ProtocolIEs.List {
		if msg.ProtocolIEs.List[i].Id.Value == ngapType.ProtocolIEIDNASPDU {
			msg.ProtocolIEs.List[i].Value.NASPDU.Value[0] = 0xFF
		}
	}

	if out.NASPDU[0] != 0x7E {
		t.Errorf("NASPDU was aliased: out[0]=%x, want 7e", out.NASPDU[0])
	}
}

func TestDecodeUplinkNASTransport_NilBody(t *testing.T) {
	out, report := decode.DecodeUplinkNASTransport(nil)
	if report == nil {
		t.Fatal("expected non-nil report")
	}

	if !report.Fatal() {
		t.Error("expected fatal report for nil body")
	}

	if !report.ProcedureRejected {
		t.Error("expected ProcedureRejected to be set")
	}

	if out.AMFUENGAPID != 0 {
		t.Errorf("expected zero value, got %d", out.AMFUENGAPID)
	}
}

func TestDecodeUplinkNASTransport_EmptyIEs(t *testing.T) {
	_, report := decode.DecodeUplinkNASTransport(&ngapType.UplinkNASTransport{})
	if report == nil || !report.Fatal() {
		t.Fatalf("expected fatal report; got %+v", report)
	}

	wantIEs := map[int64]bool{
		ngapType.ProtocolIEIDAMFUENGAPID:             true,
		ngapType.ProtocolIEIDRANUENGAPID:             true,
		ngapType.ProtocolIEIDNASPDU:                  true,
		ngapType.ProtocolIEIDUserLocationInformation: true,
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

func TestDecodeUplinkNASTransport_NilNASPDUValue(t *testing.T) {
	msg := validUplinkNASTransport()
	for i := range msg.ProtocolIEs.List {
		if msg.ProtocolIEs.List[i].Id.Value == ngapType.ProtocolIEIDNASPDU {
			msg.ProtocolIEs.List[i].Value.NASPDU = nil
		}
	}

	_, report := decode.DecodeUplinkNASTransport(msg)
	if report == nil || !report.Fatal() {
		t.Fatalf("expected fatal report; got %+v", report)
	}

	if len(report.Items) != 1 {
		t.Fatalf("expected 1 report item, got %d (%+v)", len(report.Items), report.Items)
	}

	if report.Items[0].IEID != ngapType.ProtocolIEIDNASPDU {
		t.Errorf("expected NASPDU item, got IE %d", report.Items[0].IEID)
	}
}

func TestDecodeUplinkNASTransport_EmptyNASPDUBytes(t *testing.T) {
	msg := validUplinkNASTransport()
	for i := range msg.ProtocolIEs.List {
		if msg.ProtocolIEs.List[i].Id.Value == ngapType.ProtocolIEIDNASPDU {
			msg.ProtocolIEs.List[i].Value.NASPDU = &ngapType.NASPDU{Value: aper.OctetString{}}
		}
	}

	_, report := decode.DecodeUplinkNASTransport(msg)
	if report == nil || !report.Fatal() {
		t.Fatalf("expected fatal report for empty NAS-PDU; got %+v", report)
	}
}

func TestDecodeUplinkNASTransport_MalformedULIIsNonFatal(t *testing.T) {
	msg := validUplinkNASTransport()
	for i := range msg.ProtocolIEs.List {
		if msg.ProtocolIEs.List[i].Id.Value == ngapType.ProtocolIEIDUserLocationInformation {
			msg.ProtocolIEs.List[i].Value.UserLocationInformation = &ngapType.UserLocationInformation{
				Present: ngapType.UserLocationInformationPresentUserLocationInformationNR,
				// inner pointer nil → malformed
			}
		}
	}

	out, report := decode.DecodeUplinkNASTransport(msg)
	if report == nil {
		t.Fatal("expected non-nil report")
	}

	// UserLocationInformation is mandatory-ignore: malformed must not
	// be fatal so the handler still gets invoked.
	if report.Fatal() {
		t.Errorf("expected non-fatal report for malformed ULI, got %+v", report)
	}

	if out.UserLocationInformation.Kind() != decode.UserLocationKindUnknown {
		t.Errorf("expected zero ULI kind on malformed input, got %d", out.UserLocationInformation.Kind())
	}
}

func TestDecodeUplinkNASTransport_DuplicateIELastWins(t *testing.T) {
	msg := validUplinkNASTransport()

	msg.ProtocolIEs.List = append(msg.ProtocolIEs.List, ngapType.UplinkNASTransportIEs{
		Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDAMFUENGAPID},
		Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentReject},
		Value: ngapType.UplinkNASTransportIEsValue{
			Present:     ngapType.UplinkNASTransportIEsPresentAMFUENGAPID,
			AMFUENGAPID: &ngapType.AMFUENGAPID{Value: 9999},
		},
	})

	out, report := decode.DecodeUplinkNASTransport(msg)
	if report != nil {
		t.Fatalf("expected nil report, got %+v", report)
	}

	if out.AMFUENGAPID != 9999 {
		t.Errorf("expected last-wins AMFUENGAPID=9999, got %d", out.AMFUENGAPID)
	}
}
