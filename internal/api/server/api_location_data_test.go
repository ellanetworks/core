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
