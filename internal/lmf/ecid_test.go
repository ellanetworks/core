// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lmf

import (
	"math"
	"testing"

	"github.com/ellanetworks/core/internal/lmf/models"
)

// TestNrTAToDistance checks the NR-TADV report-mapping bins against TS 38.133
// clause 13.5.1, Table 13.5.1-1. Boundary values are picked at bin edges and
// midpoints from the fine-resolution region (0..2047, 128 Tc/step), the
// coarse-resolution region (2048..7689, 512 Tc/step), and the open-ended
// clipping bin (7690).
func TestNrTAToDistance(t *testing.T) {
	const (
		tc           = 1.0 / (480000.0 * 4096.0) // basic time unit, TS 38.211 §4.1
		speedOfLight = 299792458.0
	)

	distanceForTc := func(tadvTc float64) float64 {
		return tadvTc * tc * speedOfLight / 2.0
	}

	tests := []struct {
		name       string
		reported   int32
		wantTadvTc float64 // representative TADV value (Tc) used by the implementation
	}{
		{"reported 0 (fine, first bin, midpoint of [0,128))", 0, 64},
		{"reported 1 (fine, [128,256), midpoint)", 1, 192},
		{"reported 2047 (last fine bin, [262016,262144), midpoint)", 2047, 262080},
		{"reported 2048 (first coarse bin, [262144,262656), midpoint)", 2048, 262400},
		{"reported 2049 ([262656,263168), midpoint)", 2049, 262912},
		{"reported 7689 (last finite coarse bin, [3150336,3150848), midpoint)", 7689, 3150592},
		{"reported 7690 (open-ended clipping bin, lower bound 3150848)", 7690, 3150848},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := distanceForTc(tt.wantTadvTc)
			got := nrTAToDistance(tt.reported)

			if math.Abs(got-want) > 1e-9 {
				t.Errorf("nrTAToDistance(%d) = %v, want %v", tt.reported, got, want)
			}
		})
	}
}

// TestNrTAToDistanceMonotonic verifies the mapping never decreases as the
// reported value increases, since larger NR-TADV values must always
// represent a larger (or equal) round-trip time.
func TestNrTAToDistanceMonotonic(t *testing.T) {
	prev := nrTAToDistance(0)

	for v := int32(1); v <= nrTADVMaxReportedValue; v++ {
		cur := nrTAToDistance(v)
		if cur < prev {
			t.Fatalf("nrTAToDistance(%d) = %v is less than nrTAToDistance(%d) = %v", v, cur, v-1, prev)
		}

		prev = cur
	}
}

// TestNrTAToDistanceNegative verifies that invalid (negative) reported values
// are treated as zero distance rather than producing a nonsensical result.
func TestNrTAToDistanceNegative(t *testing.T) {
	if got := nrTAToDistance(-1); got != 0 {
		t.Errorf("nrTAToDistance(-1) = %v, want 0", got)
	}
}

// TestTAToDistance checks the E-UTRA Timing-Advance report mapping against
// TS 36.133 §10.3.1 Table 10.3.1-1: reported value → round-trip TADV in Ts
// (2 Ts/step up to 4096 Ts, 8 Ts/step above), one-way distance = c·TADV·Ts/2.
func TestTAToDistance(t *testing.T) {
	const oneWayPerTs = 299792458.0 / (15000.0 * 2048.0) / 2.0

	cases := []struct {
		value  int32
		tadvTs float64
	}{
		{0, 0},
		{1, 2},
		{100, 200},
		{2047, 4094},
		{2048, 4096},
		{7689, 49224},
		{7690, 49232},
	}

	for _, c := range cases {
		want := oneWayPerTs * c.tadvTs
		if got := taToDistance(c.value); math.Abs(got-want) > 1e-6 {
			t.Errorf("taToDistance(%d) = %v, want %v", c.value, got, want)
		}
	}
}

func TestHasRadioMeasurements(t *testing.T) {
	if hasRadioMeasurements(nil) {
		t.Fatal("nil measurements must not count as E-CID")
	}

	// Serving cell / AP position only (no UE-specific quantity) is a Cell-ID fix.
	pos := &models.APPosition{LatitudeDegrees: 45}
	if hasRadioMeasurements(&models.RadioMeasurements{APPosition: pos}) {
		t.Fatal("AP position alone must not count as E-CID")
	}

	rsrp := int32(-8500)
	if !hasRadioMeasurements(&models.RadioMeasurements{RSRP: &rsrp}) {
		t.Fatal("a populated RSRP must count as E-CID")
	}
}
