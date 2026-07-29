// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import "testing"

func TestBCDIdentityRoundTrip(t *testing.T) {
	// IMSI type = 1, IMEISV type = 5 (TS 24.008 §10.5.1.4).
	for _, tc := range []struct {
		digits string
		typ    uint8
	}{
		{"001010000000001", 1},  // 15-digit IMSI (odd)
		{"0010100000000012", 5}, // 16-digit IMEISV (even)
		{"1", 1},                // single digit
		{"12345678901234", 3},   // 14-digit IMEI (even)
	} {
		enc, err := EncodeBCDIdentity(tc.digits, tc.typ)
		if err != nil {
			t.Fatalf("EncodeBCDIdentity(%q) error: %v", tc.digits, err)
		}

		if got := enc[0] & 0x07; got != tc.typ {
			t.Errorf("%q: type-of-identity = %d, want %d", tc.digits, got, tc.typ)
		}

		got, err := DecodeBCDIdentity(enc)
		if err != nil {
			t.Errorf("DecodeBCDIdentity(% x): %v", enc, err)
			continue
		}

		if got != tc.digits {
			t.Errorf("round-trip %q = %q", tc.digits, got)
		}
	}
}

func TestEncodeBCDIdentityErrors(t *testing.T) {
	if _, err := EncodeBCDIdentity("", 1); err == nil {
		t.Error("empty digits: want error")
	}

	if _, err := EncodeBCDIdentity("12a45", 1); err == nil {
		t.Error("non-digit: want error")
	}
}
