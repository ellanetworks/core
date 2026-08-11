// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import "testing"

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
			got, ok := SelectAlgorithm(tt.preference, tt.supported)
			if got != tt.want || ok != tt.wantOK {
				t.Fatalf("SelectAlgorithm = (%d, %v), want (%d, %v)", got, ok, tt.want, tt.wantOK)
			}
		})
	}
}
