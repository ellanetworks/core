// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package models_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/models"
)

func TestNormalizeSD(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"already canonical", "0a0b0c", "0a0b0c"},
		{"uppercase", "0A0B0C", "0a0b0c"},
		{"mixed case", "0a0B0c", "0a0b0c"},
		{"short, right-padded", "0a0b", "0a0b00"},
		{"single octet", "01", "010000"},
		{"not hex kept as-is", "zzz", "zzz"},
		{"odd length kept as-is", "0a0", "0a0"},
		{"too long kept as-is", "0a0b0c0d", "0a0b0c0d"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := models.NormalizeSD(tc.in); got != tc.want {
				t.Errorf("NormalizeSD(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// A slice provisioned as "0A0B0C" or "0a0b" must match the "0a0b0c" NAS and NGAP
// decoding produce, or every session on it is rejected with "no matching policy".
func TestSnssaiEqualIgnoresSDSpelling(t *testing.T) {
	fromWire := models.Snssai{Sst: 1, Sd: "0a0b0c"}

	for _, provisioned := range []string{"0a0b0c", "0A0B0C", "0a0B0c"} {
		if !(models.Snssai{Sst: 1, Sd: provisioned}).Equal(fromWire) {
			t.Errorf("provisioned SD %q did not match wire SD %q", provisioned, fromWire.Sd)
		}
	}

	if (models.Snssai{Sst: 1, Sd: "0a0b0d"}).Equal(fromWire) {
		t.Error("a genuinely different SD matched")
	}

	if (models.Snssai{Sst: 2, Sd: "0a0b0c"}).Equal(fromWire) {
		t.Error("a different SST matched")
	}
}
