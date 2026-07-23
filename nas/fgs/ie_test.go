// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"encoding/hex"
	"testing"
)

func TestSUCIToString(t *testing.T) {
	// SUPI=IMSI, MCC=001 MNC=01, routing 0000, null scheme, HNPKI 0, MSIN 0000000001.
	buf, _ := hex.DecodeString("0100f110000000000000000010")

	suci, plmn, err := SUCIToString(buf)
	if err != nil {
		t.Fatalf("SUCIToString: %v", err)
	}

	if suci != "suci-0-001-01-0000-0-0-0000000001" {
		t.Errorf("suci = %q", suci)
	}

	if plmn != "00101" {
		t.Errorf("plmn = %q", plmn)
	}
}

func TestSUCIToStringNAI(t *testing.T) {
	buf, _ := hex.DecodeString("11aabbcc")

	suci, plmn, err := SUCIToString(buf)
	if err != nil {
		t.Fatalf("SUCIToString: %v", err)
	}

	if suci != "nai-1-aabbcc" || plmn != "" {
		t.Errorf("suci = %q plmn = %q", suci, plmn)
	}
}

func TestSUCIToStringTooShort(t *testing.T) {
	if _, _, err := SUCIToString([]byte{0x01, 0x00}); err == nil {
		t.Error("expected error for too-short SUCI")
	}
}

func TestPEIToStringIMEI(t *testing.T) {
	buf, _ := hex.DecodeString("4b09512430325781") // IMEI 490154203237518 (Luhn-valid)

	pei, err := PEIToString(buf)
	if err != nil {
		t.Fatalf("PEIToString: %v", err)
	}

	if pei != "imei-490154203237518" {
		t.Errorf("pei = %q", pei)
	}
}

func TestPEIToStringInvalidChecksum(t *testing.T) {
	// Same as the valid IMEI but with the last digit changed (9 instead of 8).
	buf, _ := hex.DecodeString("4b09512430325791")

	if _, err := PEIToString(buf); err == nil {
		t.Error("expected error for invalid IMEI checksum")
	}
}

func TestTypeOfIdentity(t *testing.T) {
	cases := map[byte]MobileIdentityType{
		0x00: IdentityNoIdentity,
		0x01: IdentitySUCI,
		0xf2: IdentityGUTI,
		0x0b: IdentityIMEI,
		0x04: IdentitySTMSI,
		0x05: IdentityIMEISV,
	}

	for octet, want := range cases {
		if got := TypeOfIdentity(octet); got != want {
			t.Errorf("TypeOfIdentity(%#x) = %d, want %d", octet, got, want)
		}
	}
}
