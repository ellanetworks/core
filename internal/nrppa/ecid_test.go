// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nrppa

import (
	"testing"
)

// TestRoundTrip_ECIDMeasurementInitiationRequest is Stage B: a Request carrying
// MeasurementQuantities round-trips with all fields intact.
func TestRoundTrip_ECIDMeasurementInitiationRequest(t *testing.T) {
	const lmfMeasID = int64(5)

	quantities := []MeasurementQuantityValue{MeasCellID, MeasTimingAdvanceType1, MeasRSRP}

	encoded, err := BuildECIDMeasurementInitiationRequest(lmfMeasID, quantities)
	if err != nil {
		t.Fatalf("BuildECIDMeasurementInitiationRequest: %v", err)
	}

	parsed, err := ParsePDU(encoded)
	if err != nil {
		t.Fatalf("ParsePDU: %v", err)
	}

	if parsed.Kind != KindECIDMeasurementInitiationRequest || parsed.Request == nil {
		t.Fatalf("unexpected parse result: %+v", parsed)
	}

	req := parsed.Request
	if req.LMFUEMeasurementID != lmfMeasID {
		t.Errorf("LMF-UE-Measurement-ID: got %d, want %d", req.LMFUEMeasurementID, lmfMeasID)
	}

	if req.ReportCharacteristics != 0 { // onDemand
		t.Errorf("report characteristics: got %d, want 0 (onDemand)", req.ReportCharacteristics)
	}

	if len(req.MeasurementQuantities) != len(quantities) {
		t.Fatalf("measurement quantities: got %d, want %d", len(req.MeasurementQuantities), len(quantities))
	}

	for i, q := range quantities {
		if req.MeasurementQuantities[i] != q {
			t.Errorf("quantity[%d]: got %d, want %d", i, req.MeasurementQuantities[i], q)
		}
	}

	// ParseECIDMeasurementInitiationRequest should agree.
	req2, err := ParseECIDMeasurementInitiationRequest(encoded)
	if err != nil {
		t.Fatalf("ParseECIDMeasurementInitiationRequest: %v", err)
	}

	if req2.LMFUEMeasurementID != lmfMeasID || len(req2.MeasurementQuantities) != len(quantities) {
		t.Errorf("ParseECIDMeasurementInitiationRequest mismatch: %+v", req2)
	}
}

// TestRoundTrip_ECIDMeasurementInitiationResponse is Stage B: a Response with an
// NR-CGI, serving cell TAC, NG-RANAccessPointPosition and a
// valueTimingAdvanceType1-EUTRA measured result round-trips intact.
func TestRoundTrip_ECIDMeasurementInitiationResponse(t *testing.T) {
	const (
		lmfMeasID = int64(5)
		ranMeasID = int64(9)
	)

	nrCell := uint64(0x123456789) // 36-bit NR cell identity
	ta1 := int64(123)

	result := &ECIDResult{
		ServingCell: ServingCell{
			PLMNIdentity:   []byte{0x00, 0xf1, 0x10},
			NRCellIdentity: &nrCell,
		},
		ServingCellTAC: []byte{0x00, 0x00, 0x01},
		APPosition: &APPosition{
			LatitudeSign:           0, // north
			Latitude:               4194304,
			Longitude:              1000000,
			DirectionOfAltitude:    0, // height
			Altitude:               100,
			UncertaintySemiMajor:   5,
			UncertaintySemiMinor:   5,
			OrientationOfMajorAxis: 0,
			UncertaintyAltitude:    3,
			Confidence:             67,
		},
		TimingAdvanceType1: &ta1,
	}

	encoded, err := BuildECIDMeasurementInitiationResponse(lmfMeasID, ranMeasID, result)
	if err != nil {
		t.Fatalf("BuildECIDMeasurementInitiationResponse: %v", err)
	}

	parsed, err := ParsePDU(encoded)
	if err != nil {
		t.Fatalf("ParsePDU: %v", err)
	}

	if parsed.Kind != KindECIDMeasurementInitiationResponse || parsed.Response == nil {
		t.Fatalf("unexpected parse result: %+v", parsed)
	}

	resp := parsed.Response
	if resp.LMFUEMeasurementID != lmfMeasID {
		t.Errorf("LMF-UE-Measurement-ID: got %d, want %d", resp.LMFUEMeasurementID, lmfMeasID)
	}

	if resp.RANUEMeasurementID != ranMeasID {
		t.Errorf("RAN-UE-Measurement-ID: got %d, want %d", resp.RANUEMeasurementID, ranMeasID)
	}

	if resp.Result == nil {
		t.Fatal("result is nil")
	}

	got := resp.Result

	if want := []byte{0x00, 0xf1, 0x10}; string(got.ServingCell.PLMNIdentity) != string(want) {
		t.Errorf("PLMN identity: got % x, want % x", got.ServingCell.PLMNIdentity, want)
	}

	if got.ServingCell.NRCellIdentity == nil {
		t.Fatal("NR cell identity is nil")
	}

	if *got.ServingCell.NRCellIdentity != nrCell {
		t.Errorf("NR cell identity: got %#x, want %#x", *got.ServingCell.NRCellIdentity, nrCell)
	}

	if want := []byte{0x00, 0x00, 0x01}; string(got.ServingCellTAC) != string(want) {
		t.Errorf("serving cell TAC: got % x, want % x", got.ServingCellTAC, want)
	}

	if got.APPosition == nil {
		t.Fatal("access point position is nil")
	}

	ap := got.APPosition
	if ap.Latitude != 4194304 || ap.Longitude != 1000000 || ap.Altitude != 100 || ap.Confidence != 67 {
		t.Errorf("access point position mismatch: %+v", ap)
	}

	if ap.UncertaintySemiMajor != 5 || ap.UncertaintySemiMinor != 5 || ap.UncertaintyAltitude != 3 {
		t.Errorf("access point uncertainty mismatch: %+v", ap)
	}

	// Latitude 4194304 = 2^22 → 2^22 * 90 / 2^23 = 45 degrees (north).
	if ap.LatitudeDegrees < 44.99 || ap.LatitudeDegrees > 45.01 {
		t.Errorf("latitude degrees: got %f, want ~45", ap.LatitudeDegrees)
	}

	if got.TimingAdvanceType1 == nil || *got.TimingAdvanceType1 != ta1 {
		t.Errorf("timing advance type 1: got %v, want %d", got.TimingAdvanceType1, ta1)
	}
}

