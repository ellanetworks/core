// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import "testing"

func TestBearerResourceAllocationRequestRoundTrip(t *testing.T) {
	req := &BearerResourceAllocationRequest{EPSBearerIdentity: 0, ProcedureTransactionIdentity: 3}

	wire, err := req.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// ESM header: EBI<<4|PD(ESM=2), PTI, message type 0xD4.
	if wire[0] != (0<<4|0x02) || wire[1] != 3 || wire[2] != byte(MsgBearerResourceAllocationRequest) {
		t.Fatalf("ESM header = % x, want %x %x %x", wire[:3], 0<<4|0x02, 3, byte(MsgBearerResourceAllocationRequest))
	}

	got, err := ParseBearerResourceAllocationRequest(wire)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if got.EPSBearerIdentity != req.EPSBearerIdentity || got.ProcedureTransactionIdentity != req.ProcedureTransactionIdentity {
		t.Fatalf("round-trip mismatch: got EBI=%d PTI=%d", got.EPSBearerIdentity, got.ProcedureTransactionIdentity)
	}
}

func TestBearerResourceAllocationRejectRoundTrip(t *testing.T) {
	rej := &BearerResourceAllocationReject{ProcedureTransactionIdentity: 3, ESMCause: 31}

	wire, err := rej.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	if wire[2] != byte(MsgBearerResourceAllocationReject) || wire[3] != 31 {
		t.Fatalf("wire = % x, want message type %#x and ESM cause 31", wire, byte(MsgBearerResourceAllocationReject))
	}

	got, err := ParseBearerResourceAllocationReject(wire)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if got.ProcedureTransactionIdentity != rej.ProcedureTransactionIdentity || got.ESMCause != rej.ESMCause {
		t.Fatalf("round-trip mismatch: got PTI=%d cause=%d", got.ProcedureTransactionIdentity, got.ESMCause)
	}
}

func TestBearerResourceModificationRequestRoundTrip(t *testing.T) {
	req := &BearerResourceModificationRequest{EPSBearerIdentity: 0, ProcedureTransactionIdentity: 7}

	wire, err := req.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	if wire[0] != (0<<4|0x02) || wire[1] != 7 || wire[2] != byte(MsgBearerResourceModificationRequest) {
		t.Fatalf("ESM header = % x, want %x %x %x", wire[:3], 0<<4|0x02, 7, byte(MsgBearerResourceModificationRequest))
	}

	got, err := ParseBearerResourceModificationRequest(wire)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if got.EPSBearerIdentity != req.EPSBearerIdentity || got.ProcedureTransactionIdentity != req.ProcedureTransactionIdentity {
		t.Fatalf("round-trip mismatch: got EBI=%d PTI=%d", got.EPSBearerIdentity, got.ProcedureTransactionIdentity)
	}
}

func TestBearerResourceModificationRejectRoundTrip(t *testing.T) {
	rej := &BearerResourceModificationReject{ProcedureTransactionIdentity: 7, ESMCause: 31}

	wire, err := rej.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	if wire[2] != byte(MsgBearerResourceModificationReject) || wire[3] != 31 {
		t.Fatalf("wire = % x, want message type %#x and ESM cause 31", wire, byte(MsgBearerResourceModificationReject))
	}

	got, err := ParseBearerResourceModificationReject(wire)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if got.ProcedureTransactionIdentity != rej.ProcedureTransactionIdentity || got.ESMCause != rej.ESMCause {
		t.Fatalf("round-trip mismatch: got PTI=%d cause=%d", got.ProcedureTransactionIdentity, got.ESMCause)
	}
}
