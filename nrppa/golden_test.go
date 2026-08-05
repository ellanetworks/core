// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nrppa

import (
	"testing"
)

// realGNBResponse is the exact NRPPa-PDU (87 bytes) captured from a real gNB
// E-CID Measurement Initiation Response (gnb_ngap.pcap, frame 62). It is ground
// truth for the wire format: a SuccessfulOutcome carrying an
// E-CIDMeasurementInitiationResponse with 3 IEs,
//
//	IE[0] id=2  LMF-UE-Measurement-ID = 1
//	IE[1] id=6  RAN-UE-Measurement-ID = 1
//	IE[2] id=7  E-CID-Measurement-Result, whose measuredResults carries two
//	            choice-Extension entries, id=32 ResultSS-RSRP and
//	            id=33 ResultSS-RSRQ.
//
// The two entries differ in exactly one octet, which is the per-SSB measurement
// value; their per-cell values are the same bits and both decode to 59. The
// expectations below were derived from the octets by hand, field by field.
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

// TestParseRealGNBResponse pins this codec against a capture no part of this
// repository produced, so a symmetric encode+decode defect cannot pass
// unnoticed the way it would in a round-trip test.
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
	if got := res.ServingCell.PLMNIdentity; len(got) != 3 || got[0] != 0x00 || got[1] != 0xf1 || got[2] != 0x10 {
		t.Errorf("serving cell PLMN = %x, want 00f110", got)
	}

	if res.ServingCell.NRCellIdentity == nil {
		t.Fatalf("serving cell = %+v, want an NR cell identity", res.ServingCell)
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

	if cgi == nil || cgi.NRCellIdentity != 6733824 {
		t.Errorf("%s CGI-NR = %+v, want cell 6733824", name, cgi)
	}

	if value == nil || *value != 59 {
		t.Errorf("%s per-cell value = %v, want 59", name, value)
	}

	if len(perSSB) != 1 || perSSB[0].SSBIndex != 1 || perSSB[0].Value != wantPerSSB {
		t.Errorf("%s per-SSB = %+v, want one entry {1, %d}", name, perSSB, wantPerSSB)
	}
}
