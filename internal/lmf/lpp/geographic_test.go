// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lpp

import "testing"

func TestEncodeLongitudeRoundsDown(t *testing.T) {
	cases := []struct {
		lonE7 int32
		want  int64
	}{
		{0, 0},
		{1, 0},
		{-1, -1},
		{1224194000, 5705157},
		{-1224194000, -5705158},
		{1800000000, maxDegreesLongitude},
		{-1800000000, minDegreesLongitude},
	}

	for _, tc := range cases {
		if got := encodeLongitude(tc.lonE7); got != tc.want {
			t.Errorf("encodeLongitude(%d) = %d, want %d", tc.lonE7, got, tc.want)
		}
	}
}

func TestLongitudeRoundTripWest(t *testing.T) {
	const lonE7 = -1224194000

	got := longitudeDegrees(encodeLongitude(lonE7))
	want := float64(lonE7) / 1e7

	if got > want || want-got > 360.0/longitudeResolution {
		t.Errorf("longitudeDegrees(encodeLongitude(%d)) = %.9f, want within one step below %.9f", lonE7, got, want)
	}
}
