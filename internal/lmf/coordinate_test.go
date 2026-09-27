// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lmf

import (
	"reflect"
	"testing"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/lmf/models"
)

func ptr[T any](v T) *T { return &v }

func TestCellPositionEstimate(t *testing.T) {
	for _, tc := range []struct {
		name string
		cp   db.CellPosition
		want *models.GeographicEstimate
	}{
		{
			name: "coordinate only",
			cp:   db.CellPosition{Latitude: 45, Longitude: 21.5},
			want: &models.GeographicEstimate{LatitudeDegrees: 45, LongitudeDegrees: 21.5},
		},
		{
			name: "full ellipse with altitude and confidence",
			cp: db.CellPosition{
				Latitude: 45, Longitude: 21.5, Altitude: ptr(120.0),
				UncertaintySemiMajor: ptr(50.0), UncertaintySemiMinor: ptr(300.0), OrientationMajor: ptr(40), Confidence: ptr(68),
			},
			want: &models.GeographicEstimate{
				LatitudeDegrees: 45, LongitudeDegrees: 21.5, AltitudeMeters: ptr(120.0),
				UncertaintyEllipse: &models.UncertaintyEllipse{SemiMajorMeters: 300, SemiMinorMeters: 50, OrientationMajorDegrees: 40},
				ConfidencePercent:  ptr(int32(68)),
			},
		},
		{
			name: "axes without orientation become a circle of the larger axis",
			cp:   db.CellPosition{Latitude: 45, Longitude: 21.5, UncertaintySemiMajor: ptr(50.0), UncertaintySemiMinor: ptr(300.0)},
			want: &models.GeographicEstimate{LatitudeDegrees: 45, LongitudeDegrees: 21.5, UncertaintyRadiusMeters: ptr(300.0)},
		},
		{
			name: "single axis is a circle",
			cp:   db.CellPosition{Latitude: 45, Longitude: 21.5, UncertaintySemiMajor: ptr(150.0)},
			want: &models.GeographicEstimate{LatitudeDegrees: 45, LongitudeDegrees: 21.5, UncertaintyRadiusMeters: ptr(150.0)},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := cellPositionEstimate(&tc.cp); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("estimate = %+v, want %+v", got, tc.want)
			}
		})
	}
}
