// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nrppa

import (
	"testing"
)

// TestRoundTrip_Response_NRTimingAndAngle verifies the NR-specific extension
// measured results — Angle of Arrival NR (UL-AoA), Value Timing Advance NR and
// UE Rx-Tx Time Difference — round-trip through encode/decode intact.
func TestRoundTrip_Response_NRTimingAndAngle(t *testing.T) {
	nrCell := uint64(0x123456789)
	nrTA := int64(100)
	rxTx := int64(1200)
	zenith := int64(450)

	result := &ECIDResult{
		ServingCell: ServingCell{
			PLMNIdentity:   []byte{0x00, 0xf1, 0x10},
			NRCellIdentity: &nrCell,
		},
		ServingCellTAC:  []byte{0x00, 0x00, 0x01},
		NRTimingAdvance: &nrTA,
		UERxTxTimeDiff:  &rxTx,
		AoA: &AoAResult{
			AzimuthRaw: 900, // 90.0°
			ZenithRaw:  &zenith,
		},
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

	got := parsed.Response.Result

	if got.NRTimingAdvance == nil || *got.NRTimingAdvance != nrTA {
		t.Errorf("NR-TADV: got %v, want %d", got.NRTimingAdvance, nrTA)
	}

	if got.UERxTxTimeDiff == nil || *got.UERxTxTimeDiff != rxTx {
		t.Errorf("UE Rx-Tx: got %v, want %d", got.UERxTxTimeDiff, rxTx)
	}

	if got.AoA == nil {
		t.Fatal("AoA is nil")
	}

	if got.AoA.AzimuthRaw != 900 || got.AoA.AzimuthDegrees < 89.99 || got.AoA.AzimuthDegrees > 90.01 {
		t.Errorf("AoA azimuth: got raw=%d deg=%f, want raw=900 deg=90", got.AoA.AzimuthRaw, got.AoA.AzimuthDegrees)
	}

	if got.AoA.ZenithRaw == nil || *got.AoA.ZenithRaw != 450 || got.AoA.ZenithDegrees == nil {
		t.Errorf("AoA zenith: got %+v, want raw=450 deg=45", got.AoA)
	}
}

// TestRoundTrip_ECIDMeasurementTerminationCommand verifies the E-CID Measurement
// Termination Command (procedureCode 5) round-trips with both measurement ids.
func TestRoundTrip_ECIDMeasurementTerminationCommand(t *testing.T) {
	const (
		lmfMeasID = int64(4)
		ranMeasID = int64(1)
	)

	encoded, err := BuildECIDMeasurementTerminationCommand(lmfMeasID, ranMeasID)
	if err != nil {
		t.Fatalf("BuildECIDMeasurementTerminationCommand: %v", err)
	}

	parsed, err := ParsePDU(encoded)
	if err != nil {
		t.Fatalf("ParsePDU: %v", err)
	}

	if parsed.Kind != KindECIDMeasurementTerminationCommand || parsed.Termination == nil {
		t.Fatalf("unexpected parse result: %+v", parsed)
	}

	if parsed.Termination.LMFUEMeasurementID != lmfMeasID {
		t.Errorf("LMF-UE-Measurement-ID: got %d, want %d", parsed.Termination.LMFUEMeasurementID, lmfMeasID)
	}

	if parsed.Termination.RANUEMeasurementID != ranMeasID {
		t.Errorf("RAN-UE-Measurement-ID: got %d, want %d", parsed.Termination.RANUEMeasurementID, ranMeasID)
	}
}

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

// TestRoundTrip_Response_NRMeasurements encodes SS-RSRP, SS-RSRQ, CSI-RSRP and
// CSI-RSRQ via choice-Extension and verifies the round-trip preserves all values.
func TestRoundTrip_Response_NRMeasurements(t *testing.T) {
	nrCell := uint64(0x123456789)

	ssrsrpVal := int64(42) // maps to -99 dBm
	ssrsrqVal := int64(18) // maps to about -10.5 dB
	csirsrpVal := int64(60)
	csirsrqVal := int64(25)

	result := &ECIDResult{
		ServingCell: ServingCell{
			PLMNIdentity:   []byte{0x00, 0xf1, 0x10},
			NRCellIdentity: &nrCell,
		},
		ServingCellTAC: []byte{0x00, 0x00, 0x01},
		ResultSSRSRP: &SSRSRPResult{
			Items: []SSRSRPItem{{NRPCI: 1, Value: ssrsrpVal}},
		},
		ResultSSRSRQ: &SSRSRQResult{
			Items: []SSRSRQItem{{NRPCI: 1, Value: ssrsrqVal}},
		},
		ResultCSIRSRP: &CSIRSRPResult{
			Items: []CSIRSRPItem{{NRPCI: 1, Value: csirsrpVal}},
		},
		ResultCSIRSRQ: &CSIRSRQResult{
			Items: []CSIRSRQItem{{NRPCI: 1, Value: csirsrqVal}},
		},
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

	got := parsed.Response.Result

	if got.ResultSSRSRP == nil || len(got.ResultSSRSRP.Items) != 1 || got.ResultSSRSRP.Items[0].Value != ssrsrpVal {
		t.Errorf("SS-RSRP: got %+v, want value=%d", got.ResultSSRSRP, ssrsrpVal)
	}

	if got.ResultSSRSRQ == nil || len(got.ResultSSRSRQ.Items) != 1 || got.ResultSSRSRQ.Items[0].Value != ssrsrqVal {
		t.Errorf("SS-RSRQ: got %+v, want value=%d", got.ResultSSRSRQ, ssrsrqVal)
	}

	if got.ResultCSIRSRP == nil || len(got.ResultCSIRSRP.Items) != 1 || got.ResultCSIRSRP.Items[0].Value != csirsrpVal {
		t.Errorf("CSI-RSRP: got %+v, want value=%d", got.ResultCSIRSRP, csirsrpVal)
	}

	if got.ResultCSIRSRQ == nil || len(got.ResultCSIRSRQ.Items) != 1 || got.ResultCSIRSRQ.Items[0].Value != csirsrqVal {
		t.Errorf("CSI-RSRQ: got %+v, want value=%d", got.ResultCSIRSRQ, csirsrqVal)
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
