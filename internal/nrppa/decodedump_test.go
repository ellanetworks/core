// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nrppa

import (
	"testing"

	"github.com/ellanetworks/core/internal/nrppa/nrppatype"
	"github.com/free5gc/aper"
)

// realGNBResponse is the exact NRPPa-PDU (87 bytes) captured from a real gNB
// E-CID Measurement Initiation Response (see gnb_ngap.pcap, frame 62). It is a
// SuccessfulOutcome carrying an E-CIDMeasurementInitiationResponse with 3 IEs:
//
//	IE[0] id=2  LMF-UE-Measurement-ID = 1
//	IE[1] id=6  RAN-UE-Measurement-ID = 1
//	IE[2] id=7  E-CID-Measurement-Result, whose measuredResults carries two
//	            choice-Extension entries:
//	              - id=32 ResultSS-RSRP: PCI 1, valueSS-RSRP-Cell 59
//	              - id=33 ResultSS-RSRQ: PCI 1, valueSS-RSRQ-Cell 66
//
// The reference decode was verified with Wireshark (tshark -V).
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
	0x06, 0x6c, 0x00, 0x08, 0x40, 0x00, 0xc2,
}

// TestDecode_RealGNBResponse decodes the raw NRPPa-PDU CHOICE and asserts the
// outer structure (SuccessfulOutcome / E-CIDMeasurementInitiationResponse).
func TestDecode_RealGNBResponse(t *testing.T) {
	pdu := &nrppatype.NRPPaPDU{}
	if err := aper.UnmarshalWithParams(realGNBResponse, pdu, "valueExt,valueLB:0,valueUB:2"); err != nil {
		t.Fatalf("UnmarshalWithParams: %v", err)
	}

	if pdu.Present != nrppatype.NRPPaPDUPresentSuccessfulOutcome {
		t.Fatalf("Present = %d, want SuccessfulOutcome (%d)", pdu.Present,
			nrppatype.NRPPaPDUPresentSuccessfulOutcome)
	}

	so := pdu.SuccessfulOutcome
	if so == nil {
		t.Fatal("SuccessfulOutcome is nil")
	}

	if so.ProcedureCode.Value != nrppatype.ProcedureCodeECIDMeasurementInitiation {
		t.Fatalf("ProcedureCode = %d, want %d", so.ProcedureCode.Value,
			nrppatype.ProcedureCodeECIDMeasurementInitiation)
	}

	resp := so.Value.ECIDMeasurementInitiationResponse
	if resp == nil {
		t.Fatal("ECIDMeasurementInitiationResponse is nil")
	}

	if got := len(resp.ProtocolIEs.List); got != 3 {
		t.Fatalf("ProtocolIEs count = %d, want 3", got)
	}
}

// TestParse_RealGNBResponse exercises the high-level parser and asserts the
// decoded SS-RSRP and SS-RSRQ measurement values.
func TestParse_RealGNBResponse(t *testing.T) {
	parsed, err := ParsePDU(realGNBResponse)
	if err != nil {
		t.Fatalf("ParsePDU: %v", err)
	}

	if parsed.Kind != KindECIDMeasurementInitiationResponse {
		t.Fatalf("Kind = %d, want ECIDMeasurementInitiationResponse (%d)",
			parsed.Kind, KindECIDMeasurementInitiationResponse)
	}

	resp := parsed.Response
	if resp == nil {
		t.Fatal("Response is nil")
	}

	if resp.Result == nil {
		t.Fatal("Response.Result is nil")
	}

	res := resp.Result

	// SS-RSRP: one item, PCI 1, value 59.
	if res.ResultSSRSRP == nil || len(res.ResultSSRSRP.Items) != 1 {
		t.Fatalf("ResultSSRSRP = %+v, want 1 item", res.ResultSSRSRP)
	}

	if got := res.ResultSSRSRP.Items[0]; got.NRPCI != 1 || got.Value != 59 {
		t.Fatalf("SS-RSRP item = %+v, want {NRPCI:1 Value:59}", got)
	}

	// SS-RSRQ: one item, PCI 1, value 66.
	if res.ResultSSRSRQ == nil || len(res.ResultSSRSRQ.Items) != 1 {
		t.Fatalf("ResultSSRSRQ = %+v, want 1 item", res.ResultSSRSRQ)
	}

	if got := res.ResultSSRSRQ.Items[0]; got.NRPCI != 1 || got.Value != 66 {
		t.Fatalf("SS-RSRQ item = %+v, want {NRPCI:1 Value:66}", got)
	}
}
