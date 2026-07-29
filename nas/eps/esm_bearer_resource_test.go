// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import "testing"

func TestBearerResourceAllocationRequestRoundTrip(t *testing.T) {
	req := &BearerResourceAllocationRequest{EPSBearerIdentity: 0, PTI: 3}

	wire, err := req.MarshalBinary()
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

	if got.EPSBearerIdentity != req.EPSBearerIdentity || got.PTI != req.PTI {
		t.Fatalf("round-trip mismatch: got EBI=%d PTI=%d", got.EPSBearerIdentity, got.PTI)
	}
}

func TestBearerResourceAllocationRejectRoundTrip(t *testing.T) {
	rej := &BearerResourceAllocationReject{PTI: 3, Cause: 31}

	wire, err := rej.MarshalBinary()
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

	if got.PTI != rej.PTI || got.Cause != rej.Cause {
		t.Fatalf("round-trip mismatch: got PTI=%d cause=%d", got.PTI, got.Cause)
	}
}

func TestBearerResourceModificationRequestRoundTrip(t *testing.T) {
	req := &BearerResourceModificationRequest{EPSBearerIdentity: 0, PTI: 7}

	wire, err := req.MarshalBinary()
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

	if got.EPSBearerIdentity != req.EPSBearerIdentity || got.PTI != req.PTI {
		t.Fatalf("round-trip mismatch: got EBI=%d PTI=%d", got.EPSBearerIdentity, got.PTI)
	}
}

func TestBearerResourceModificationRejectRoundTrip(t *testing.T) {
	rej := &BearerResourceModificationReject{PTI: 7, Cause: 31}

	wire, err := rej.MarshalBinary()
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

	if got.PTI != rej.PTI || got.Cause != rej.Cause {
		t.Fatalf("round-trip mismatch: got PTI=%d cause=%d", got.PTI, got.Cause)
	}
}
