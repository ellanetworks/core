// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"bytes"
	"reflect"
	"testing"
)

func TestBearerResourceAllocationRequestRoundTrip(t *testing.T) {
	req := &BearerResourceAllocationRequest{
		EPSBearerIdentity:       0,
		PTI:                     3,
		LinkedEPSBearerIdentity: 5,
		TrafficFlowAggregate:    []byte{0x20, 0x01, 0x01, 0x00},
		RequiredTrafficFlowQoS:  EPSQoS{QCI: 9},
	}

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

	if !reflect.DeepEqual(got, req) {
		t.Fatalf("round trip:\n got %+v\nwant %+v", got, req)
	}

	// The mandatory fields follow the header directly (TS 24.301 table 8.3.8.1):
	// the linked identity and spare half octet, the traffic flow aggregate as an
	// LV, then the required QoS as an LV.
	want := []byte{0x05, 0x04, 0x20, 0x01, 0x01, 0x00, 0x01, 0x09}
	if !bytes.Equal(wire[3:], want) {
		t.Errorf("mandatory part = % x, want % x", wire[3:], want)
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
	qos := EPSQoS{QCI: 7}
	cause := ESMCauseSemanticErrorInTFT
	req := &BearerResourceModificationRequest{
		EPSBearerIdentity:                0,
		PTI:                              7,
		EPSBearerIdentityForPacketFilter: 6,
		TrafficFlowAggregate:             []byte{0x41, 0x01, 0x01, 0x00},
		RequiredTrafficFlowQoS:           &qos,
		Cause:                            &cause,
	}

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

	if !reflect.DeepEqual(got, req) {
		t.Fatalf("round trip:\n got %+v\nwant %+v", got, req)
	}

	// TS 24.301 table 8.3.10.1: the identity and spare half octet, the traffic
	// flow aggregate as an LV, then the optional QoS TLV and ESM cause TV.
	want := []byte{0x06, 0x04, 0x41, 0x01, 0x01, 0x00, 0x5B, 0x01, 0x07, 0x58, 0x29}
	if !bytes.Equal(wire[3:], want) {
		t.Errorf("message body = % x, want % x", wire[3:], want)
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
