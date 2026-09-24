// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import "testing"

func TestNextNgKsi(t *testing.T) {
	tests := []struct {
		name    string
		current int32
		want    int32
	}{
		{"0 -> 1", 0, 1},
		{"3 -> 4", 3, 4},
		{"5 -> 6", 5, 6},
		{"6 wraps to 0", 6, 0},
		{"7 (no key) wraps to 0", 7, 0},
		{"8 (out of range) wraps to 0", 8, 0},
		{"negative wraps to 0", -1, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NextNgKsi(tt.current)
			if got != tt.want {
				t.Errorf("NextNgKsi(%d) = %d, want %d", tt.current, got, tt.want)
			}
		})
	}
}

func TestSelectNgKsiDiffersFromTheCitedAndTheStoredNgKsi(t *testing.T) {
	for _, tc := range []struct{ cited, stored, want int32 }{
		{cited: 1, stored: 7, want: 2},
		{cited: 1, stored: 2, want: 3},
		{cited: 6, stored: 0, want: 1},
		{cited: 7, stored: 0, want: 1},
		{cited: 7, stored: 7, want: 0},
	} {
		if got := SelectNgKsi(tc.cited, tc.stored); got != tc.want {
			t.Errorf("SelectNgKsi(%d, %d) = %d, want %d", tc.cited, tc.stored, got, tc.want)
		}
	}
}

func TestSelectNgKsiAfterNgKSIAlreadyInUseAvoidsTheCitedAndStoredNgKsi(t *testing.T) {
	if got := SelectNgKsi(3, 2, 4); got == 2 || got == 3 || got == 4 {
		t.Fatalf("SelectNgKsi(rejected 3, cited 2, stored 4) = %d, want a value other than all three (TS 24.501 §5.4.1.3.2, §5.4.1.3.4)", got)
	}
}
