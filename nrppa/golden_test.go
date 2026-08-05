// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nrppa

import (
	"encoding/hex"
	"math"
	"testing"

	"github.com/ellanetworks/core/per"
)

// Golden vectors produced by asn1tools from the spec-verbatim ASN.1, not by
// this codec, so a symmetric encode+decode defect cannot pass unnoticed the way
// it would in a round-trip test. lppa/golden_test.go pins TS 36.455 the same
// way; the values below differ from it wherever TS 38.455 differs.
const (
	goldenAPPosition        = "1035a27880290e30000064283c785430"
	goldenNGRANCGI          = "0000f1104000066c0000"
	goldenMeasurementResult = "6000f1104000066c00000000071035a27880290e30000064283c785430c001672004d26000002a20073aa10000002a20073a50"
	goldenResultSSRSRP      = "0700000180059f1a0000f11000066c00076000bb"
	goldenCauseRadioNetwork = "10"
	goldenCauseProtocol     = "50"
	goldenCauseMisc         = "80"
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

func sampleNRCellID() uint64 { return 0x00066c000 }

func sampleNGRANCGI() NGRANCGI {
	cell := sampleNRCellID()

	return NGRANCGI{PLMNIdentity: []byte{0x00, 0xf1, 0x10}, NRCellIdentity: &cell}
}

func sampleResult() *ECIDResult {
	aoa := int64(359)
	ta1 := int64(1234)

	return &ECIDResult{
		ServingCell:        sampleNGRANCGI(),
		ServingCellTAC:     []byte{0x00, 0x00, 0x07},
		APPosition:         sampleAPPosition(),
		AngleOfArrival:     &aoa,
		TimingAdvanceType1: &ta1,
		RSRP:               []RSRPItem{{PCI: 42, EARFCN: 1850, ValueRSRP: 80}},
		RSRQ:               []RSRQItem{{PCI: 42, EARFCN: 1850, ValueRSRQ: 20}},
	}
}

func sampleSSRSRP() []SSRSRPItem {
	value := int64(59)
	cell := sampleNRCellID()

	return []SSRSRPItem{{
		NRPCI:   1,
		NRARFCN: 368410,
		CGI:     &CGINR{PLMNIdentity: []byte{0x00, 0xf1, 0x10}, NRCellIdentity: cell},
		Value:   &value,
		PerSSB:  []SSBResultItem{{SSBIndex: 1, Value: 59}},
	}}
}

func encodeHex(t *testing.T, enc func(*per.Writer) error) string {
	t.Helper()

	w := per.NewWriter()

	if err := enc(w); err != nil {
		t.Fatalf("encode: %v", err)
	}

	return hex.EncodeToString(perAlignedBytes(w))
}

// The NG-RAN position carries an iE-Extensions field the E-UTRAN position of
// TS 36.455 §9.2.1 does not, so its preamble is one bit wider than LPPa's.
func TestGoldenAPPosition(t *testing.T) {
	got := encodeHex(t, func(w *per.Writer) error { return encAPPosition(w, sampleAPPosition()) })
	if got != goldenAPPosition {
		t.Fatalf("NG-RANAccessPointPosition\n got=%s\nwant=%s", got, goldenAPPosition)
	}
}

func TestGoldenNGRANCGI(t *testing.T) {
	got := encodeHex(t, func(w *per.Writer) error { return encNGRANCGI(w, sampleNGRANCGI()) })
	if got != goldenNGRANCGI {
		t.Fatalf("NG-RAN-CGI\n got=%s\nwant=%s", got, goldenNGRANCGI)
	}
}

func TestGoldenMeasurementResult(t *testing.T) {
	got := encodeHex(t, encMeasurementResult(sampleResult()))
	if got != goldenMeasurementResult {
		t.Fatalf("E-CID-MeasurementResult\n got=%s\nwant=%s", got, goldenMeasurementResult)
	}
}

func TestGoldenResultSSRSRP(t *testing.T) {
	items := sampleSSRSRP()

	got := encodeHex(t, func(w *per.Writer) error {
		return encRSResultList(w, len(items), func(w *per.Writer, i int) error { return encSSRSRPItem(w, items[i]) })
	})
	if got != goldenResultSSRSRP {
		t.Fatalf("ResultSS-RSRP\n got=%s\nwant=%s", got, goldenResultSSRSRP)
	}
}

// Unlike TS 36.455, the NRPPa Cause CHOICE has no extension marker, so the group
// index is a plain two-bit field with no leading extension bit.
func TestGoldenCause(t *testing.T) {
	for _, tt := range []struct {
		name  string
		cause Cause
		want  string
	}{
		{"radioNetwork", Cause{Group: CauseGroupRadioNetwork, Value: 2}, goldenCauseRadioNetwork},
		{"protocol", Cause{Group: CauseGroupProtocol, Value: 4}, goldenCauseProtocol},
		{"misc", Cause{Group: CauseGroupMisc, Value: 0}, goldenCauseMisc},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := encodeHex(t, encCause(tt.cause)); got != tt.want {
				t.Fatalf("Cause\n got=%s\nwant=%s", got, tt.want)
			}
		})
	}
}

