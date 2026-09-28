// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/lmf/models"
	coremodels "github.com/ellanetworks/core/internal/models"
)

func ptr[T any](v T) *T { return &v }

func gnssAttempt() []models.PositioningAttempt {
	return []models.PositioningAttempt{{Method: models.PositioningMethodGNSS, Mode: models.PositioningModeStandalone, Usage: models.PositioningUsageResultsUsedToGenerate}}
}

func circleEstimate(radius float64) *models.GeographicEstimate {
	return &models.GeographicEstimate{LatitudeDegrees: 45, LongitudeDegrees: 21.45, UncertaintyRadiusMeters: &radius}
}

func TestToLocationData_CellID(t *testing.T) {
	r := &models.LocationResult{
		SUPI:        "imsi-001010000000001",
		Positioning: []models.PositioningAttempt{{Method: models.PositioningMethodCellID, Mode: models.PositioningModeNetworkBased, Usage: models.PositioningUsageResultsUsedToGenerate}},
		AccessType:  "NR",
		NCGI:        &coremodels.Ncgi{PlmnID: &coremodels.PlmnID{Mcc: "001", Mnc: "01"}, NrCellID: "00066c000"},
		Estimate:    circleEstimate(150),
	}

	ld := toLocationData(r)

	if ld.LocationEstimate.Shape != gadShapeCircle {
		t.Fatalf("expected POINT_UNCERTAINTY_CIRCLE, got %+v", ld.LocationEstimate)
	}

	if ld.LocationEstimate.Point.Lat != 45 {
		t.Errorf("lat: got %f, want 45", ld.LocationEstimate.Point.Lat)
	}

	if ld.LocationEstimate.Uncertainty == nil || *ld.LocationEstimate.Uncertainty != 150 {
		t.Errorf("uncertainty: got %v, want 150", ld.LocationEstimate.Uncertainty)
	}

	want := []PositioningMethodAndUsage{{Method: posMethodCellID, Mode: posModeConvention, Usage: "SUCCESS_RESULTS_USED_TO_GENERATE_LOCATION"}}
	if !reflect.DeepEqual(ld.PositioningDataList, want) {
		t.Errorf("positioningDataList = %+v, want %+v", ld.PositioningDataList, want)
	}

	if ld.LocNcgi == nil || ld.LocNcgi.NrCellID != "00066c000" {
		t.Errorf("ncgi mismatch: %+v", ld.LocNcgi)
	}

	if ld.RadioMeasurements != nil {
		t.Errorf("radio measurements = %+v, want none", ld.RadioMeasurements)
	}
}

func TestToLocationData_ECID(t *testing.T) {
	plmn := &coremodels.PlmnID{Mcc: "999", Mnc: "70"}
	r := &models.LocationResult{
		AccessType: "NR",
		Estimate:   circleEstimate(78),
		Positioning: []models.PositioningAttempt{
			{Method: models.PositioningMethodCellID, Mode: models.PositioningModeNetworkBased, Usage: models.PositioningUsageResultsUsedToGenerate},
			{Method: models.PositioningMethodNRECID, Mode: models.PositioningModeNetworkBased, Usage: models.PositioningUsageUnsuccess},
			{Method: models.PositioningMethodNRECID, Mode: models.PositioningModeUEAssisted, Usage: models.PositioningUsageResultsNotUsed},
		},
		Measurements: []models.CellMeasurement{{
			Source: models.MeasurementSourceUE, RAT: models.RATNR, Serving: true,
			PCI: ptr(int64(180)), ARFCN: ptr(int64(662592)), ARFCNType: models.ARFCNTypeSSB,
			NCGI:     &coremodels.Ncgi{PlmnID: plmn, NrCellID: "000000fc1"},
			SSRSRP:   ptr(-84.0),
			SSBBeams: []models.BeamMeasurement{{Index: 0, RSRP: ptr(-84.0), RSRQ: ptr(-11.0)}},
		}},
	}

	ld := toLocationData(r)

	wantList := []PositioningMethodAndUsage{
		{Method: posMethodCellID, Mode: posModeConvention, Usage: "SUCCESS_RESULTS_USED_TO_GENERATE_LOCATION"},
		{Method: posMethodNRECID, Mode: posModeConvention, Usage: "UNSUCCESS"},
		{Method: posMethodNRECID, Mode: posModeUEAssisted, Usage: "SUCCESS_RESULTS_NOT_USED"},
	}
	if !reflect.DeepEqual(ld.PositioningDataList, wantList) {
		t.Errorf("positioningDataList = %+v, want %+v", ld.PositioningDataList, wantList)
	}

	b, err := json.Marshal(ld.RadioMeasurements)
	if err != nil {
		t.Fatal(err)
	}

	want := `[{"source":"UE","rat":"NR","serving":true,"pci":180,"arfcn":662592,"arfcnType":"SSB",` +
		`"ncgi":{"plmnId":{"mcc":"999","mnc":"70"},"nrCellId":"000000fc1"},"ssRsrp":-84,` +
		`"ssbBeams":[{"index":0,"rsrp":-84,"rsrq":-11}]}]`
	if string(b) != want {
		t.Errorf("radioMeasurements =\n%s\nwant\n%s", b, want)
	}
}

