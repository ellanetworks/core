// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import "testing"

func TestIMSIWireRoundTripAcrossLengths(t *testing.T) {
	for _, imsi := range []string{
		"001011",
		"0010112",
		"0010112345",
		"00101123456",
		"00101975613993",
		"001019756139935",
	} {
		b, err := IMSI(imsi).MarshalBinary()
		if err != nil {
			t.Fatalf("IMSI(%q).MarshalBinary: %v", imsi, err)
		}

		got, err := ParseEPSMobileIdentity(b)
		if err != nil {
			t.Fatalf("ParseEPSMobileIdentity(%q): %v", imsi, err)
		}

		if got.IMSI == nil || string(*got.IMSI) != imsi {
			t.Errorf("round trip of %q gave %+v", imsi, got)
		}
	}
}

func TestIMSIWireOddEvenIndicator(t *testing.T) {
	odd, err := IMSI("0010112345678").MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}

	if odd[0]&0x08 == 0 {
		t.Errorf("13-digit IMSI: odd/even indicator clear, want set")
	}

	even, err := IMSI("001011234567").MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}

	if even[0]&0x08 != 0 {
		t.Errorf("12-digit IMSI: odd/even indicator set, want clear")
	}

	if even[len(even)-1]&0xF0 != 0xF0 {
		t.Errorf("12-digit IMSI: last octet high nibble = %#x, want the 0xF end mark", even[len(even)-1]>>4)
	}
}
