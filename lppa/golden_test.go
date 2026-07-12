// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lppa

import (
	"encoding/hex"
	"math"
	"testing"

	"github.com/ellanetworks/core/s1ap/aper"
)

// Golden vectors are the aligned-PER encoding of TS 36.455 IE values produced by
// an independent encoder (asn1tools, from the spec-verbatim ASN.1), not by this
// codec. They pin the wire bytes so a symmetric encode+decode defect cannot pass
// unnoticed the way a round-trip test would.
const (
	goldenAPPosition        = "2035a27880290e30000064283c785430"
	goldenECGI              = "0000f1100abcde10"
	goldenMeasurementResult = "6000f1100abcde10007235a27880290e30000064283c785430c001671004d23000002a00073a504000002a00073a28"
)

func sampleAPPosition() *APPosition {
	return &APPosition{
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
	}
}

func sampleECGI() ECGI {
	return ECGI{PLMNIdentity: []byte{0x00, 0xf1, 0x10}, EUTRACellID: 0x0abcde1}
}

func sampleResult() *ECIDResult {
	aoa := int64(359)
	ta1 := int64(1234)

	return &ECIDResult{
		ServingCell:        sampleECGI(),
		ServingCellTAC:     []byte{0x00, 0x07},
		APPosition:         sampleAPPosition(),
		AngleOfArrival:     &aoa,
		TimingAdvanceType1: &ta1,
		RSRP:               []RSRPItem{{PCI: 42, EARFCN: 1850, ValueRSRP: 80}},
		RSRQ:               []RSRQItem{{PCI: 42, EARFCN: 1850, ValueRSRQ: 20}},
	}
}

func encodeHex(t *testing.T, enc func(*aper.Writer) error) string {
	t.Helper()

	var w aper.Writer

	if err := enc(&w); err != nil {
		t.Fatalf("encode: %v", err)
	}

	return hex.EncodeToString(w.Bytes())
}

func TestGoldenAPPosition(t *testing.T) {
	got := encodeHex(t, func(w *aper.Writer) error { return encAPPosition(w, sampleAPPosition()) })
	if got != goldenAPPosition {
		t.Fatalf("E-UTRANAccessPointPosition\n got=%s\nwant=%s", got, goldenAPPosition)
	}
}

func TestGoldenECGI(t *testing.T) {
	got := encodeHex(t, func(w *aper.Writer) error { return encECGI(w, sampleECGI()) })
	if got != goldenECGI {
		t.Fatalf("ECGI\n got=%s\nwant=%s", got, goldenECGI)
	}
}

func TestGoldenMeasurementResult(t *testing.T) {
	got := encodeHex(t, encMeasurementResult(sampleResult()))
	if got != goldenMeasurementResult {
		t.Fatalf("E-CID-MeasurementResult\n got=%s\nwant=%s", got, goldenMeasurementResult)
	}
}

// TestDecodeGoldenMeasurementResult decodes the independent reference bytes, so
// the decoder is validated against the same external oracle as the encoder.
func TestDecodeGoldenMeasurementResult(t *testing.T) {
	raw, err := hex.DecodeString(goldenMeasurementResult)
	if err != nil {
		t.Fatal(err)
	}

	res, err := decodeMeasurementResult(aper.NewReader(raw))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if res.ServingCell.EUTRACellID != 0x0abcde1 {
		t.Fatalf("cell id = %x", res.ServingCell.EUTRACellID)
	}

	if res.APPosition == nil || res.APPosition.Latitude != 3515000 || res.APPosition.Longitude != -5698000 {
		t.Fatalf("ap position = %+v", res.APPosition)
	}

	if res.AngleOfArrival == nil || *res.AngleOfArrival != 359 {
		t.Fatalf("aoa = %v", res.AngleOfArrival)
	}

	if res.TimingAdvanceType1 == nil || *res.TimingAdvanceType1 != 1234 {
		t.Fatalf("ta1 = %v", res.TimingAdvanceType1)
	}

	if len(res.RSRP) != 1 || res.RSRP[0].ValueRSRP != 80 || len(res.RSRQ) != 1 || res.RSRQ[0].ValueRSRQ != 20 {
		t.Fatalf("rsrp/rsrq = %+v / %+v", res.RSRP, res.RSRQ)
	}
}

