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

func validInitialUEMessage() *ngapType.InitialUEMessage {
	msg := &ngapType.InitialUEMessage{}

	msg.ProtocolIEs.List = []ngapType.InitialUEMessageIEs{
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDRANUENGAPID},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentReject},
			Value: ngapType.InitialUEMessageIEsValue{
				Present:     ngapType.InitialUEMessageIEsPresentRANUENGAPID,
				RANUENGAPID: &ngapType.RANUENGAPID{Value: 42},
			},
		},
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDNASPDU},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentReject},
			Value: ngapType.InitialUEMessageIEsValue{
				Present: ngapType.InitialUEMessageIEsPresentNASPDU,
				NASPDU:  &ngapType.NASPDU{Value: aper.OctetString{0xDE, 0xAD, 0xBE, 0xEF}},
			},
		},
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDUserLocationInformation},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentReject},
			Value: ngapType.InitialUEMessageIEsValue{
				Present: ngapType.InitialUEMessageIEsPresentUserLocationInformation,
				UserLocationInformation: &ngapType.UserLocationInformation{
					Present:                   ngapType.UserLocationInformationPresentUserLocationInformationNR,
					UserLocationInformationNR: &ngapType.UserLocationInformationNR{},
				},
			},
		},
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDRRCEstablishmentCause},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
			Value: ngapType.InitialUEMessageIEsValue{
				Present:               ngapType.InitialUEMessageIEsPresentRRCEstablishmentCause,
				RRCEstablishmentCause: &ngapType.RRCEstablishmentCause{Value: ngapType.RRCEstablishmentCausePresentMoSignalling},
			},
		},
	}

	return msg
}

func validFiveGSTMSI() *ngapType.FiveGSTMSI {
	return &ngapType.FiveGSTMSI{
		AMFSetID: ngapType.AMFSetID{
			Value: aper.BitString{Bytes: []byte{0xC0, 0x00}, BitLength: 10},
		},
		AMFPointer: ngapType.AMFPointer{
			Value: aper.BitString{Bytes: []byte{0x40}, BitLength: 6},
		},
		FiveGTMSI: ngapType.FiveGTMSI{Value: aper.OctetString{0x01, 0x02, 0x03, 0x04}},
	}
}

func TestDecodeInitialUEMessage_Happy(t *testing.T) {
	msg := validInitialUEMessage()

	out, report := decode.DecodeInitialUEMessage(msg)
	if report != nil {
		t.Fatalf("expected nil report, got %+v", report)
	}

	if out.RANUENGAPID != 42 {
		t.Errorf("RANUENGAPID = %d, want 42", out.RANUENGAPID)
	}

	if !bytes.Equal(out.NASPDU, []byte{0xDE, 0xAD, 0xBE, 0xEF}) {
		t.Errorf("NASPDU = %x, want deadbeef", out.NASPDU)
	}

	if out.UserLocationInformation.Kind() != decode.UserLocationKindNR {
		t.Errorf("ULI kind = %d, want NR", out.UserLocationInformation.Kind())
	}

	if out.UserLocationInformation.Raw() == nil {
		t.Errorf("ULI raw is nil")
	}

	if out.RRCEstablishmentCause != decode.RRCEstablishmentCause(ngapType.RRCEstablishmentCausePresentMoSignalling) {
		t.Errorf("unexpected RRCEstablishmentCause %d", out.RRCEstablishmentCause)
	}

	if out.FiveGSTMSI != nil {
		t.Errorf("FiveGSTMSI should be nil when IE absent")
	}

	if out.UEContextRequest {
		t.Errorf("UEContextRequest should default to false")
	}
}

func TestDecodeInitialUEMessage_NASPDUCopiesBytes(t *testing.T) {
	msg := validInitialUEMessage()

	out, report := decode.DecodeInitialUEMessage(msg)
	if report != nil {
		t.Fatalf("unexpected report: %+v", report)
	}

	// Mutate the source buffer; the decoded value must be unaffected.
	for _, ie := range msg.ProtocolIEs.List {
		if ie.Id.Value == ngapType.ProtocolIEIDNASPDU {
			ie.Value.NASPDU.Value[0] = 0x00
		}
	}

	if out.NASPDU[0] != 0xDE {
		t.Errorf("decoded NASPDU was aliased to source buffer; got %x", out.NASPDU)
	}
}

func TestDecodeInitialUEMessage_NilBody(t *testing.T) {
	out, report := decode.DecodeInitialUEMessage(nil)
	if report == nil {
		t.Fatal("expected non-nil report")
	}

	if !report.Fatal() {
		t.Errorf("expected fatal report for nil body")
	}

	if out.RANUENGAPID != 0 {
		t.Errorf("expected zero value")
	}
}

