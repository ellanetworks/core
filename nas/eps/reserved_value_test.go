// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"bytes"
	"testing"

	"github.com/ellanetworks/core/nas"
)

// TestReservedType1ValueIsAbsentButPreserved pins how a value the spec reserves
// is handled: the element is recognised, so it claims its IEI and reports a soft
// error, the field stays absent (TS 24.301 §7.7.1), and the octets survive.
//
// Claiming matters beyond bookkeeping. A recognised-but-unusable element that
// declined silently would leave a later element with the same IEI free to take
// the field, and which of the two won would then flip once encoding put the
// message in its canonical order.
func TestReservedType1ValueIsAbsentButPreserved(t *testing.T) {
	// A PDN CONNECTIVITY REQUEST carrying the ESM information transfer flag twice:
	// first with the reserved value 0, then with the assigned value 1.
	b := []byte{
		0x32, 0x30, 0xd0, 0x30,
		ieiESMInformationTransferFlag,        // low nibble 0: the reserved value
		ieiESMInformationTransferFlag | 0x01, // assigned, but a repetition
	}

	msg, err := ParsePDNConnectivityRequest(b)
	if err == nil || !nas.SoftOnly(err) {
		t.Fatalf("want a soft error for the reserved value, got %v", err)
	}

	// The first occurrence claimed the IEI, so the second cannot take the field
	// (TS 24.301 §7.6.3).
	if msg.ESMInformationTransferFlag {
		t.Error("a reserved first occurrence must not leave the field to a repetition")
	}

	raw, err := msg.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	// Both occurrences are preserved — the re-encoding carries the reserved
	// value and the repetition, and neither became the field.
	if len(msg.Unrecognized) != 2 {
		t.Fatalf("both occurrences must be preserved, got %+v", msg.Unrecognized)
	}

	// Encoding is a fixed point from here.
	again, err := ParsePDNConnectivityRequest(raw)
	if err != nil && !nas.SoftOnly(err) {
		t.Fatal(err)
	}

	stable, err := again.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(stable, raw) {
		t.Fatalf("encoding is not idempotent:\n first % x\nsecond % x", raw, stable)
	}
}