// TestAPPositionDegrees checks the TS 23.032 decimal-degree conversion against a
// hand-computed value (37.71°N, -122.26°E ≈ San Francisco).
func TestAPPositionDegrees(t *testing.T) {
	raw, err := hex.DecodeString(goldenAPPosition)
	if err != nil {
		t.Fatal(err)
	}

	ap, err := decodeAPPosition(aper.NewReader(raw))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if math.Abs(ap.LatitudeDegrees-37.7118) > 0.001 {
		t.Fatalf("lat = %f, want ~37.7118", ap.LatitudeDegrees)
	}

	if math.Abs(ap.LongitudeDegrees-(-122.2658)) > 0.001 {
		t.Fatalf("lon = %f, want ~-122.2658", ap.LongitudeDegrees)
	}
}

func TestParseFailureIndication(t *testing.T) {
	fields := []ieField{
		{id: idESMLCUEMeasurementID, crit: CriticalityReject, enc: encMeasurementID(4)},
		{id: idENBUEMeasurementID, crit: CriticalityReject, enc: encMeasurementID(9)},
		{id: idCause, crit: CriticalityIgnore, enc: encCause(Cause{Group: CauseGroupRadioNetwork, Value: 2})},
	}

	body, err := encodeMessageBody(fields)
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := marshalPDU(pduInitiatingMessage, ProcECIDMeasurementFailureIndication, body)
	if err != nil {
		t.Fatal(err)
	}

	parsed, err := ParsePDU(pdu)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if parsed.Kind != KindECIDMeasurementFailureIndication {
		t.Fatalf("kind = %d", parsed.Kind)
	}

	fi := parsed.FailureIndication
	if fi.ESMLCUEMeasurementID != 4 || fi.ENBUEMeasurementID != 9 {
		t.Fatalf("ids = %d/%d", fi.ESMLCUEMeasurementID, fi.ENBUEMeasurementID)
	}

	if fi.Cause.Group != CauseGroupRadioNetwork || fi.Cause.Value != 2 {
		t.Fatalf("cause = %+v", fi.Cause)
	}
}

func TestParseUnknownProcedure(t *testing.T) {
	pdu, err := marshalPDU(pduInitiatingMessage, ProcErrorIndication, nil)
	if err != nil {
		t.Fatal(err)
	}

	parsed, err := ParsePDU(pdu)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if parsed.Kind != KindUnknown {
		t.Fatalf("kind = %d, want KindUnknown", parsed.Kind)
	}
}

// TestForwardCompatUnknownIE decodes a Response carrying an IE this codec does
// not model; it must be skipped and the message parse successfully.
func TestForwardCompatUnknownIE(t *testing.T) {
	fields := []ieField{
		{id: idESMLCUEMeasurementID, crit: CriticalityReject, enc: encMeasurementID(7)},
		{id: idENBUEMeasurementID, crit: CriticalityReject, enc: encMeasurementID(5)},
		{id: 999, crit: CriticalityIgnore, enc: func(w *aper.Writer) error { w.WriteOctets([]byte{0xde, 0xad}); return nil }},
	}

	body, err := encodeMessageBody(fields)
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := marshalPDU(pduSuccessfulOutcome, ProcECIDMeasurementInitiation, body)
	if err != nil {
		t.Fatal(err)
	}

	parsed, err := ParsePDU(pdu)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if parsed.Kind != KindECIDMeasurementInitiationResponse {
		t.Fatalf("kind = %d", parsed.Kind)
	}

	if parsed.Response.ESMLCUEMeasurementID != 7 || parsed.Response.ENBUEMeasurementID != 5 {
		t.Fatalf("ids = %d/%d", parsed.Response.ESMLCUEMeasurementID, parsed.Response.ENBUEMeasurementID)
	}
}

