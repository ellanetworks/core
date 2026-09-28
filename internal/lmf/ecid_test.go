// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lmf

import (
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/ellanetworks/core/internal/lmf/models"
	coremodels "github.com/ellanetworks/core/internal/models"
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

func TestECIDPositioning(t *testing.T) {
	anchor := &models.GeographicEstimate{LatitudeDegrees: 45}
	cells := []models.CellMeasurement{{Source: models.MeasurementSourceNetwork, RAT: models.RATNR, Serving: true}}
	cellID := models.PositioningAttempt{Method: models.PositioningMethodCellID, Mode: models.PositioningModeNetworkBased, Usage: models.PositioningUsageResultsUsedToGenerate}

	attempt := func(mode models.PositioningMode, usage models.PositioningUsage) models.PositioningAttempt {
		return models.PositioningAttempt{Method: models.PositioningMethodNRECID, Mode: mode, Usage: usage}
	}

	cases := []struct {
		name        string
		wantNetwork bool
		wantUE      bool
		network     *models.RadioMeasurements
		ueErr       error
		noAnchor    bool
		want        []models.PositioningAttempt
	}{
		{
			name:        "RAN anchor generates the estimate",
			wantNetwork: true,
			wantUE:      true,
			network:     &models.RadioMeasurements{Cells: cells, APPosition: anchor},
			want: []models.PositioningAttempt{
				attempt(models.PositioningModeNetworkBased, models.PositioningUsageResultsUsedToGenerate),
				attempt(models.PositioningModeUEAssisted, models.PositioningUsageResultsNotUsed),
			},
		},
		{
			name:        "cell table generates the estimate, measurements reported but unused",
			wantNetwork: true,
			wantUE:      true,
			network:     &models.RadioMeasurements{Cells: cells},
			want: []models.PositioningAttempt{
				cellID,
				attempt(models.PositioningModeNetworkBased, models.PositioningUsageResultsNotUsed),
				attempt(models.PositioningModeUEAssisted, models.PositioningUsageResultsNotUsed),
			},
		},
		{
			name:        "both variants fail",
			wantNetwork: true,
			wantUE:      true,
			ueErr:       errors.New("UE does not support E-CID"),
			want: []models.PositioningAttempt{
				cellID,
				attempt(models.PositioningModeNetworkBased, models.PositioningUsageUnsuccess),
				attempt(models.PositioningModeUEAssisted, models.PositioningUsageUnsuccess),
			},
		},
		{
			name:        "no position for the serving cell, measurements still reported",
			wantNetwork: true,
			wantUE:      true,
			network:     &models.RadioMeasurements{Cells: cells},
			noAnchor:    true,
			want: []models.PositioningAttempt{
				attempt(models.PositioningModeNetworkBased, models.PositioningUsageResultsNotUsed),
				attempt(models.PositioningModeUEAssisted, models.PositioningUsageResultsNotUsed),
			},
		},
		{
			name:   "UE-assisted only",
			wantUE: true,
			want: []models.PositioningAttempt{
				cellID,
				attempt(models.PositioningModeUEAssisted, models.PositioningUsageResultsNotUsed),
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ecidPositioning(models.PositioningMethodNRECID, tc.wantNetwork, tc.wantUE, tc.network, tc.ueErr, !tc.noAnchor)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestWithDistancePrefersNRTimingAdvance(t *testing.T) {
	nrTA, ta := int64(10), int64(100)

	cells := withDistance([]models.CellMeasurement{
		{NRTimingAdvance: &nrTA, TimingAdvance: &ta},
		{TimingAdvance: &ta},
		{},
	})

	if d := cells[0].DistanceMeters; d == nil || *d != nrTAToDistance(10) {
		t.Errorf("NR TA distance = %v, want %v", d, nrTAToDistance(10))
	}

	if d := cells[1].DistanceMeters; d == nil || *d != taToDistance(100) {
		t.Errorf("TA distance = %v, want %v", d, taToDistance(100))
	}

	if cells[2].DistanceMeters != nil {
		t.Errorf("distance without timing = %v, want nil", *cells[2].DistanceMeters)
	}
}

func TestValidateMode(t *testing.T) {
	cases := []struct {
		method RequestedMethod
		mode   models.PositioningMode
		ok     bool
	}{
		{RequestedCellID, "", true},
		{RequestedCellID, models.PositioningModeNetworkBased, false},
		{RequestedECID, "", true},
		{RequestedECID, models.PositioningModeUEAssisted, true},
		{RequestedECID, models.PositioningModeNetworkBased, true},
		{RequestedECID, models.PositioningModeUEBased, false},
		{RequestedGNSS, models.PositioningModeStandalone, true},
		{RequestedGNSS, models.PositioningModeUEAssisted, false},
		{RequestedGNSS, "bogus", false},
	}

	for _, tc := range cases {
		err := ValidateMode(tc.method, tc.mode)
		if (err == nil) != tc.ok {
			t.Errorf("ValidateMode(%q, %q) = %v, want ok=%t", tc.method, tc.mode, err, tc.ok)
		}

		if err != nil && !errors.Is(err, ErrUnsupportedMode) {
			t.Errorf("ValidateMode(%q, %q) = %v, want ErrUnsupportedMode", tc.method, tc.mode, err)
		}
	}
}

func TestMarkServingMatchesTheServingCellGlobalID(t *testing.T) {
	plmn := &coremodels.PlmnID{Mcc: "001", Mnc: "01"}
	serving := &coremodels.Ecgi{PlmnID: plmn, EutraCellID: "5ee0000"}

	cells := markServing([]models.CellMeasurement{
		{RAT: models.RATEUTRA, ECGI: &coremodels.Ecgi{PlmnID: plmn, EutraCellID: "5ee0000"}},
		{RAT: models.RATEUTRA, ECGI: &coremodels.Ecgi{PlmnID: plmn, EutraCellID: "5ee0001"}},
		{RAT: models.RATEUTRA},
		{RAT: models.RATNR, Serving: true},
	}, nil, serving)

	want := []bool{true, false, false, true}
	for i, c := range cells {
		if c.Serving != want[i] {
			t.Errorf("cell %d serving = %t, want %t", i, c.Serving, want[i])
		}
	}
}

func TestUEECIDTimeoutKeepsTheDefaultWithinTheNetworkBudget(t *testing.T) {
	if got := ueECIDTimeout(""); got != ecidMeasurementTimeout {
		t.Errorf("default mode UE timeout = %s, want %s", got, ecidMeasurementTimeout)
	}

	if got := ueECIDTimeout(models.PositioningModeUEAssisted); got != ecidUETimeout {
		t.Errorf("ue_assisted UE timeout = %s, want %s", got, ecidUETimeout)
	}
}
