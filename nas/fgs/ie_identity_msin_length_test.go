// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"testing"

	"github.com/ellanetworks/core/nas"
)

func TestSUCIMSINRoundTripAcrossLengths(t *testing.T) {
	cases := []struct {
		plmn nas.PLMN
		msin string
		supi string
	}{
		{nas.PLMN{MCC: "001", MNC: "01"}, "1", "imsi-001011"},
		{nas.PLMN{MCC: "001", MNC: "01"}, "12", "imsi-0010112"},
		{nas.PLMN{MCC: "001", MNC: "01"}, "12345", "imsi-0010112345"},
		{nas.PLMN{MCC: "001", MNC: "01"}, "0000000001", "imsi-001010000000001"},
		{nas.PLMN{MCC: "001", MNC: "001"}, "123", "imsi-001001123"},
		{nas.PLMN{MCC: "001", MNC: "001"}, "975613993", "imsi-001001975613993"},
	}

	for _, tc := range cases {
		scheme, err := nas.EncodeTBCD(tc.msin)
		if err != nil {
			t.Fatalf("EncodeTBCD(%q): %v", tc.msin, err)
		}

		buf, err := SUCI{
			PLMN:             tc.plmn,
			RoutingIndicator: "0000",
			ProtectionScheme: ProtectionSchemeNull,
			SchemeOutput:     scheme,
		}.MarshalBinary()
		if err != nil {
			t.Fatalf("MarshalBinary(%q): %v", tc.msin, err)
		}

		got, err := ParseSUCI(buf)
		if err != nil {
			t.Fatalf("ParseSUCI(%q): %v", tc.msin, err)
		}

		if msin, ok := got.MSIN(); !ok || msin != tc.msin {
			t.Errorf("MSIN of %q = %q (revealed %v)", tc.msin, msin, ok)
		}

		if supi, ok := got.SUPI(); !ok || supi != tc.supi {
			t.Errorf("SUPI for MSIN %q = %q, want %q", tc.msin, supi, tc.supi)
		}
	}
}

func TestSUCIOddMSINCarriesFiller(t *testing.T) {
	odd, err := nas.EncodeTBCD("12345")
	if err != nil {
		t.Fatalf("EncodeTBCD: %v", err)
	}

	if odd[len(odd)-1]&0xF0 != 0xF0 {
		t.Errorf("odd MSIN: last octet high nibble = %#x, want the 0xF filler", odd[len(odd)-1]>>4)
	}

	even, err := nas.EncodeTBCD("123456")
	if err != nil {
		t.Fatalf("EncodeTBCD: %v", err)
	}

	if even[len(even)-1]&0xF0 == 0xF0 {
		t.Errorf("even MSIN: last octet carries a filler it should not")
	}
}
