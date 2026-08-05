// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package models_test

import (
	"encoding/json"
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

// An operator reads back the rate they entered. "1000 Mbps" and "1 Gbps" are
// the same rate, and rewriting one into the other would change API responses
// for no benefit.
func TestBitRatePreservesConfiguredText(t *testing.T) {
	for _, in := range []string{"200 Mbps", "1000 Mbps", "1000 Kbps", "2000 Kbps", "1 Gbps", "4 Tbps", "1.5 Gbps", "1500 bps"} {
		parsed := models.MustParseBitRate(in)
		if got := parsed.String(); got != in {
			t.Errorf("%q read back as %q", in, got)
		}

		again, err := models.ParseBitRate(parsed.String())
		if err != nil || !again.Equal(parsed) {
			t.Errorf("%q did not survive a round trip: %q -> %d bps (%v)", in, parsed.String(), again.Bps(), err)
		}
	}
}

// A computed rate has no configured text, so it renders in the widest unit that
// divides it evenly. Zero divides by every unit and must not widen to "0 Tbps".
func TestBitRateFromBpsRendersCanonically(t *testing.T) {
	for _, c := range []struct {
		bps  uint64
		want string
	}{
		{0, "0 bps"},
		{1500, "1500 bps"},
		{200_000_000, "200 Mbps"},
		{1_000_000_000, "1 Gbps"},
	} {
		if got := models.BitRateFromBps(c.bps).String(); got != c.want {
			t.Errorf("%d bps rendered %q, want %q", c.bps, got, c.want)
		}
	}
}

// Equal compares rates; == would also compare the unit they were written in.
func TestBitRateEqualIgnoresUnit(t *testing.T) {
	a := models.MustParseBitRate("1000 Mbps")
	b := models.MustParseBitRate("1 Gbps")

	if !a.Equal(b) {
		t.Fatalf("%s and %s are the same rate", a, b)
	}

	if a == b {
		t.Error("== compared equal; it must not, or this test proves nothing about Equal")
	}
}

func TestBitRateJSONKeepsConfiguredText(t *testing.T) {
	type payload struct {
		Rate models.BitRate `json:"rate"`
	}

	var p payload
	if err := json.Unmarshal([]byte(`{"rate":"1000 Mbps"}`), &p); err != nil {
		t.Fatal(err)
	}

	if p.Rate.Bps() != 1_000_000_000 {
		t.Fatalf("decoded %d bps, want 1000000000", p.Rate.Bps())
	}

	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}

	if string(b) != `{"rate":"1000 Mbps"}` {
		t.Fatalf("re-encoded %s, want the configured text back", b)
	}

	if err := json.Unmarshal([]byte(`{"rate":"1 Xbps"}`), &p); err == nil {
		t.Error("decoded an unknown unit, want an error")
	}
}
