// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package models

import (
	"math"
	"testing"
)

func TestGADUncertainty(t *testing.T) {
	for _, tc := range []struct {
		name string
		got  float64
		want float64
	}{
		{"horizontal k=0", HorizontalUncertaintyMeters(0), 0},
		{"horizontal k=12", HorizontalUncertaintyMeters(12), 21.384},
		{"horizontal clamps above 127", HorizontalUncertaintyMeters(200), HorizontalUncertaintyMeters(127)},
		{"altitude k=0", AltitudeUncertaintyMeters(0), 0},
		{"altitude k=19", AltitudeUncertaintyMeters(19), 26.938},
		{"altitude k=127", AltitudeUncertaintyMeters(127), 990.5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if math.Abs(tc.got-tc.want) > 0.5 {
				t.Errorf("got %f m, want %f m", tc.got, tc.want)
			}
		})
	}
}

func TestEllipsoidPointWithAltitudeAndUncertaintyEllipsoidEstimate(t *testing.T) {
	e := EllipsoidPointWithAltitudeAndUncertaintyEllipsoid{
		LatitudeDegrees: 48.4, LongitudeDegrees: -68.6, AltitudeMeters: 16,
		UncertaintySemiMajor: 12, UncertaintySemiMinor: 8, OrientationMajor: 30, UncertaintyAltitude: 19, Confidence: 68,
	}.Estimate()

	if e.UncertaintyEllipse == nil || e.UncertaintyEllipse.OrientationMajorDegrees != 30 {
		t.Fatalf("ellipse = %+v, want orientation 30", e.UncertaintyEllipse)
	}

	if math.Abs(e.UncertaintyEllipse.SemiMajorMeters-HorizontalUncertaintyMeters(12)) > 1e-9 ||
		math.Abs(e.UncertaintyEllipse.SemiMinorMeters-HorizontalUncertaintyMeters(8)) > 1e-9 {
		t.Errorf("ellipse axes = %+v", e.UncertaintyEllipse)
	}

	if e.UncertaintyAltitudeMeters == nil || math.Abs(*e.UncertaintyAltitudeMeters-AltitudeUncertaintyMeters(19)) > 1e-9 {
		t.Errorf("altitude uncertainty = %v, want the TS 23.032 altitude formula", e.UncertaintyAltitudeMeters)
	}

	if e.AltitudeMeters == nil || *e.AltitudeMeters != 16 || e.ConfidencePercent == nil || *e.ConfidencePercent != 68 {
		t.Errorf("estimate = %+v", e)
	}
}