func TestDecodeInitialUEMessage_MissingMandatoryIEs(t *testing.T) {
	msg := &ngapType.InitialUEMessage{}

	_, report := decode.DecodeInitialUEMessage(msg)
	if report == nil {
		t.Fatal("expected non-nil report")
	}

	if !report.Fatal() {
		t.Errorf("expected fatal report when all mandatory IEs missing")
	}

	wantIEs := map[int64]bool{
		ngapType.ProtocolIEIDRANUENGAPID:             true,
		ngapType.ProtocolIEIDNASPDU:                  true,
		ngapType.ProtocolIEIDUserLocationInformation: true,
		ngapType.ProtocolIEIDRRCEstablishmentCause:   true,
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

func TestDecodeInitialUEMessage_NilRANUENGAPIDValue(t *testing.T) {
	msg := validInitialUEMessage()
	for i := range msg.ProtocolIEs.List {
		if msg.ProtocolIEs.List[i].Id.Value == ngapType.ProtocolIEIDRANUENGAPID {
			msg.ProtocolIEs.List[i].Value.RANUENGAPID = nil
		}
	}

	_, report := decode.DecodeInitialUEMessage(msg)
	if report == nil || !report.Fatal() {
		t.Fatalf("expected fatal report; got %+v", report)
	}

	found := false

	for _, item := range report.Items {
		if item.IEID == ngapType.ProtocolIEIDRANUENGAPID {
			found = true

			if item.TypeOfError != ngapType.TypeOfErrorPresentNotUnderstood {
				t.Errorf("expected TypeOfError NotUnderstood, got %d", item.TypeOfError)
			}
		}
	}

	if !found {
		t.Errorf("expected report item for RANUENGAPID")
	}
}

func TestDecodeInitialUEMessage_EmptyNASPDU(t *testing.T) {
	msg := validInitialUEMessage()
	for i := range msg.ProtocolIEs.List {
		if msg.ProtocolIEs.List[i].Id.Value == ngapType.ProtocolIEIDNASPDU {
			msg.ProtocolIEs.List[i].Value.NASPDU = &ngapType.NASPDU{Value: aper.OctetString{}}
		}
	}

	_, report := decode.DecodeInitialUEMessage(msg)
	if report == nil || !report.Fatal() {
		t.Fatalf("expected fatal report for empty NASPDU; got %+v", report)
	}
}

func TestDecodeInitialUEMessage_NilNASPDUValue(t *testing.T) {
	msg := validInitialUEMessage()
	for i := range msg.ProtocolIEs.List {
		if msg.ProtocolIEs.List[i].Id.Value == ngapType.ProtocolIEIDNASPDU {
			msg.ProtocolIEs.List[i].Value.NASPDU = nil
		}
	}

	_, report := decode.DecodeInitialUEMessage(msg)
	if report == nil || !report.Fatal() {
		t.Fatalf("expected fatal report for nil NASPDU; got %+v", report)
	}
}

func TestDecodeInitialUEMessage_MalformedUserLocation(t *testing.T) {
	cases := []struct {
		name string
		uli  *ngapType.UserLocationInformation
	}{
		{
			name: "nil pointer",
			uli:  nil,
		},
		{
			name: "NR present but variant nil",
			uli: &ngapType.UserLocationInformation{
				Present: ngapType.UserLocationInformationPresentUserLocationInformationNR,
			},
		},
		{
			name: "EUTRA present but variant nil",
			uli: &ngapType.UserLocationInformation{
				Present: ngapType.UserLocationInformationPresentUserLocationInformationEUTRA,
			},
		},
		{
			name: "unknown discriminator",
			uli: &ngapType.UserLocationInformation{
				Present: 99,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			msg := validInitialUEMessage()
			for i := range msg.ProtocolIEs.List {
				if msg.ProtocolIEs.List[i].Id.Value == ngapType.ProtocolIEIDUserLocationInformation {
					msg.ProtocolIEs.List[i].Value.UserLocationInformation = tc.uli
				}
			}

			_, report := decode.DecodeInitialUEMessage(msg)
			if report == nil || !report.Fatal() {
				t.Fatalf("expected fatal report; got %+v", report)
			}
		})
	}
}

func TestDecodeInitialUEMessage_WithFiveGSTMSI(t *testing.T) {
	msg := validInitialUEMessage()
	msg.ProtocolIEs.List = append(msg.ProtocolIEs.List, ngapType.InitialUEMessageIEs{
		Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDFiveGSTMSI},
		Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentReject},
		Value: ngapType.InitialUEMessageIEsValue{
			Present:    ngapType.InitialUEMessageIEsPresentFiveGSTMSI,
			FiveGSTMSI: validFiveGSTMSI(),
		},
	})

	out, report := decode.DecodeInitialUEMessage(msg)
	if report != nil {
		t.Fatalf("unexpected report: %+v", report)
	}

	if out.FiveGSTMSI == nil {
		t.Fatal("expected FiveGSTMSI to be populated")
	}

	if !bytes.Equal(out.FiveGSTMSI.FiveGTMSI, []byte{0x01, 0x02, 0x03, 0x04}) {
		t.Errorf("unexpected FiveGTMSI bytes: %x", out.FiveGSTMSI.FiveGTMSI)
	}

	if out.FiveGSTMSI.AMFSetID.BitLength != 10 {
		t.Errorf("AMFSetID BitLength = %d, want 10", out.FiveGSTMSI.AMFSetID.BitLength)
	}

	if out.FiveGSTMSI.AMFPointer.BitLength != 6 {
		t.Errorf("AMFPointer BitLength = %d, want 6", out.FiveGSTMSI.AMFPointer.BitLength)
	}
}

