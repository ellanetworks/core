// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas_test

import (
	"testing"

	"github.com/ellanetworks/core/nas"
)

// TestKeySetIdentifierRoundTrip pins the four-bit coding both generations share
// (TS 24.501 §9.11.3.32, TS 24.301 §9.9.3.21): the value in bits 1-3, the type
// of security context flag in bit 4.
func TestKeySetIdentifierRoundTrip(t *testing.T) {
	for half := range uint8(16) {
		k := nas.ParseKeySetIdentifier(half)
		if got := k.HalfOctet(); got != half {
			t.Errorf("half octet %#x round-tripped to %#x", half, got)
		}

		if want := half&0x08 != 0; k.Mapped != want {
			t.Errorf("half octet %#x: Mapped = %t, want %t", half, k.Mapped, want)
		}
	}

	// The high nibble is another field's, and decoding ignores it.
	if k := nas.ParseKeySetIdentifier(0xF5); k != (nas.KeySetIdentifier{Value: 5}) {
		t.Errorf("ParseKeySetIdentifier(0xf5) = %+v, want native 5", k)
	}
}

func TestKeySetIdentifierAvailable(t *testing.T) {
	if !nas.NoKeySet.Mapped && nas.NoKeySet.Available() {
		t.Error("NoKeySet reports a key set is available")
	}

	if nas.NoKeySet.HalfOctet() != nas.NoKeyAvailable {
		t.Errorf("NoKeySet half octet = %#x, want %#x", nas.NoKeySet.HalfOctet(), nas.NoKeyAvailable)
	}

	if !(nas.KeySetIdentifier{}).Available() {
		t.Error("the zero identifier reports no key set")
	}

	if got := nas.NoKeySet.String(); got != "no key available" {
		t.Errorf("NoKeySet String() = %q", got)
	}

	if got := (nas.KeySetIdentifier{Value: 2, Mapped: true}).String(); got != "mapped 2" {
		t.Errorf("String() = %q, want \"mapped 2\"", got)
	}
}