func TestToLocationData_GNSS(t *testing.T) {
	r := &models.LocationResult{Positioning: gnssAttempt(), Estimate: circleEstimate(10)}

	ld := toLocationData(r)

	if len(ld.PositioningDataList) != 0 {
		t.Errorf("positioningDataList = %+v, want none for a GNSS fix", ld.PositioningDataList)
	}

	want := []GnssPositioningMethodAndUsage{{Mode: posModeConvention, Gnss: gnssGPS, Usage: "SUCCESS_RESULTS_USED_TO_GENERATE_LOCATION"}}
	if !reflect.DeepEqual(ld.GnssPositioningDataList, want) {
		t.Errorf("gnssPositioningDataList = %+v, want %+v", ld.GnssPositioningDataList, want)
	}
}

func TestToGeographicArea(t *testing.T) {
	ellipse := &models.UncertaintyEllipse{SemiMajorMeters: 21.4, SemiMinorMeters: 11.4, OrientationMajorDegrees: 30}

	for _, tc := range []struct {
		name         string
		estimate     *models.GeographicEstimate
		want         GeographicArea
		wantAltitude *float64
	}{
		{
			name:     "point",
			estimate: &models.GeographicEstimate{LatitudeDegrees: 45, LongitudeDegrees: 21.45},
			want:     GeographicArea{Shape: gadShapePoint, Point: GeographicalCoord{Lat: 45, Lon: 21.45}},
		},
		{
			name:     "point with altitude",
			estimate: &models.GeographicEstimate{LatitudeDegrees: 45, LongitudeDegrees: 21.45, AltitudeMeters: ptr(16.0)},
			want:     GeographicArea{Shape: gadShapePointAltitude, Point: GeographicalCoord{Lat: 45, Lon: 21.45}, Altitude: ptr(16.0)},
		},
		{
			name:         "circle with altitude",
			estimate:     &models.GeographicEstimate{LatitudeDegrees: 45, LongitudeDegrees: 21.45, AltitudeMeters: ptr(16.0), UncertaintyRadiusMeters: ptr(150.0)},
			want:         GeographicArea{Shape: gadShapeCircle, Point: GeographicalCoord{Lat: 45, Lon: 21.45}, Uncertainty: ptr(150.0)},
			wantAltitude: ptr(16.0),
		},
		{
			name:     "ellipse",
			estimate: &models.GeographicEstimate{LatitudeDegrees: 45, LongitudeDegrees: 21.45, UncertaintyEllipse: ellipse, ConfidencePercent: ptr(int32(68))},
			want: GeographicArea{
				Shape: gadShapeEllipse, Point: GeographicalCoord{Lat: 45, Lon: 21.45},
				UncertaintyEllipse: &UncertaintyEllipse{SemiMajor: 21.4, SemiMinor: 11.4, OrientationMajor: 30},
				Confidence:         ptr(int32(68)),
			},
		},
		{
			name: "ellipsoid with altitude",
			estimate: &models.GeographicEstimate{
				LatitudeDegrees: 45, LongitudeDegrees: 21.45, AltitudeMeters: ptr(16.0),
				UncertaintyEllipse: ellipse, UncertaintyAltitudeMeters: ptr(28.5), ConfidencePercent: ptr(int32(68)),
			},
			want: GeographicArea{
				Shape: gadShapePointAltitudeUncertainty, Point: GeographicalCoord{Lat: 45, Lon: 21.45}, Altitude: ptr(16.0),
				UncertaintyEllipse:  &UncertaintyEllipse{SemiMajor: 21.4, SemiMinor: 11.4, OrientationMajor: 30},
				UncertaintyAltitude: ptr(28.5), Confidence: ptr(int32(68)),
			},
		},
		{
			name:     "ellipse without confidence falls back to a circle",
			estimate: &models.GeographicEstimate{LatitudeDegrees: 45, LongitudeDegrees: 21.45, UncertaintyEllipse: ellipse},
			want:     GeographicArea{Shape: gadShapeCircle, Point: GeographicalCoord{Lat: 45, Lon: 21.45}, Uncertainty: ptr(21.4)},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, altitude := toGeographicArea(tc.estimate)

			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("area = %+v, want %+v", got, tc.want)
			}

			if !reflect.DeepEqual(altitude, tc.wantAltitude) {
				t.Errorf("top-level altitude = %v, want %v", altitude, tc.wantAltitude)
			}
		})
	}
}

