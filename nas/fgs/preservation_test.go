// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"bytes"
	"testing"
)

// TestServiceAcceptPreservesUnrecognized confirms an element the message does
// not model survives a decode/encode round trip instead of being dropped
// (TS 24.501 §7.6.1: ignore what you do not comprehend, without losing it).
func TestServiceAcceptPreservesUnrecognized(t *testing.T) {
	wire := []byte{
		uint8(EPD5GMM), 0x00, uint8(MsgServiceAccept),
		ieiPDUSessionStatus, 0x02, 0x00, 0x20, // modelled
		0x31, 0x02, 0xaa, 0xbb, // a TLV from a later release
	}

	msg, err := ParseServiceAccept(wire)
	if err != nil {
		t.Fatalf("ParseServiceAccept: %v", err)
	}

	if len(msg.Unrecognized) != 1 || msg.Unrecognized[0].IEI != 0x31 {
		t.Fatalf("Unrecognized = %+v, want the 0x31 element", msg.Unrecognized)
	}

	got, err := msg.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}

	if !bytes.Equal(got, wire) {
		t.Fatalf("round-trip = %#x, want %#x", got, wire)
	}
}
