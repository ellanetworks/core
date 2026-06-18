// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import "testing"

func TestBitRateToBps(t *testing.T) {
	cases := []struct {
		in   string
		want uint64
	}{
		{"1000 bps", 1000},
		{"500 Kbps", 500_000},
		{"100 Mbps", 100_000_000},
		{"1 Gbps", 1_000_000_000},
		{"2 Tbps", 2_000_000_000_000},
		{"", 0},
		{"garbage", 0},  // no unit
		{"5", 0},        // missing unit token
		{"abc Mbps", 0}, // non-numeric value
		{"10 Xbps", 0},  // unknown unit
		{"1 Gbps x", 0}, // too many tokens
	}

	for _, c := range cases {
		if got := bitRateToBps(c.in); got != c.want {
			t.Errorf("bitRateToBps(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}