func TestBoundaryValuesRoundTrip(t *testing.T) {
	for _, measID := range []int64{1, 15} {
		pdu, err := BuildECIDMeasurementTerminationCommand(measID, measID)
		if err != nil {
			t.Fatalf("measID %d: %v", measID, err)
		}

		parsed, err := ParsePDU(pdu)
		if err != nil {
			t.Fatalf("measID %d: %v", measID, err)
		}

		if parsed.Termination.ESMLCUEMeasurementID != measID {
			t.Fatalf("measID %d round-trip = %d", measID, parsed.Termination.ESMLCUEMeasurementID)
		}
	}

	maxCell := uint64(0x0FFFFFFF)
	tp1 := int64(7690)
	res := &ECIDResult{
		ServingCell:        ECGI{PLMNIdentity: []byte{0x00, 0xf1, 0x10}, EUTRACellID: maxCell},
		ServingCellTAC:     []byte{0xff, 0xff},
		AngleOfArrival:     func() *int64 { v := int64(719); return &v }(),
		TimingAdvanceType2: &tp1,
		RSRP:               []RSRPItem{{PCI: 503, EARFCN: 65535, ValueRSRP: 97}},
		RSRQ:               []RSRQItem{{PCI: 0, EARFCN: 0, ValueRSRQ: 34}},
	}

	pdu, err := BuildECIDMeasurementInitiationResponse(15, 1, res)
	if err != nil {
		t.Fatal(err)
	}

	parsed, err := ParsePDU(pdu)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	got := parsed.Response.Result
	if got.ServingCell.EUTRACellID != maxCell {
		t.Fatalf("max cell id = %x", got.ServingCell.EUTRACellID)
	}

	if got.RSRP[0].ValueRSRP != 97 || got.RSRQ[0].ValueRSRQ != 34 || got.RSRP[0].PCI != 503 {
		t.Fatalf("boundary rsrp/rsrq = %+v / %+v", got.RSRP, got.RSRQ)
	}

	if got.TimingAdvanceType2 == nil || *got.TimingAdvanceType2 != 7690 {
		t.Fatalf("ta2 = %v", got.TimingAdvanceType2)
	}
}

// TestResultWithECGIPerItem exercises the optional per-item eCGI in a RSRP item.
func TestResultWithECGIPerItem(t *testing.T) {
	ecgi := sampleECGI()
	res := &ECIDResult{
		ServingCell:    sampleECGI(),
		ServingCellTAC: []byte{0x00, 0x01},
		RSRP:           []RSRPItem{{PCI: 1, EARFCN: 100, ECGI: &ecgi, ValueRSRP: 50}},
	}

	pdu, err := BuildECIDMeasurementInitiationResponse(2, 3, res)
	if err != nil {
		t.Fatal(err)
	}

	parsed, err := ParsePDU(pdu)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	item := parsed.Response.Result.RSRP[0]
	if item.ECGI == nil || item.ECGI.EUTRACellID != 0x0abcde1 {
		t.Fatalf("per-item ecgi = %+v", item.ECGI)
	}
}

func TestValidationErrors(t *testing.T) {
	if _, err := BuildECIDMeasurementInitiationRequest(0, []MeasurementQuantityValue{MeasCellID}); err == nil {
		t.Fatal("measID 0 should be rejected")
	}

	if _, err := BuildECIDMeasurementInitiationRequest(16, []MeasurementQuantityValue{MeasCellID}); err == nil {
		t.Fatal("measID 16 should be rejected")
	}

	if _, err := BuildECIDMeasurementInitiationRequest(1, nil); err == nil {
		t.Fatal("empty quantities should be rejected")
	}

	if _, err := BuildECIDMeasurementInitiationRequest(1, []MeasurementQuantityValue{99}); err == nil {
		t.Fatal("out-of-range quantity should be rejected")
	}

	if _, err := BuildECIDMeasurementTerminationCommand(1, 16); err == nil {
		t.Fatal("enbMeasID 16 should be rejected")
	}
}

func FuzzParsePDU(f *testing.F) {
	for _, seed := range []string{goldenMeasurementResult, "00", "6000"} {
		if b, err := hex.DecodeString(seed); err == nil {
			f.Add(b)
		}
	}

	f.Fuzz(func(_ *testing.T, data []byte) {
		_, _ = ParsePDU(data)
	})
}