// TestDecodeGoldenMeasurementResult holds the decoder to the same external
// oracle as the encoder.
func TestDecodeGoldenMeasurementResult(t *testing.T) {
	raw, err := hex.DecodeString(goldenMeasurementResult)
	if err != nil {
		t.Fatal(err)
	}

	res, err := decodeMeasurementResult(per.NewReader(raw))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if res.ServingCell.NRCellIdentity == nil || *res.ServingCell.NRCellIdentity != sampleNRCellID() {
		t.Fatalf("cell id = %v", res.ServingCell.NRCellIdentity)
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

	if len(res.RSRP) != 1 || res.RSRP[0].ValueRSRP != 80 || res.RSRP[0].EARFCN != 1850 {
		t.Fatalf("rsrp = %+v", res.RSRP)
	}

	if len(res.RSRQ) != 1 || res.RSRQ[0].ValueRSRQ != 20 || res.RSRQ[0].EARFCN != 1850 {
		t.Fatalf("rsrq = %+v", res.RSRQ)
	}
}

// TestAPPositionDegrees checks the TS 23.032 conversion against hand-computed
// degrees (37.71°N, -122.26°E).
func TestAPPositionDegrees(t *testing.T) {
	raw, err := hex.DecodeString(goldenAPPosition)
	if err != nil {
		t.Fatal(err)
	}

	p, err := decodeAPPosition(per.NewReader(raw))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if math.Abs(p.LatitudeDegrees-37.71) > 0.01 {
		t.Errorf("latitude = %f, want ~37.71", p.LatitudeDegrees)
	}

	if math.Abs(p.LongitudeDegrees-(-122.26)) > 0.01 {
		t.Errorf("longitude = %f, want ~-122.26", p.LongitudeDegrees)
	}
}

// realGNBResponse is the exact NRPPa-PDU (87 bytes) captured from a real gNB
// E-CID Measurement Initiation Response (gnb_ngap.pcap, frame 62): a
// SuccessfulOutcome carrying an E-CIDMeasurementInitiationResponse whose
// measuredResults holds an id=32 ResultSS-RSRP and an id=33 ResultSS-RSRQ.
//
// Its ResultSS-RSRP octets are byte-identical to goldenResultSSRSRP above, so
// the capture and asn1tools agree independently of this codec. The two entries
// differ in exactly one octet, the per-SSB value; their per-cell values are the
// same bits and both decode to 59.
var realGNBResponse = []byte{
	0x20, 0x02, 0x00, 0x00, 0x01, 0x51, 0x00, 0x00,
	0x03, 0x00, 0x02, 0x00, 0x01, 0x00, 0x00, 0x06,
	0x00, 0x01, 0x00, 0x00, 0x07, 0x40, 0x40, 0x20,
	0x00, 0xf1, 0x10, 0x40, 0x00, 0x06, 0x6c, 0x00,
	0x00, 0x00, 0x00, 0x01, 0x06, 0x80, 0x00, 0x20,
	0x40, 0x14, 0x07, 0x00, 0x00, 0x01, 0x80, 0x05,
	0x9f, 0x1a, 0x00, 0x00, 0xf1, 0x10, 0x00, 0x06,
	0x6c, 0x00, 0x07, 0x60, 0x00, 0xbb, 0xa0, 0x00,
	0x21, 0x40, 0x14, 0x07, 0x00, 0x00, 0x01, 0x80,
	0x05, 0x9f, 0x1a, 0x00, 0x00, 0xf1, 0x10, 0x00,
	0x06, 0x6c, 0x00, 0x07, 0x60, 0x00, 0xc2,
}

func TestParseRealGNBResponse(t *testing.T) {
	parsed, err := ParsePDU(realGNBResponse)
	if err != nil {
		t.Fatalf("ParsePDU: %v", err)
	}

	if parsed.Kind != KindECIDMeasurementInitiationResponse || parsed.Response == nil {
		t.Fatalf("kind = %d, response = %v, want an initiation response", parsed.Kind, parsed.Response)
	}

	resp := parsed.Response
	if resp.LMFUEMeasurementID != 1 || resp.RANUEMeasurementID != 1 {
		t.Errorf("measurement ids = (%d, %d), want (1, 1)", resp.LMFUEMeasurementID, resp.RANUEMeasurementID)
	}

	if resp.Result == nil {
		t.Fatal("no E-CID measurement result")
	}

	res := resp.Result
	if got := res.ServingCell.PLMNIdentity; hex.EncodeToString(got) != "00f110" {
		t.Errorf("serving cell PLMN = %x, want 00f110", got)
	}

	if res.ServingCell.NRCellIdentity == nil || *res.ServingCell.NRCellIdentity != sampleNRCellID() {
		t.Errorf("serving cell = %+v, want NR cell %x", res.ServingCell, sampleNRCellID())
	}

	if len(res.SSRSRP) != 1 {
		t.Fatalf("SS-RSRP items = %d, want 1", len(res.SSRSRP))
	}

	assertNRItem(t, "SS-RSRP", res.SSRSRP[0].NRPCI, res.SSRSRP[0].NRARFCN, res.SSRSRP[0].CGI,
		res.SSRSRP[0].Value, res.SSRSRP[0].PerSSB, 59)

	if len(res.SSRSRQ) != 1 {
		t.Fatalf("SS-RSRQ items = %d, want 1", len(res.SSRSRQ))
	}

	assertNRItem(t, "SS-RSRQ", res.SSRSRQ[0].NRPCI, res.SSRSRQ[0].NRARFCN, res.SSRSRQ[0].CGI,
		res.SSRSRQ[0].Value, res.SSRSRQ[0].PerSSB, 66)
}

// assertNRItem checks the fields both NR per-cell results carry. wantPerSSB is
// the only value that differs between the two entries in the capture.
func assertNRItem(t *testing.T, name string, pci, arfcn int64, cgi *CGINR, value *int64, perSSB []SSBResultItem, wantPerSSB int64) {
	t.Helper()

	if pci != 1 {
		t.Errorf("%s PCI = %d, want 1", name, pci)
	}

	if arfcn != 368410 {
		t.Errorf("%s NR-ARFCN = %d, want 368410", name, arfcn)
	}

	if cgi == nil || cgi.NRCellIdentity != sampleNRCellID() {
		t.Errorf("%s CGI-NR = %+v, want cell %x", name, cgi, sampleNRCellID())
	}

	if value == nil || *value != 59 {
		t.Errorf("%s per-cell value = %v, want 59", name, value)
	}

	if len(perSSB) != 1 || perSSB[0].SSBIndex != 1 || perSSB[0].Value != wantPerSSB {
		t.Errorf("%s per-SSB = %+v, want one entry {1, %d}", name, perSSB, wantPerSSB)
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

// TestForwardCompatUnknownIE requires an unmodeled IE to be skipped rather than
// to fail the parse.
func TestForwardCompatUnknownIE(t *testing.T) {
	fields := []ieField{
		{id: idLMFUEMeasurementID, crit: CriticalityReject, enc: encMeasurementID(7)},
		{id: idRANUEMeasurementID, crit: CriticalityReject, enc: encMeasurementID(5)},
		{id: 999, crit: CriticalityIgnore, enc: func(w *per.Writer) error { _ = w.WriteOctets([]byte{0xde, 0xad}); return nil }},
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

	if parsed.Response.LMFUEMeasurementID != 7 || parsed.Response.RANUEMeasurementID != 5 {
		t.Fatalf("ids = %d/%d", parsed.Response.LMFUEMeasurementID, parsed.Response.RANUEMeasurementID)
	}
}

// A measuredResults choice-Extension naming a quantity this codec does not
// model must be stepped over, leaving the entries after it readable. NRPPa
// carries every NR quantity this way, so a release adding one must not cost the
// rest of the report.
func TestForwardCompatUnknownMeasuredResult(t *testing.T) {
	ta := int64(1234)

	w := per.NewWriter()

	writeSeqPreamble(w, false, []bool{false, true, false})

	if err := encNGRANCGI(w, sampleNGRANCGI()); err != nil {
		t.Fatal(err)
	}

	if err := per.EncodeOctetString(w, per.Aligned, 3, 3, true, true, false, []byte{0x00, 0x00, 0x07}); err != nil {
		t.Fatal(err)
	}

	if err := per.EncodeConstrainedWholeNumber(w, per.Aligned, 1, maxNoMeas, 2); err != nil {
		t.Fatal(err)
	}

	// An id NRPPa does not assign, so this codec cannot model it.
	unknown := encMeasuredChoiceExtension(4242, func(w *per.Writer) error {
		return w.WriteOctets([]byte{0xde, 0xad, 0xbe, 0xef})
	})
	if err := unknown(w); err != nil {
		t.Fatal(err)
	}

	if err := encMeasuredChoiceInt(w, measuredTimingAdvanceType1EUTRA, ta, 0, 7690); err != nil {
		t.Fatal(err)
	}

	res, err := decodeMeasurementResult(per.NewReader(perAlignedBytes(w)))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if res.TimingAdvanceType1 == nil || *res.TimingAdvanceType1 != ta {
		t.Fatalf("entry after the unknown one = %v, want %d", res.TimingAdvanceType1, ta)
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

		if parsed.Termination.LMFUEMeasurementID != measID {
			t.Fatalf("measID %d round-trip = %d", measID, parsed.Termination.LMFUEMeasurementID)
		}
	}

	maxCell := uint64(0xFFFFFFFFF) // 36 bits
	ta2 := int64(7690)
	aoa := int64(719)
	ssMax := int64(127)
	res := &ECIDResult{
		ServingCell:        NGRANCGI{PLMNIdentity: []byte{0x00, 0xf1, 0x10}, NRCellIdentity: &maxCell},
		ServingCellTAC:     []byte{0xff, 0xff, 0xff},
		AngleOfArrival:     &aoa,
		TimingAdvanceType2: &ta2,
		RSRP:               []RSRPItem{{PCI: 503, EARFCN: 262143, ValueRSRP: 97}},
		RSRQ:               []RSRQItem{{PCI: 0, EARFCN: 0, ValueRSRQ: 34}},
		SSRSRP:             []SSRSRPItem{{NRPCI: 1007, NRARFCN: 3279165, Value: &ssMax}},
		AoA:                &AoAResult{AzimuthRaw: 3599, ZenithRaw: &[]int64{1799}[0]},
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
	if got.ServingCell.NRCellIdentity == nil || *got.ServingCell.NRCellIdentity != maxCell {
		t.Fatalf("max cell id = %v", got.ServingCell.NRCellIdentity)
	}

	if got.RSRP[0].ValueRSRP != 97 || got.RSRP[0].PCI != 503 || got.RSRP[0].EARFCN != 262143 {
		t.Fatalf("boundary rsrp = %+v", got.RSRP)
	}

	if got.RSRQ[0].ValueRSRQ != 34 {
		t.Fatalf("boundary rsrq = %+v", got.RSRQ)
	}

	if got.TimingAdvanceType2 == nil || *got.TimingAdvanceType2 != 7690 {
		t.Fatalf("ta2 = %v", got.TimingAdvanceType2)
	}

	if len(got.SSRSRP) != 1 || got.SSRSRP[0].NRPCI != 1007 || got.SSRSRP[0].NRARFCN != 3279165 {
		t.Fatalf("boundary ss-rsrp = %+v", got.SSRSRP)
	}

	if got.AoA == nil || got.AoA.AzimuthRaw != 3599 || got.AoA.ZenithRaw == nil || *got.AoA.ZenithRaw != 1799 {
		t.Fatalf("boundary aoa = %+v", got.AoA)
	}
}

// A per-item cell identity is optional on every result type, and E-UTRA items
// name a CGI-EUTRA where NR items name a CGI-NR (TS 38.455 §9.2.11, §9.2.12).
func TestResultWithPerItemCGI(t *testing.T) {
	value := int64(50)
	res := &ECIDResult{
		ServingCell:    sampleNGRANCGI(),
		ServingCellTAC: []byte{0x00, 0x00, 0x01},
		RSRP: []RSRPItem{{
			PCI: 1, EARFCN: 100, ValueRSRP: 50,
			CGI: &CGIEUTRA{PLMNIdentity: []byte{0x00, 0xf1, 0x10}, EUTRACellID: 0x0abcde1},
		}},
		SSRSRP: []SSRSRPItem{{
			NRPCI: 1, NRARFCN: 100, Value: &value,
			CGI: &CGINR{PLMNIdentity: []byte{0x00, 0xf1, 0x10}, NRCellIdentity: sampleNRCellID()},
		}},
	}

	pdu, err := BuildECIDMeasurementInitiationResponse(2, 3, res)
	if err != nil {
		t.Fatal(err)
	}

	parsed, err := ParsePDU(pdu)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	got := parsed.Response.Result
	if got.RSRP[0].CGI == nil || got.RSRP[0].CGI.EUTRACellID != 0x0abcde1 {
		t.Errorf("per-item CGI-EUTRA = %+v", got.RSRP[0].CGI)
	}

	if got.SSRSRP[0].CGI == nil || got.SSRSRP[0].CGI.NRCellIdentity != sampleNRCellID() {
		t.Errorf("per-item CGI-NR = %+v", got.SSRSRP[0].CGI)
	}
}

func TestValidationErrors(t *testing.T) {
	for _, tt := range []struct {
		name string
		call func() error
	}{
		{"measID 0", func() error {
			_, err := BuildECIDMeasurementInitiationRequest(0, []MeasurementQuantityValue{MeasCellID})

			return err
		}},
		{"measID 16", func() error {
			_, err := BuildECIDMeasurementInitiationRequest(16, []MeasurementQuantityValue{MeasCellID})

			return err
		}},
		{"empty quantities", func() error {
			_, err := BuildECIDMeasurementInitiationRequest(1, nil)

			return err
		}},
		{"quantity out of range", func() error {
			_, err := BuildECIDMeasurementInitiationRequest(1, []MeasurementQuantityValue{99})

			return err
		}},
		{"ranMeasID 16", func() error {
			_, err := BuildECIDMeasurementTerminationCommand(1, 16)

			return err
		}},
		{"TAC not three octets", func() error {
			_, err := BuildECIDMeasurementInitiationResponse(1, 1, &ECIDResult{
				ServingCell:    sampleNGRANCGI(),
				ServingCellTAC: []byte{0x00, 0x01},
			})

			return err
		}},
		{"serving cell with no identity", func() error {
			_, err := BuildECIDMeasurementInitiationResponse(1, 1, &ECIDResult{
				ServingCell:    NGRANCGI{PLMNIdentity: []byte{0x00, 0xf1, 0x10}},
				ServingCellTAC: []byte{0x00, 0x00, 0x01},
			})

			return err
		}},
		{"serving cell with both identities", func() error {
			nr, eutra := sampleNRCellID(), uint64(1)
			_, err := BuildECIDMeasurementInitiationResponse(1, 1, &ECIDResult{
				ServingCell:    NGRANCGI{PLMNIdentity: []byte{0x00, 0xf1, 0x10}, NRCellIdentity: &nr, EUTRACellID: &eutra},
				ServingCellTAC: []byte{0x00, 0x00, 0x01},
			})

			return err
		}},
		{"cause group out of range", func() error {
			_, err := BuildECIDMeasurementInitiationFailure(1, Cause{Group: CauseGroupChoiceExtension})

			return err
		}},
		{"cause value outside its group", func() error {
			_, err := BuildECIDMeasurementInitiationFailure(1, Cause{Group: CauseGroupMisc, Value: 1})

			return err
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.call(); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func FuzzParsePDU(f *testing.F) {
	f.Add(realGNBResponse)

	for _, seed := range []string{goldenMeasurementResult, "00", "6000"} {
		if b, err := hex.DecodeString(seed); err == nil {
			f.Add(b)
		}
	}

	f.Fuzz(func(_ *testing.T, data []byte) {
		_, _ = ParsePDU(data)
	})
}

// TS 38.455 gives MeasurementQuantitiesValue six root values and seven
// extension additions; TS 36.455 has only the six. An extension addition is
// encoded as a normally-small index in a one-octet open type, not as a wider
// root field, so pin both forms against the oracle and round-trip them.
func TestGoldenMeasurementQuantities(t *testing.T) {
	for _, tt := range []struct {
		name string
		q    MeasurementQuantityValue
		want string
	}{
		{"cell-ID (root)", MeasCellID, "00"},
		{"rSRQ (last root)", MeasRSRQ, "14"},
		{"sS-RSRP (first extension)", MeasSSRSRP, "2000"},
		{"uE-Rx-Tx-Time-Diff (last extension)", MeasUERxTxTimeDiff, "2180"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := encodeHex(t, func(w *per.Writer) error {
				writeSeqPreamble(w, false, []bool{false})

				return per.EncodeEnumerated(w, per.Aligned, measurementQuantityRootCount, true, int64(tt.q))
			})
			if got != tt.want {
				t.Fatalf("MeasurementQuantities-Item\n got=%s\nwant=%s", got, tt.want)
			}

			pdu, err := BuildECIDMeasurementInitiationRequest(1, []MeasurementQuantityValue{tt.q})
			if err != nil {
				t.Fatalf("build: %v", err)
			}

			parsed, err := ParsePDU(pdu)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}

			if got := parsed.Request.MeasurementQuantities; len(got) != 1 || got[0] != tt.q {
				t.Fatalf("round-trip = %v, want [%d]", got, tt.q)
			}
		})
	}
}

// goldenEveryMeasuredResult carries one entry of every MeasuredResultsValue
// alternative NRPPa defines: the five E-UTRA root ones LPPa shares, then each NR
// quantity in its choice-Extension container. It pins the whole measured-results
// encoder in a single vector, so a change to any one of them shows up here.
const goldenEveryMeasuredResult = "6000f1104000066c00000000071035a27880290e30000064283c785432c000012000024000036000000120073aa10000000120073a5280002040140700000180059f1a0000f11000066c00076000bba0002140090200000180059f1a84a00022400702000001000178a00023400702000001000132a00024400540038401c2a0005e40020064a00076400204b0"

func everyMeasuredResult() *ECIDResult {
	ssrsrqValue, csirsrpValue, csirsrqValue := int64(66), int64(60), int64(25)

	return &ECIDResult{
		ServingCell:        sampleNGRANCGI(),
		ServingCellTAC:     []byte{0x00, 0x00, 0x07},
		APPosition:         sampleAPPosition(),
		AngleOfArrival:     &[]int64{1}[0],
		TimingAdvanceType1: &[]int64{2}[0],
		TimingAdvanceType2: &[]int64{3}[0],
		RSRP:               []RSRPItem{{PCI: 1, EARFCN: 1850, ValueRSRP: 80}},
		RSRQ:               []RSRQItem{{PCI: 1, EARFCN: 1850, ValueRSRQ: 20}},
		SSRSRP:             sampleSSRSRP(),
		SSRSRQ:             []SSRSRQItem{{NRPCI: 1, NRARFCN: 368410, Value: &ssrsrqValue}},
		CSIRSRP:            []CSIRSRPItem{{NRPCI: 1, NRARFCN: 1, Value: &csirsrpValue}},
		CSIRSRQ:            []CSIRSRQItem{{NRPCI: 1, NRARFCN: 1, Value: &csirsrqValue}},
		AoA:                &AoAResult{AzimuthRaw: 900, ZenithRaw: &[]int64{450}[0]},
		NRTimingAdvance:    &[]int64{100}[0],
		UERxTxTimeDiff:     &[]int64{1200}[0],
	}
}

func TestGoldenEveryMeasuredResult(t *testing.T) {
	got := encodeHex(t, encMeasurementResult(everyMeasuredResult()))
	if got != goldenEveryMeasuredResult {
		t.Fatalf("E-CID-MeasurementResult with every quantity\n got=%s\nwant=%s", got, goldenEveryMeasuredResult)
	}
}

// The decoder must recover every quantity from the same oracle bytes.
func TestDecodeGoldenEveryMeasuredResult(t *testing.T) {
	raw, err := hex.DecodeString(goldenEveryMeasuredResult)
	if err != nil {
		t.Fatal(err)
	}

	res, err := decodeMeasurementResult(per.NewReader(raw))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	for _, c := range []struct {
		name string
		ok   bool
	}{
		{"angleOfArrival-EUTRA", res.AngleOfArrival != nil && *res.AngleOfArrival == 1},
		{"timingAdvanceType1-EUTRA", res.TimingAdvanceType1 != nil && *res.TimingAdvanceType1 == 2},
		{"timingAdvanceType2-EUTRA", res.TimingAdvanceType2 != nil && *res.TimingAdvanceType2 == 3},
		{"resultRSRP-EUTRA", len(res.RSRP) == 1 && res.RSRP[0].ValueRSRP == 80},
		{"resultRSRQ-EUTRA", len(res.RSRQ) == 1 && res.RSRQ[0].ValueRSRQ == 20},
		{"resultSS-RSRP", len(res.SSRSRP) == 1 && res.SSRSRP[0].Value != nil && *res.SSRSRP[0].Value == 59},
		{"resultSS-RSRQ", len(res.SSRSRQ) == 1 && res.SSRSRQ[0].Value != nil && *res.SSRSRQ[0].Value == 66},
		{"resultCSI-RSRP", len(res.CSIRSRP) == 1 && res.CSIRSRP[0].Value != nil && *res.CSIRSRP[0].Value == 60},
		{"resultCSI-RSRQ", len(res.CSIRSRQ) == 1 && res.CSIRSRQ[0].Value != nil && *res.CSIRSRQ[0].Value == 25},
		{"angleOfArrivalNR", res.AoA != nil && res.AoA.AzimuthRaw == 900 && res.AoA.ZenithRaw != nil},
		{"nR-TADV", res.NRTimingAdvance != nil && *res.NRTimingAdvance == 100},
		{"uE-Rx-Tx-Time-Diff", res.UERxTxTimeDiff != nil && *res.UERxTxTimeDiff == 1200},
	} {
		if !c.ok {
			t.Errorf("%s did not decode", c.name)
		}
	}
}
