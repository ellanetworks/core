// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"testing"

	"github.com/ellanetworks/core/nas"
)

func TestSelectAlgorithm(t *testing.T) {
	supportsAll := func(uint8) bool { return true }
	supportsAESOnly := func(n uint8) bool { return n == 2 }
	supportsNullOnly := func(n uint8) bool { return n == 0 }

	// Preferences are EPS algorithm codes (NULL=0, SNOW3G=1, AES=2).
	tests := []struct {
		name       string
		preference []uint8
		supported  func(uint8) bool
		want       byte
		wantOK     bool
	}{
		{"AES preferred", []uint8{2, 1}, supportsAll, 2, true},
		{"SNOW3G preferred", []uint8{1, 2}, supportsAll, 1, true},
		{"SNOW3G preferred but UE lacks it", []uint8{1, 2}, supportsAESOnly, 2, true},
		{"no common algorithm", []uint8{1}, supportsAESOnly, 0, false},
		{"NULL configured and UE advertises it", []uint8{2, 0}, supportsNullOnly, 0, true},
		{"NULL configured but UE does not advertise it", []uint8{2, 0}, supportsAESOnly, 2, true},
		{"NULL configured, UE supports nothing", []uint8{0}, supportsAESOnly, 0, false},
		{"empty preference", nil, supportsAll, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := selectAlgorithm(tt.preference, tt.supported)
			if got != tt.want || ok != tt.wantOK {
				t.Fatalf("selectAlgorithm = (%d, %v), want (%d, %v)", got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

// The EPS algorithms are read out of the UE network capability's own EEA and EIA
// octets, never out of the UMTS or GERAN ones sitting beside them.
func TestSelectNASAlgorithms(t *testing.T) {
	// EEA: EEA0 and 128-EEA2. EIA: 128-EIA2 only.
	uecap := UENetworkCapability{EEA: 0b1010_0000, EIA: 0b0010_0000}

	order := []nas.CipheringAlgorithm{nas.CipheringSNOW3G, nas.CipheringAES, nas.CipheringNull}
	intOrder := []nas.IntegrityAlgorithm{nas.IntegritySNOW3G, nas.IntegrityAES}

	eea, eia, ok := SelectNASAlgorithms(uecap, intOrder, order)
	if !ok {
		t.Fatal("a UE and operator sharing 128-EEA2 and 128-EIA2 must negotiate")
	}

	if eea != nas.CipheringAES || eia != nas.IntegrityAES {
		t.Fatalf("SelectNASAlgorithms = (%s, %s), want (128-EEA2, 128-EIA2)", eea, eia)
	}

	// No integrity algorithm in common: the pair is rejected rather than falling
	// back to EIA0.
	if _, _, ok := SelectNASAlgorithms(uecap, []nas.IntegrityAlgorithm{nas.IntegritySNOW3G}, order); ok {
		t.Fatal("a UE with no operator-preferred integrity algorithm must not negotiate")
	}
}
