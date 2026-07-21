// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"testing"

	"github.com/ellanetworks/core/internal/lmf/models"
	coremodels "github.com/ellanetworks/core/internal/models"
)

func TestToLocationData_CellID(t *testing.T) {
	r := &models.LocationResult{
		SUPI:               "imsi-001010000000001",
		Shape:              models.GADCellID,
		AccessType:         "NR",
		NCGI:               &coremodels.Ncgi{PlmnID: &coremodels.PlmnID{Mcc: "001", Mnc: "01"}, NrCellID: "00066c000"},
		Latitude:           450000000, // 45.0°
		Longitude:          214500000, // 21.45°
		HorizontalAccuracy: 150,
	}

	ld := toLocationData(r, false)

	if ld.LocationEstimate == nil || ld.LocationEstimate.Shape != gadShapeCircle {
		t.Fatalf("expected POINT_UNCERTAINTY_CIRCLE, got %+v", ld.LocationEstimate)
	}

	if ld.LocationEstimate.Point.Lat < 44.99 || ld.LocationEstimate.Point.Lat > 45.01 {
		t.Errorf("lat: got %f, want ~45", ld.LocationEstimate.Point.Lat)
	}

	if ld.LocationEstimate.Uncertainty == nil || *ld.LocationEstimate.Uncertainty != 150 {
		t.Errorf("uncertainty: got %v, want 150", ld.LocationEstimate.Uncertainty)
	}

	if len(ld.PositioningDataList) != 1 || ld.PositioningDataList[0].Method != posMethodCellID {
		t.Errorf("expected CELLID method, got %+v", ld.PositioningDataList)
	}

	if ld.LocNcgi == nil || ld.LocNcgi.NrCellID != "00066c000" {
		t.Errorf("ncgi mismatch: %+v", ld.LocNcgi)
	}

	if ld.SupplementaryMeasurements != nil {
		t.Errorf("supplementary measurements must be absent without verbose")
	}
}

func TestToLocationData_ECID_NR_Verbose(t *testing.T) {
	rsrp := int32(-5600)
	r := &models.LocationResult{
		Shape:              models.GADECID,
		AccessType:         "NR",
		Latitude:           450000000,
		Longitude:          214500000,
		HorizontalAccuracy: 78,
		SSRSRP:             &rsrp,
	}

	ld := toLocationData(r, true)

	if m := ld.PositioningDataList[0].Method; m != posMethodNRECID {
		t.Errorf("expected NR_ECID, got %q", m)
	}

	if ld.SupplementaryMeasurements == nil || ld.SupplementaryMeasurements.SSRSRP == nil || *ld.SupplementaryMeasurements.SSRSRP != -5600 {
		t.Errorf("expected verbose SS-RSRP -5600, got %+v", ld.SupplementaryMeasurements)
	}
}

func TestToLocationData_AGNSS(t *testing.T) {
	r := &models.LocationResult{
		Shape:              models.GADEllipsoidalPoint,
		Latitude:           450000000,
		Longitude:          214500000,
		HorizontalAccuracy: 10,
	}

	ld := toLocationData(r, false)

	if m := ld.PositioningDataList[0].Method; m != posMethodGNSS {
		t.Errorf("expected GNSS, got %q", m)
	}
}

// TestToLocationData_AltitudeConversion verifies that the internally stored
// altitude (in centimeters) is rendered on the wire in meters. The 0.01
// scaling factor is the unit under test; regression would surface as the raw
// cm value leaking through (e.g. 123.45 m reported as 12345 m).
func TestToLocationData_AltitudeConversion(t *testing.T) {
	cases := []struct {
		name    string
		altCm   int32
		want    float64
		wantSet bool
	}{
		{name: "positive", altCm: 12345, want: 123.45, wantSet: true},
		{name: "below sea level", altCm: -5000, want: -50.0, wantSet: true},
		{name: "zero is omitted", altCm: 0, wantSet: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := &models.LocationResult{
				Shape:              models.GADEllipsoidalPoint,
				Latitude:           450000000,
				Longitude:          214500000,
				HorizontalAccuracy: 10,
				Altitude:           tc.altCm,
			}

			ld := toLocationData(r, false)
			if ld.LocationEstimate == nil {
				t.Fatalf("expected non-nil LocationEstimate")
			}

			if !tc.wantSet {
				if ld.LocationEstimate.Altitude != nil {
					t.Errorf("expected no altitude, got %v", *ld.LocationEstimate.Altitude)
				}

				return
			}

			if ld.LocationEstimate.Altitude == nil {
				t.Fatalf("expected non-nil Altitude for %s", tc.name)
			}

			got := *ld.LocationEstimate.Altitude

			const eps = 1e-9
			if got < tc.want-eps || got > tc.want+eps {
				t.Errorf("altitude for %s: got %f m, want %f m", tc.name, got, tc.want)
			}
		})
	}
}
