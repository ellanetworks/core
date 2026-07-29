// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"reflect"
	"testing"

	"github.com/ellanetworks/core/nas"
)

func TestModifyEPSBearerContextRequestRoundTrip(t *testing.T) {
	pco := nas.NewProtocolConfigurationOptions([][]byte{{1, 1, 1, 1}}, 1500)
	pcoPtr := &pco

	req := &ModifyEPSBearerContextRequest{
		EPSBearerIdentity:            5,
		PTI:                          0,
		ProtocolConfigurationOptions: pcoPtr,
	}

	wire, err := req.MarshalBinary()
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

	if got.EPSBearerIdentity != req.EPSBearerIdentity || got.PTI != req.PTI {
		t.Fatalf("header round-trip mismatch: got EBI=%d PTI=%d", got.EPSBearerIdentity, got.PTI)
	}

	if !reflect.DeepEqual(got.ProtocolConfigurationOptions, pcoPtr) {
		t.Fatalf("PCO = % x, want % x", got.ProtocolConfigurationOptions, pco)
	}
}

func TestModifyEPSBearerContextRequestAPNAMBRRoundTrip(t *testing.T) {
	const dlKbps, ulKbps = 200 * 1_000, 100 * 1_000

	apnambr, err := APNAMBRFromKbps(dlKbps, ulKbps)
	if err != nil {
		t.Fatal(err)
	}

	pco := nas.NewProtocolConfigurationOptions([][]byte{{1, 1, 1, 1}}, 1500)
	pcoPtr := &pco

	req := &ModifyEPSBearerContextRequest{
		EPSBearerIdentity:            5,
		PTI:                          0,
		APNAMBR:                      &apnambr,
		ProtocolConfigurationOptions: pcoPtr,
	}

	wire, err := req.MarshalBinary()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// TS 24.301: APN-AMBR precedes PCO in message order.
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

	if got.APNAMBR == nil || !reflect.DeepEqual(*got.APNAMBR, apnambr) {
		t.Fatalf("APN-AMBR = %+v, want %+v", got.APNAMBR, apnambr)
	}

	if !reflect.DeepEqual(got.ProtocolConfigurationOptions, pcoPtr) {
		t.Fatalf("PCO = % x, want % x", got.ProtocolConfigurationOptions, pco)
	}

	ambr := *got.APNAMBR

	if dl, ul, ok := ambr.Kbps(); !ok || dl != dlKbps || ul != ulKbps {
		t.Fatalf("APN-AMBR = %d/%d kbit/s, want %d/%d", dl, ul, dlKbps, ulKbps)
	}
}

func TestModifyEPSBearerContextRequestNewEPSQoSRoundTrip(t *testing.T) {
	epsQoS := EPSQoS{QCI: 7}

	apnambr, err := APNAMBRFromKbps(200*1_000, 100*1_000)
	if err != nil {
		t.Fatal(err)
	}

	req := &ModifyEPSBearerContextRequest{
		EPSBearerIdentity: 5,
		NewEPSQoS:         &epsQoS,
		APNAMBR:           &apnambr,
	}

	wire, err := req.MarshalBinary()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// TS 24.301: New EPS QoS precedes APN-AMBR.
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

	if got.NewEPSQoS == nil || !reflect.DeepEqual(*got.NewEPSQoS, epsQoS) {
		t.Fatalf("New EPS QoS = %+v, want %+v", got.NewEPSQoS, epsQoS)
	}

	if got.APNAMBR == nil || !reflect.DeepEqual(*got.APNAMBR, apnambr) {
		t.Fatalf("APN-AMBR = %+v, want %+v", got.APNAMBR, apnambr)
	}
}

func TestModifyEPSBearerContextAcceptRoundTrip(t *testing.T) {
	acc := &ModifyEPSBearerContextAccept{EPSBearerIdentity: 5, PTI: 0}

	wire, err := acc.MarshalBinary()
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

	if got.EPSBearerIdentity != acc.EPSBearerIdentity || got.PTI != acc.PTI {
		t.Fatalf("round-trip mismatch: got EBI=%d PTI=%d", got.EPSBearerIdentity, got.PTI)
	}
}