// TestRoundTrip_Response_EUTRACell checks the E-UTRA cell-identity alternative
// of the NG-RANCell CHOICE round-trips.
func TestRoundTrip_Response_EUTRACell(t *testing.T) {
	eutraCell := uint64(0x0ABCDEF) // 28-bit

	result := &ECIDResult{
		ServingCell: ServingCell{
			PLMNIdentity: []byte{0x02, 0xf8, 0x39},
			EUTRACellID:  &eutraCell,
		},
		ServingCellTAC: []byte{0x12, 0x34, 0x56},
	}

	encoded, err := BuildECIDMeasurementInitiationResponse(1, 2, result)
	if err != nil {
		t.Fatalf("BuildECIDMeasurementInitiationResponse: %v", err)
	}

	parsed, err := ParsePDU(encoded)
	if err != nil {
		t.Fatalf("ParsePDU: %v", err)
	}

	if parsed.Response == nil || parsed.Response.Result == nil {
		t.Fatal("missing response/result")
	}

	got := parsed.Response.Result.ServingCell
	if got.EUTRACellID == nil {
		t.Fatal("E-UTRA cell identity is nil")
	}

	if *got.EUTRACellID != eutraCell {
		t.Errorf("E-UTRA cell identity: got %#x, want %#x", *got.EUTRACellID, eutraCell)
	}

	if got.NRCellIdentity != nil {
		t.Errorf("NR cell identity should be nil, got %#x", *got.NRCellIdentity)
	}
}

// TestRoundTrip_Response_NoResult checks a response without an
// E-CID-MeasurementResult (the optional IE absent) round-trips.
func TestRoundTrip_Response_NoResult(t *testing.T) {
	encoded, err := BuildECIDMeasurementInitiationResponse(4, 8, nil)
	if err != nil {
		t.Fatalf("BuildECIDMeasurementInitiationResponse: %v", err)
	}

	parsed, err := ParsePDU(encoded)
	if err != nil {
		t.Fatalf("ParsePDU: %v", err)
	}

	if parsed.Response == nil {
		t.Fatal("response is nil")
	}

	if parsed.Response.LMFUEMeasurementID != 4 || parsed.Response.RANUEMeasurementID != 8 {
		t.Errorf("measurement ids: got lmf=%d ran=%d, want 4/8",
			parsed.Response.LMFUEMeasurementID, parsed.Response.RANUEMeasurementID)
	}

	if parsed.Response.Result != nil {
		t.Errorf("expected nil result, got %+v", parsed.Response.Result)
	}
}
