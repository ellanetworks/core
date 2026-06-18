// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"testing"

	"github.com/ellanetworks/core/s1ap"
)

func TestPLMNOctets(t *testing.T) {
	tests := []struct {
		mcc, mnc string
		want     s1ap.PLMNIdentity
	}{
		{"001", "01", s1ap.PLMNIdentity{0x00, 0xf1, 0x10}},  // 2-digit MNC
		{"310", "260", s1ap.PLMNIdentity{0x13, 0x20, 0x06}}, // 3-digit MNC
		{"208", "93", s1ap.PLMNIdentity{0x02, 0xf8, 0x39}},  // 2-digit MNC
	}

	for _, tc := range tests {
		got, err := plmnOctets(tc.mcc, tc.mnc)
		if err != nil {
			t.Fatalf("%s/%s: %v", tc.mcc, tc.mnc, err)
		}

		if got != tc.want {
			t.Fatalf("plmnOctets(%s,%s) = % x, want % x", tc.mcc, tc.mnc, got, tc.want)
		}
	}

	if _, err := plmnOctets("1", "01"); err == nil {
		t.Fatal("expected error for malformed MCC")
	}
}

func TestParseTAC(t *testing.T) {
	tests := []struct {
		in   string
		want uint16
	}{
		{"1", 1},
		{"7", 7},
		{"0x1a", 0x1a},
		{"65535", 65535},
	}

	for _, tc := range tests {
		got, err := parseTAC(tc.in)
		if err != nil || got != tc.want {
			t.Fatalf("parseTAC(%q) = %d, %v; want %d", tc.in, got, err, tc.want)
		}
	}

	if _, err := parseTAC(""); err == nil {
		t.Fatal("expected error for empty TAC")
	}
}