func TestDecodeInitialUEMessage_MalformedFiveGSTMSI(t *testing.T) {
	msg := validInitialUEMessage()
	bad := validFiveGSTMSI()
	bad.AMFSetID.Value.BitLength = 7 // wrong length

	msg.ProtocolIEs.List = append(msg.ProtocolIEs.List, ngapType.InitialUEMessageIEs{
		Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDFiveGSTMSI},
		Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentReject},
		Value: ngapType.InitialUEMessageIEsValue{
			Present:    ngapType.InitialUEMessageIEsPresentFiveGSTMSI,
			FiveGSTMSI: bad,
		},
	})

	out, report := decode.DecodeInitialUEMessage(msg)
	if report == nil || !report.Fatal() {
		t.Fatalf("expected fatal report; got %+v", report)
	}

	if out.FiveGSTMSI != nil {
		t.Errorf("FiveGSTMSI should remain nil on malformed input")
	}
}

func TestDecodeInitialUEMessage_UEContextRequestPresent(t *testing.T) {
	msg := validInitialUEMessage()
	msg.ProtocolIEs.List = append(msg.ProtocolIEs.List, ngapType.InitialUEMessageIEs{
		Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDUEContextRequest},
		Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
		Value: ngapType.InitialUEMessageIEsValue{
			Present:          ngapType.InitialUEMessageIEsPresentUEContextRequest,
			UEContextRequest: &ngapType.UEContextRequest{Value: ngapType.UEContextRequestPresentRequested},
		},
	})

	out, report := decode.DecodeInitialUEMessage(msg)
	if report != nil {
		t.Fatalf("unexpected report: %+v", report)
	}

	if !out.UEContextRequest {
		t.Errorf("UEContextRequest should be true when IE present")
	}
}

func TestDecodeInitialUEMessage_UnknownIEIgnored(t *testing.T) {
	msg := validInitialUEMessage()
	msg.ProtocolIEs.List = append(msg.ProtocolIEs.List, ngapType.InitialUEMessageIEs{
		Id:          ngapType.ProtocolIEID{Value: 9999},
		Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
	})

	_, report := decode.DecodeInitialUEMessage(msg)
	if report != nil {
		t.Fatalf("unknown IE should not produce a report; got %+v", report)
	}
}

func TestDecodeInitialUEMessage_DuplicateIELastWins(t *testing.T) {
	msg := validInitialUEMessage()
	msg.ProtocolIEs.List = append(msg.ProtocolIEs.List, ngapType.InitialUEMessageIEs{
		Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDRANUENGAPID},
		Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentReject},
		Value: ngapType.InitialUEMessageIEsValue{
			Present:     ngapType.InitialUEMessageIEsPresentRANUENGAPID,
			RANUENGAPID: &ngapType.RANUENGAPID{Value: 999},
		},
	})

	out, report := decode.DecodeInitialUEMessage(msg)
	if report != nil {
		t.Fatalf("unexpected report: %+v", report)
	}

	if out.RANUENGAPID != 999 {
		t.Errorf("expected last-wins (999), got %d", out.RANUENGAPID)
	}
}

func TestDecodeReport_ToCriticalityDiagnostics(t *testing.T) {
	r := &decode.DecodeReport{
		ProcedureCode:        ngapType.ProcedureCodeInitialUEMessage,
		TriggeringMessage:    ngapType.TriggeringMessagePresentInitiatingMessage,
		ProcedureCriticality: ngapType.CriticalityPresentIgnore,
	}
	r.MissingMandatory(ngapType.ProtocolIEIDRANUENGAPID, ngapType.CriticalityPresentReject)
	r.Malformed(ngapType.ProtocolIEIDNASPDU, ngapType.CriticalityPresentReject)

	cd := r.ToCriticalityDiagnostics()

	if cd.ProcedureCode == nil || cd.ProcedureCode.Value != ngapType.ProcedureCodeInitialUEMessage {
		t.Errorf("ProcedureCode mismatch")
	}

	if cd.IEsCriticalityDiagnostics == nil || len(cd.IEsCriticalityDiagnostics.List) != 2 {
		t.Fatalf("expected 2 IE diagnostics, got %+v", cd.IEsCriticalityDiagnostics)
	}

	if !r.Fatal() {
		t.Errorf("expected report to be fatal")
	}
}
