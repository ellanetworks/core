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
