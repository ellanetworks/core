// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lppa

import (
	"bytes"
	"testing"
)

func TestECIDMeasurementInitiationRequestRoundTrip(t *testing.T) {
	quantities := []MeasurementQuantityValue{MeasCellID, MeasAngleOfArrival, MeasTimingAdvanceType1, MeasRSRP, MeasRSRQ}

	pdu, err := BuildECIDMeasurementInitiationRequest(7, quantities)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	parsed, err := ParsePDU(pdu)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if parsed.Kind != KindECIDMeasurementInitiationRequest {
		t.Fatalf("kind = %d, want request", parsed.Kind)
	}

	req := parsed.Request
	if req.ESMLCUEMeasurementID != 7 {
		t.Fatalf("meas id = %d, want 7", req.ESMLCUEMeasurementID)
	}

	if req.ReportCharacteristics != reportOnDemand {
		t.Fatalf("report characteristics = %d, want onDemand", req.ReportCharacteristics)
	}

	if len(req.MeasurementQuantities) != len(quantities) {
		t.Fatalf("quantities len = %d, want %d", len(req.MeasurementQuantities), len(quantities))
	}

	for i, q := range quantities {
		if req.MeasurementQuantities[i] != q {
			t.Fatalf("quantity[%d] = %d, want %d", i, req.MeasurementQuantities[i], q)
		}
	}
}

func TestECIDMeasurementTerminationRoundTrip(t *testing.T) {
	pdu, err := BuildECIDMeasurementTerminationCommand(3, 12)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	parsed, err := ParsePDU(pdu)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if parsed.Kind != KindECIDMeasurementTerminationCommand {
		t.Fatalf("kind = %d, want termination", parsed.Kind)
	}

	if parsed.Termination.ESMLCUEMeasurementID != 3 || parsed.Termination.ENBUEMeasurementID != 12 {
		t.Fatalf("ids = %d/%d, want 3/12", parsed.Termination.ESMLCUEMeasurementID, parsed.Termination.ENBUEMeasurementID)
	}
}

func TestECIDMeasurementInitiationResponseRoundTrip(t *testing.T) {
	ta1 := int64(1234)
	aoa := int64(359)
	plmn := []byte{0x00, 0xf1, 0x10}
	tac := []byte{0x00, 0x07}

	result := &ECIDResult{
		ServingCell:    ECGI{PLMNIdentity: plmn, EUTRACellID: 0x0abcde1},
		ServingCellTAC: tac,
		APPosition: &APPosition{
			LatitudeSign:           0,
			Latitude:               3515000,
			Longitude:              -5698000,
			DirectionOfAltitude:    0,
			Altitude:               100,
			UncertaintySemiMajor:   20,
			UncertaintySemiMinor:   15,
			OrientationOfMajorAxis: 30,
			UncertaintyAltitude:    10,
			Confidence:             67,
		},
		AngleOfArrival:     &aoa,
		TimingAdvanceType1: &ta1,
		RSRP:               []RSRPItem{{PCI: 42, EARFCN: 1850, ValueRSRP: 80}},
		RSRQ:               []RSRQItem{{PCI: 42, EARFCN: 1850, ValueRSRQ: 20}},
	}

	pdu, err := BuildECIDMeasurementInitiationResponse(7, 5, result)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	parsed, err := ParsePDU(pdu)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if parsed.Kind != KindECIDMeasurementInitiationResponse {
		t.Fatalf("kind = %d, want response", parsed.Kind)
	}

	resp := parsed.Response
	if resp.ESMLCUEMeasurementID != 7 || resp.ENBUEMeasurementID != 5 {
		t.Fatalf("ids = %d/%d, want 7/5", resp.ESMLCUEMeasurementID, resp.ENBUEMeasurementID)
	}

	got := resp.Result
	if got == nil {
		t.Fatal("result is nil")
	}

	if !bytes.Equal(got.ServingCell.PLMNIdentity, plmn) {
		t.Fatalf("plmn = %x, want %x", got.ServingCell.PLMNIdentity, plmn)
	}

	if got.ServingCell.EUTRACellID != 0x0abcde1 {
		t.Fatalf("cell id = %x, want 0abcde1", got.ServingCell.EUTRACellID)
	}

	if !bytes.Equal(got.ServingCellTAC, tac) {
		t.Fatalf("tac = %x, want %x", got.ServingCellTAC, tac)
	}

	if got.APPosition == nil || got.APPosition.Latitude != 3515000 || got.APPosition.Longitude != -5698000 {
		t.Fatalf("ap position mismatch: %+v", got.APPosition)
	}

	if got.APPosition.Confidence != 67 {
		t.Fatalf("confidence = %d, want 67", got.APPosition.Confidence)
	}

	if got.AngleOfArrival == nil || *got.AngleOfArrival != aoa {
		t.Fatalf("aoa = %v, want %d", got.AngleOfArrival, aoa)
	}

	if got.TimingAdvanceType1 == nil || *got.TimingAdvanceType1 != ta1 {
		t.Fatalf("ta1 = %v, want %d", got.TimingAdvanceType1, ta1)
	}

	if len(got.RSRP) != 1 || got.RSRP[0].ValueRSRP != 80 || got.RSRP[0].PCI != 42 {
		t.Fatalf("rsrp mismatch: %+v", got.RSRP)
	}

	if len(got.RSRQ) != 1 || got.RSRQ[0].ValueRSRQ != 20 {
		t.Fatalf("rsrq mismatch: %+v", got.RSRQ)
	}
}

func TestECIDMeasurementInitiationResponseNoResult(t *testing.T) {
	pdu, err := BuildECIDMeasurementInitiationResponse(1, 2, nil)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	parsed, err := ParsePDU(pdu)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if parsed.Response.Result != nil {
		t.Fatalf("result = %+v, want nil", parsed.Response.Result)
	}
}

func TestECIDMeasurementInitiationFailureRoundTrip(t *testing.T) {
	pdu, err := BuildECIDMeasurementInitiationFailure(9, Cause{Group: CauseGroupRadioNetwork, Value: 1})
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	parsed, err := ParsePDU(pdu)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if parsed.Kind != KindECIDMeasurementInitiationFailure {
		t.Fatalf("kind = %d, want failure", parsed.Kind)
	}

	if parsed.Failure.ESMLCUEMeasurementID != 9 {
		t.Fatalf("meas id = %d, want 9", parsed.Failure.ESMLCUEMeasurementID)
	}

	if parsed.Failure.Cause.Group != CauseGroupRadioNetwork || parsed.Failure.Cause.Value != 1 {
		t.Fatalf("cause = %+v, want radioNetwork/1", parsed.Failure.Cause)
	}
}
