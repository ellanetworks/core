// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"bytes"
	"testing"
)

func TestNetworkNameRoundTrip(t *testing.T) {
	for _, name := range []string{"", "Ella", "Ella Networks", "A", "1234567"} {
		enc, err := NewNetworkName(name).MarshalBinary()
		if err != nil {
			t.Fatalf("MarshalBinary(%q): %v", name, err)
		}

		got, err := ParseNetworkName(enc)
		if err != nil {
			t.Fatalf("ParseNetworkName(%q) error: %v", name, err)
		}

		if got.Name != name {
			t.Errorf("round-trip %q = %q", name, got.Name)
		}
	}
}

func TestParseNetworkNameErrors(t *testing.T) {
	if _, err := ParseNetworkName(nil); err == nil {
		t.Error("empty value: want error")
	}

	// Coding scheme 010 and above are reserved (TS 24.008 table 10.5.94).
	if _, err := ParseNetworkName([]byte{0xA0, 0x00}); err == nil {
		t.Error("non-GSM-7-bit coding scheme: want error")
	}
}

// TestParseNetworkNameRejectsImpossibleSpareBits is the regression for a value
// whose spare-bit count exceeds the octets it carries: the character count went
// negative and the decoder panicked on attacker-controlled input.
func TestParseNetworkNameRejectsImpossibleSpareBits(t *testing.T) {
	for _, raw := range [][]byte{{0x07}, {0x01}, {0x03}} {
		if _, err := ParseNetworkName(raw); err == nil {
			t.Errorf("ParseNetworkName(% x) = nil error, want one", raw)
		}
	}
}

// TestNetworkNameUCS2RoundTrip covers coding scheme 001, which TS 24.008
// table 10.5.94 defines alongside the GSM default alphabet and which a name
// outside ASCII needs.
func TestNetworkNameUCS2RoundTrip(t *testing.T) {
	for _, name := range []string{"Ella", "Ellä Nätworks", "日本ネットワーク", ""} {
		n := NewNetworkName(name)

		raw, err := n.MarshalBinary()
		if err != nil {
			t.Fatalf("%q: %v", name, err)
		}

		got, err := ParseNetworkName(raw)
		if err != nil {
			t.Fatalf("%q: %v", name, err)
		}

		if got.Name != name {
			t.Fatalf("%q round-tripped to %q", name, got.Name)
		}

		again, err := got.MarshalBinary()
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(again, raw) {
			t.Fatalf("%q: re-encode % x, want % x", name, again, raw)
		}
	}
}

// TestNetworkNameAddCISurvives pins the Add CI bit (TS 24.008 table 10.5.94,
// octet 3 bit 4), which a receiver that dropped it could not put back.
func TestNetworkNameAddCISurvives(t *testing.T) {
	n := NewNetworkName("Ella")
	n.AddCI = true

	raw, err := n.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	got, err := ParseNetworkName(raw)
	if err != nil {
		t.Fatal(err)
	}

	if !got.AddCI {
		t.Fatalf("Add CI lost: %+v (wire % x)", got, raw)
	}
}
