// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ebpf

import "testing"

func TestIMSITagRoundTrip(t *testing.T) {
	for _, imsi := range []string{
		"001011",
		"0010112345",
		"00101123456789",
		"001019756139935",
		"999999999999999",
		"000000000000001",
		"001010000000001",
	} {
		tag, err := EncodeIMSITag(imsi)
		if err != nil {
			t.Fatalf("EncodeIMSITag(%q): %v", imsi, err)
		}

		if got := DecodeIMSITag(tag); got != imsi {
			t.Errorf("round trip of %q gave %q", imsi, got)
		}
	}
}

func TestIMSITagDistinguishesLeadingZeros(t *testing.T) {
	short, err := EncodeIMSITag("0010112345")
	if err != nil {
		t.Fatalf("EncodeIMSITag: %v", err)
	}

	long, err := EncodeIMSITag("000000010112345")
	if err != nil {
		t.Fatalf("EncodeIMSITag: %v", err)
	}

	if short == long {
		t.Fatalf("IMSIs of different lengths share tag %d", short)
	}
}

func TestEncodeIMSITagRejectsMalformed(t *testing.T) {
	for _, imsi := range []string{"", "0001011234567890", "00101a"} {
		if _, err := EncodeIMSITag(imsi); err == nil {
			t.Errorf("EncodeIMSITag(%q): encoded without error, want a rejection", imsi)
		}
	}
}

func TestDecodeIMSITagZeroIsEmpty(t *testing.T) {
	if got := DecodeIMSITag(0); got != "" {
		t.Errorf("DecodeIMSITag(0) = %q, want the empty string", got)
	}
}
