// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"bytes"
	"testing"

	"github.com/ellanetworks/core/nas"
)

// TestUnusableElementIsAbsentButPreserved pins how an element the receiver
// recognises but cannot use is handled: it claims its IEI and reports a soft
// error, the field stays absent (TS 24.301 §7.7.1), and the octets survive.
//
// Claiming matters beyond bookkeeping. A recognised-but-unusable element that
// declined silently would leave a later element with the same IEI free to take
// the field, and which of the two won would then flip once encoding put the
// message in its canonical order.
func TestUnusableElementIsAbsentButPreserved(t *testing.T) {
	// A PDN CONNECTIVITY REQUEST carrying the access point name twice: first with
	// a value no receiver accepts — a single empty label, which the dotted form
	// cannot represent (TS 24.008 §10.5.6.1) — then with a well-formed one.
	b := []byte{
		0x32, 0x30, 0xd0, 0x30,
		ieiAccessPointName, 0x01, 0x00, // one empty label: unusable
		ieiAccessPointName, 0x04, 0x03, 'a', 'b', 'c', // well-formed, but a repetition
	}

	msg, err := ParsePDNConnectivityRequest(b)
	if err == nil || !nas.SoftOnly(err) {
		t.Fatalf("want a soft error for the unusable value, got %v", err)
	}

	// The first occurrence claimed the IEI, so the second cannot take the field
	// (TS 24.301 §7.6.3).
	if msg.AccessPointName != nil {
		t.Errorf("an unusable first occurrence left the field to a repetition: %v", *msg.AccessPointName)
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
