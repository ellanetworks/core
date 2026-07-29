// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"bytes"
	"testing"
)

func TestParseUESecurityCapability(t *testing.T) {
	// 5G-EA: EA0+EA2 set (0xA0); 5G-IA: IA1+IA2 set (0x60); then the EPS octets.
	sc, err := ParseUESecurityCapability([]byte{0xA0, 0x60, 0xFF, 0x80})
	if err != nil {
		t.Fatalf("ParseUESecurityCapability: %v", err)
	}

	if !sc.SupportsEA(0) || sc.SupportsEA(1) || !sc.SupportsEA(2) || sc.SupportsEA(3) {
		t.Errorf("EA support wrong: EA=%#x", sc.EA)
	}

	if sc.SupportsIA(0) || !sc.SupportsIA(1) || !sc.SupportsIA(2) || sc.SupportsIA(3) {
		t.Errorf("IA support wrong: IA=%#x", sc.IA)
	}

	if !sc.HasEPS || !sc.SupportsEEA(7) || !sc.SupportsEIA(0) || sc.SupportsEIA(1) {
		t.Errorf("EPS support wrong: EEA=%#x EIA=%#x", sc.EEA, sc.EIA)
	}

	if sc.SupportsEA(8) || sc.SupportsIA(9) {
		t.Errorf("out-of-range n must be false")
	}
}

func TestParseUESecurityCapabilityBadLength(t *testing.T) {
	for _, raw := range [][]byte{{}, {0x80}, {0x80, 0x00, 0xff}} {
		if _, err := ParseUESecurityCapability(raw); err == nil {
			t.Errorf("ParseUESecurityCapability(% x) = nil error, want one", raw)
		}
	}
}

// TestUESecurityCapabilityRoundTrip confirms each spec-permitted length decodes
// and re-encodes byte-for-byte, which the bidding-down replay depends on
// (TS 24.501 §9.11.3.54).
func TestUESecurityCapabilityRoundTrip(t *testing.T) {
	for _, raw := range [][]byte{
		{0xe0, 0xe0},
		{0xe0, 0xe0, 0xc0, 0xc0},
		{0xe0, 0xe0, 0xc0, 0xc0, 0x00, 0x00, 0x00, 0x00},
	} {
		c, err := ParseUESecurityCapability(raw)
		if err != nil {
			t.Fatalf("Parse(% x): %v", raw, err)
		}

		got, err := c.MarshalBinary()
		if err != nil {
			t.Fatalf("MarshalBinary(% x): %v", raw, err)
		}

		if !bytes.Equal(got, raw) {
			t.Errorf("round-trip % x -> % x", raw, got)
		}
	}
}