func TestToLocationData_WireFormatMatchesSpec(t *testing.T) {
	r := &models.LocationResult{
		Positioning: gnssAttempt(),
		Estimate: &models.GeographicEstimate{
			LatitudeDegrees: 45, LongitudeDegrees: 21.45, AltitudeMeters: ptr(16.0),
			UncertaintyEllipse:        &models.UncertaintyEllipse{SemiMajorMeters: 21.4, SemiMinorMeters: 11.4, OrientationMajorDegrees: 30},
			UncertaintyAltitudeMeters: ptr(28.5),
			ConfidencePercent:         ptr(int32(68)),
		},
		TAI: &coremodels.Tai{PlmnID: &coremodels.PlmnID{Mcc: "001", Mnc: "01"}, Tac: "000001"},
	}

	b, err := json.Marshal(toLocationData(r))
	if err != nil {
		t.Fatal(err)
	}

	var wire map[string]any
	if err := json.Unmarshal(b, &wire); err != nil {
		t.Fatal(err)
	}

	estimate := wire["locationEstimate"].(map[string]any)

	want := map[string]any{
		"shape":               "POINT_ALTITUDE_UNCERTAINTY",
		"point":               map[string]any{"lat": 45.0, "lon": 21.45},
		"altitude":            16.0,
		"uncertaintyEllipse":  map[string]any{"semiMajor": 21.4, "semiMinor": 11.4, "orientationMajor": 30.0},
		"uncertaintyAltitude": 28.5,
		"confidence":          68.0,
	}
	if !reflect.DeepEqual(estimate, want) {
		t.Errorf("locationEstimate = %v, want %v", estimate, want)
	}

	if _, ok := wire["altitude"]; ok {
		t.Error("altitude is repeated at the top level although the shape carries it")
	}

	if _, ok := wire["tai"]; ok {
		t.Error("tai is not a LocationData attribute")
	}

	gnss := wire["gnssPositioningDataList"].([]any)[0].(map[string]any)
	if gnss["usage"] != "SUCCESS_RESULTS_USED_TO_GENERATE_LOCATION" {
		t.Errorf("usage = %v, want SUCCESS_RESULTS_USED_TO_GENERATE_LOCATION", gnss["usage"])
	}
}

func TestStoredLocationData(t *testing.T) {
	stored, err := json.Marshal(&models.LocationResult{Positioning: gnssAttempt(), Estimate: circleEstimate(10)})
	if err != nil {
		t.Fatal(err)
	}

	s := string(stored)

	ld := storedLocationData(&s)
	if ld == nil || ld.LocationEstimate.Shape != gadShapeCircle || len(ld.GnssPositioningDataList) != 1 {
		t.Fatalf("storedLocationData = %+v, want the spec-shaped rendering of the stored result", ld)
	}

	if storedLocationData(nil) != nil {
		t.Error("a session without a result rendered one")
	}
}

func TestToLocationData_MeasurementsWithoutEstimate(t *testing.T) {
	measuredAt := time.Date(2026, 9, 28, 21, 0, 13, 0, time.UTC)
	r := &models.LocationResult{
		AccessType:          "EUTRA",
		UeLocationTimestamp: &measuredAt,
		AgeOfLocationInfo:   2,
		ECGI:                &coremodels.Ecgi{PlmnID: &coremodels.PlmnID{Mcc: "001", Mnc: "01"}, EutraCellID: "5ee0000"},
		Positioning: []models.PositioningAttempt{
			{Method: models.PositioningMethodECID, Mode: models.PositioningModeUEAssisted, Usage: models.PositioningUsageResultsNotUsed},
		},
		Measurements: []models.CellMeasurement{{Source: models.MeasurementSourceUE, RAT: models.RATEUTRA, Serving: true, RSRP: ptr(-76.0)}},
	}

	b, err := json.Marshal(toLocationData(r))
	if err != nil {
		t.Fatal(err)
	}

	var wire map[string]any
	if err := json.Unmarshal(b, &wire); err != nil {
		t.Fatal(err)
	}

	if _, ok := wire["locationEstimate"]; ok {
		t.Errorf("locationEstimate = %v, want it absent without an estimate", wire["locationEstimate"])
	}

	for _, attr := range []string{"timestampOfLocationEstimate", "ageOfLocationEstimate"} {
		if _, ok := wire[attr]; ok {
			t.Errorf("%s = %v, want it absent without an estimate", attr, wire[attr])
		}
	}

	if m, ok := wire["radioMeasurements"].([]any); !ok || len(m) != 1 {
		t.Errorf("radioMeasurements = %v, want the measured cell", wire["radioMeasurements"])
	}

	stored, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}

	s := string(stored)
	if ld := storedLocationData(&s); ld == nil || len(ld.RadioMeasurements) != 1 {
		t.Errorf("storedLocationData = %+v, want the stored measurements", ld)
	}
}
