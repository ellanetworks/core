// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package models

import "math"

type GeographicEstimate struct {
	LatitudeDegrees           float64             `json:"latitude_degrees"`
	LongitudeDegrees          float64             `json:"longitude_degrees"`
	AltitudeMeters            *float64            `json:"altitude_meters,omitempty"`
	UncertaintyRadiusMeters   *float64            `json:"uncertainty_radius_meters,omitempty"`
	UncertaintyEllipse        *UncertaintyEllipse `json:"uncertainty_ellipse,omitempty"`
	UncertaintyAltitudeMeters *float64            `json:"uncertainty_altitude_meters,omitempty"`
	ConfidencePercent         *int32              `json:"confidence_percent,omitempty"`
}

type UncertaintyEllipse struct {
	SemiMajorMeters         float64 `json:"semi_major_meters"`
	SemiMinorMeters         float64 `json:"semi_minor_meters"`
	OrientationMajorDegrees int32   `json:"orientation_major_degrees"`
}

const (
	gadMaxUncertaintyCode = 127

	gadHorizontalUncertaintyC = 10.0
	gadHorizontalUncertaintyX = 0.1

	gadAltitudeUncertaintyC = 45.0
	gadAltitudeUncertaintyX = 0.025
)

func HorizontalUncertaintyMeters(k int64) float64 {
	return gadUncertainty(k, gadHorizontalUncertaintyC, gadHorizontalUncertaintyX)
}

func AltitudeUncertaintyMeters(k int64) float64 {
	return gadUncertainty(k, gadAltitudeUncertaintyC, gadAltitudeUncertaintyX)
}

func gadUncertainty(k int64, c, x float64) float64 {
	k = min(max(k, 0), gadMaxUncertaintyCode)

	return c * (math.Pow(1+x, float64(k)) - 1)
}

type EllipsoidPointWithAltitudeAndUncertaintyEllipsoid struct {
	LatitudeDegrees      float64
	LongitudeDegrees     float64
	AltitudeMeters       float64
	UncertaintySemiMajor int64
	UncertaintySemiMinor int64
	OrientationMajor     int64
	UncertaintyAltitude  int64
	Confidence           int64
}

func (g EllipsoidPointWithAltitudeAndUncertaintyEllipsoid) Estimate() *GeographicEstimate {
	altitude := g.AltitudeMeters
	uncertaintyAltitude := AltitudeUncertaintyMeters(g.UncertaintyAltitude)
	confidence := int32(g.Confidence)

	return &GeographicEstimate{
		LatitudeDegrees:  g.LatitudeDegrees,
		LongitudeDegrees: g.LongitudeDegrees,
		AltitudeMeters:   &altitude,
		UncertaintyEllipse: &UncertaintyEllipse{
			SemiMajorMeters:         HorizontalUncertaintyMeters(g.UncertaintySemiMajor),
			SemiMinorMeters:         HorizontalUncertaintyMeters(g.UncertaintySemiMinor),
			OrientationMajorDegrees: int32(g.OrientationMajor),
		},
		UncertaintyAltitudeMeters: &uncertaintyAltitude,
		ConfidencePercent:         &confidence,
	}
}
