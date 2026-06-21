// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"bytes"
	"testing"
)

func TestModifyEPSBearerContextRequestRoundTrip(t *testing.T) {
	pco := BuildProtocolConfigurationOptions([][]byte{{1, 1, 1, 1}}, 1500)

	req := &ModifyEPSBearerContextRequest{
		EPSBearerIdentity:            5,
		ProcedureTransactionIdentity: 0,
		ProtocolConfigurationOptions: pco,
	}

	wire, err := req.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// ESM header: EBI<<4|PD(ESM=2), PTI, message type 0xC9.
	if wire[0] != (5<<4|0x02) || wire[1] != 0 || wire[2] != byte(MsgModifyEPSBearerContextRequest) {
		t.Fatalf("ESM header = % x, want first three bytes %x %x %x", wire[:3], 5<<4|0x02, 0, byte(MsgModifyEPSBearerContextRequest))
	}

	if wire[3] != ieiProtocolConfigurationOptions {
		t.Fatalf("PCO IEI = %#x, want %#x", wire[3], ieiProtocolConfigurationOptions)
	}

	got, err := ParseModifyEPSBearerContextRequest(wire)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if got.EPSBearerIdentity != req.EPSBearerIdentity || got.ProcedureTransactionIdentity != req.ProcedureTransactionIdentity {
		t.Fatalf("header round-trip mismatch: got EBI=%d PTI=%d", got.EPSBearerIdentity, got.ProcedureTransactionIdentity)
	}

	if !bytes.Equal(got.ProtocolConfigurationOptions, pco) {
		t.Fatalf("PCO = % x, want % x", got.ProtocolConfigurationOptions, pco)
	}
}

func TestModifyEPSBearerContextRequestAPNAMBRRoundTrip(t *testing.T) {
	const dlBps, ulBps = 200 * 1_000_000, 100 * 1_000_000

	apnambr := EncodeAPNAMBR(dlBps, ulBps).Marshal()
	pco := BuildProtocolConfigurationOptions([][]byte{{1, 1, 1, 1}}, 1500)

	req := &ModifyEPSBearerContextRequest{
		EPSBearerIdentity:            5,
		ProcedureTransactionIdentity: 0,
		APNAMBR:                      apnambr,
		ProtocolConfigurationOptions: pco,
	}

	wire, err := req.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// TS 24.301 §8.3.18.1: APN-AMBR precedes PCO in message order.
	if wire[3] != ieiAPNAMBR {
		t.Fatalf("first optional IEI = %#x, want APN-AMBR %#x", wire[3], ieiAPNAMBR)
	}

	apnLen := int(wire[4])
	if pcoOff := 5 + apnLen; pcoOff >= len(wire) || wire[pcoOff] != ieiProtocolConfigurationOptions {
		t.Fatalf("PCO IEI not found after APN-AMBR at offset %d (wire % x)", pcoOff, wire)
	}

	got, err := ParseModifyEPSBearerContextRequest(wire)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if !bytes.Equal(got.APNAMBR, apnambr) {
		t.Fatalf("APN-AMBR = % x, want % x", got.APNAMBR, apnambr)
	}

	if !bytes.Equal(got.ProtocolConfigurationOptions, pco) {
		t.Fatalf("PCO = % x, want % x", got.ProtocolConfigurationOptions, pco)
	}

	ambr, err := ParseAPNAMBR(got.APNAMBR)
	if err != nil {
		t.Fatalf("parse APN-AMBR: %v", err)
	}

	if dl, ul := ambr.BitsPerSecond(); dl != dlBps || ul != ulBps {
		t.Fatalf("APN-AMBR = %d/%d bps, want %d/%d", dl, ul, dlBps, ulBps)
	}
}

func TestModifyEPSBearerContextRequestNewEPSQoSRoundTrip(t *testing.T) {
	epsQoS := EPSQoS{QCI: 7}.Marshal()
	apnambr := EncodeAPNAMBR(200*1_000_000, 100*1_000_000).Marshal()

	req := &ModifyEPSBearerContextRequest{
		EPSBearerIdentity: 5,
		NewEPSQoS:         epsQoS,
		APNAMBR:           apnambr,
	}

	wire, err := req.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// TS 24.301 §8.3.18.1: New EPS QoS precedes APN-AMBR.
	if wire[3] != ieiNewEPSQoS {
		t.Fatalf("first optional IEI = %#x, want New EPS QoS %#x", wire[3], ieiNewEPSQoS)
	}

	qosLen := int(wire[4])
	if ambrOff := 5 + qosLen; ambrOff >= len(wire) || wire[ambrOff] != ieiAPNAMBR {
		t.Fatalf("APN-AMBR IEI not found after New EPS QoS at offset %d (wire % x)", ambrOff, wire)
	}

	got, err := ParseModifyEPSBearerContextRequest(wire)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if !bytes.Equal(got.NewEPSQoS, epsQoS) {
		t.Fatalf("New EPS QoS = % x, want % x", got.NewEPSQoS, epsQoS)
	}

	if !bytes.Equal(got.APNAMBR, apnambr) {
		t.Fatalf("APN-AMBR = % x, want % x", got.APNAMBR, apnambr)
	}
}

func TestModifyEPSBearerContextAcceptRoundTrip(t *testing.T) {
	acc := &ModifyEPSBearerContextAccept{EPSBearerIdentity: 5, ProcedureTransactionIdentity: 0}

	wire, err := acc.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	if wire[2] != byte(MsgModifyEPSBearerContextAccept) {
		t.Fatalf("message type = %#x, want %#x", wire[2], byte(MsgModifyEPSBearerContextAccept))
	}

	got, err := ParseModifyEPSBearerContextAccept(wire)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if got.EPSBearerIdentity != acc.EPSBearerIdentity || got.ProcedureTransactionIdentity != acc.ProcedureTransactionIdentity {
		t.Fatalf("round-trip mismatch: got EBI=%d PTI=%d", got.EPSBearerIdentity, got.ProcedureTransactionIdentity)
	}
}
