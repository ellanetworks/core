// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas_test

import (
	"reflect"
	"testing"

	"github.com/ellanetworks/core/nas"
)

// TestAlgorithmSetBitOrder pins the coding of TS 24.501 §9.11.3.54 and
// TS 24.301 §9.9.3.34: algorithm n occupies bit 8-n, so algorithm 0 is the most
// significant bit.
func TestAlgorithmSetBitOrder(t *testing.T) {
	for n, want := range map[uint8]nas.AlgorithmSet{0: 0x80, 1: 0x40, 2: 0x20, 7: 0x01} {
		if got := nas.Algorithms(n); got != want {
			t.Errorf("Algorithms(%d) = %#02x, want %#02x", n, uint8(got), uint8(want))
		}
	}

	s := nas.Algorithms(0, 2, 7)
	if s != 0xA1 {
		t.Fatalf("Algorithms(0,2,7) = %#02x, want 0xa1", uint8(s))
	}

	for n := range uint8(8) {
		want := n == 0 || n == 2 || n == 7
		if got := s.Supports(n); got != want {
			t.Errorf("Supports(%d) = %t, want %t", n, got, want)
		}
	}

	if !reflect.DeepEqual(s.Identities(), []uint8{0, 2, 7}) {
		t.Errorf("Identities() = %v, want [0 2 7]", s.Identities())
	}

	if s.String() != "0,2,7" {
		t.Errorf("String() = %q, want \"0,2,7\"", s.String())
	}

	if nas.AlgorithmSet(0).String() != "none" {
		t.Errorf("empty String() = %q, want \"none\"", nas.AlgorithmSet(0).String())
	}
}

// TestAlgorithmSetIgnoresUncodedIdentities checks that an identity with no bit in
// the octet neither sets one nor reads as supported.
func TestAlgorithmSetIgnoresUncodedIdentities(t *testing.T) {
	if s := nas.Algorithms(8, 255); s != 0 {
		t.Errorf("Algorithms(8,255) = %#02x, want 0", uint8(s))
	}

	if nas.AlgorithmSet(0xFF).Supports(8) {
		t.Error("Supports(8) is true for a full octet")
	}
}
