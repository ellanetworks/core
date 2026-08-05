// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package models_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/models"
)

func TestParseBitRate(t *testing.T) {
	for _, c := range []struct {
		in   string
		want uint64
	}{
		{"1 bps", 1},
		{"1 Kbps", 1_000},
		{"200 Mbps", 200_000_000},
		{"1 Gbps", 1_000_000_000},
		{"1.5 Gbps", 1_500_000_000},
		{"4 Tbps", 4_000_000_000_000},
		{"  1 Gbps", 1_000_000_000},
		{"0 bps", 0},
	} {
		got, err := models.ParseBitRate(c.in)
		if err != nil {
			t.Errorf("%q: %v", c.in, err)

			continue
		}

		if got.Bps() != c.want {
			t.Errorf("%q = %d bps, want %d", c.in, got.Bps(), c.want)
		}
	}
}

// free5gc's converter returned 0 for a malformed rate and treated an unknown
// unit as bps. Both put a wrong rate on the wire with no signal.
func TestParseBitRateRejectsMalformed(t *testing.T) {
	for _, in := range []string{
		"",
		"1Gbps",
		"1 Xbps",
		"Gbps",
		"abc Gbps",
		"-1 Gbps",
		"NaN Gbps",
		"Inf Gbps",
	} {
		if got, err := models.ParseBitRate(in); err == nil {
			t.Errorf("%q parsed as %d bps, want an error", in, got.Bps())
		}
	}
}
