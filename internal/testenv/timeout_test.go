// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package testenv

import "testing"

func TestParseMultiplier(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		ci   bool
		want int64
	}{
		{"", false, 1},
		{"", true, ciMultiplier},
		{"5", false, 5},
		{"5", true, 5},
		{"0", true, ciMultiplier},
		{"-2", false, 1},
		{"abc", false, 1},
	} {
		if got := parseMultiplier(tc.raw, tc.ci); got != tc.want {
			t.Errorf("parseMultiplier(%q, %v) = %d, want %d", tc.raw, tc.ci, got, tc.want)
		}
	}
}
