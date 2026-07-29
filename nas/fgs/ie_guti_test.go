// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import "testing"

// TestAMFIdentifierRoundTrip confirms the 3-octet AMF identifier round-trips,
// including the 10-bit Set ID / 6-bit Pointer split (TS 23.003 §2.10.1).
func TestAMFIdentifierRoundTrip(t *testing.T) {
	for _, in := range []AMFIdentifier{
		{},
		{RegionID: 0xCA, SetID: 0x3FF, Pointer: 0x3F},
		{RegionID: 0x01, SetID: 1, Pointer: 0},
	} {
		raw, err := in.MarshalBinary()
		if err != nil {
			t.Fatalf("MarshalBinary(%+v): %v", in, err)
		}

		got, err := ParseAMFIdentifier(raw)
		if err != nil {
			t.Fatalf("Parse(% x): %v", raw, err)
		}

		if got != in {
			t.Errorf("round-trip %+v -> %+v", in, got)
		}
	}

	if _, err := ParseAMFIdentifier([]byte{0x01, 0x02}); err == nil {
		t.Error("short value: want error")
	}
}
