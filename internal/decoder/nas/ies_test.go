// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"encoding/hex"
	"slices"
	"testing"
)

// TS 24.301 §9.9.3.36 packs each algorithm set MSB-first, and bit 8 of the UIA
// and GEA octets is spare. Bytes observed on a live 001/01 network.
func TestUESecurityCapabilityBitLayout(t *testing.T) {
	raw, err := hex.DecodeString("f0f0c040")
	if err != nil {
		t.Fatal(err)
	}

	got := UESecurityCapabilityFromBytes(raw)

	if got.Error != "" {
		t.Fatalf("decode error: %s", got.Error)
	}

	if got.Hex != "f0f0c040" {
		t.Errorf("hex = %q, want the bytes as received", got.Hex)
	}

	for _, tc := range []struct {
		name string
		got  []string
		want []string
	}{
		{"eea", got.EEA, []string{"EEA0", "EEA1", "EEA2", "EEA3"}},
		{"eia", got.EIA, []string{"EIA0", "EIA1", "EIA2", "EIA3"}},
		{"uea", got.UEA, []string{"UEA0", "UEA1"}},
		{"uia", got.UIA, []string{"UIA1"}},
		{"gea absent on a four octet element", got.GEA, nil},
	} {
		if !slices.Equal(tc.got, tc.want) {
			t.Errorf("%s = %v, want %v", tc.name, tc.got, tc.want)
		}
	}
}

func TestUESecurityCapabilityKeepsBytesOnBadLength(t *testing.T) {
	got := UESecurityCapabilityFromBytes([]byte{0x01, 0x02, 0x03})

	if got.Error == "" {
		t.Error("expected an error for a three octet element")
	}

	if got.Hex != "010203" {
		t.Errorf("hex = %q, want the bytes preserved for the bidding-down check", got.Hex)
	}
}
