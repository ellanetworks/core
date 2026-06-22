// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"testing"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/s1ap"
)

func TestEncodePLMN(t *testing.T) {
	cases := []struct {
		mcc, mnc string
		want     s1ap.PLMNIdentity
	}{
		{"001", "01", s1ap.PLMNIdentity{0x00, 0xf1, 0x10}},  // 2-digit MNC
		{"310", "260", s1ap.PLMNIdentity{0x13, 0x20, 0x06}}, // 3-digit MNC
	}
	for _, c := range cases {
		got, err := encodePLMN(models.PlmnID{Mcc: c.mcc, Mnc: c.mnc})
		if err != nil {
			t.Fatalf("%s/%s: %v", c.mcc, c.mnc, err)
		}

		if got != c.want {
			t.Fatalf("%s/%s: got % x, want % x", c.mcc, c.mnc, got, c.want)
		}
	}
}

func TestEncodePLMNInvalid(t *testing.T) {
	if _, err := encodePLMN(models.PlmnID{Mcc: "1", Mnc: "01"}); err == nil {
		t.Fatal("expected error for malformed MCC")
	}
}

// TestDecodePLMNRoundTrip confirms decodePLMN inverts encodePLMN for both 2- and
// 3-digit MNCs (TS 23.003).
func TestDecodePLMNRoundTrip(t *testing.T) {
	cases := []models.PlmnID{
		{Mcc: "999", Mnc: "01"},
		{Mcc: "001", Mnc: "01"},
		{Mcc: "310", Mnc: "260"},
		{Mcc: "208", Mnc: "10"},
	}

	for _, want := range cases {
		t.Run(want.Mcc+"-"+want.Mnc, func(t *testing.T) {
			encoded, err := encodePLMN(want)
			if err != nil {
				t.Fatal(err)
			}

			if got := decodePLMN(encoded); got != want {
				t.Errorf("decodePLMN round-trip: got %+v, want %+v", got, want)
			}
		})
	}
}

// TestENBSupportedTAIs confirms an S1 Setup Request's Supported TAs flatten into
// one TAI per (broadcast PLMN, TAC) pair (TS 36.413 §8.7.3.2).
func TestENBSupportedTAIs(t *testing.T) {
	plmnA, err := encodePLMN(models.PlmnID{Mcc: "999", Mnc: "01"})
	if err != nil {
		t.Fatal(err)
	}

	plmnB, err := encodePLMN(models.PlmnID{Mcc: "001", Mnc: "01"})
	if err != nil {
		t.Fatal(err)
	}

	tas := s1ap.SupportedTAs{
		{TAC: 1, BroadcastPLMNs: s1ap.BPLMNs{plmnA, plmnB}},
		{TAC: 7, BroadcastPLMNs: s1ap.BPLMNs{plmnA}},
	}

	want := []ENBTAI{
		{PlmnID: models.PlmnID{Mcc: "999", Mnc: "01"}, TAC: 1},
		{PlmnID: models.PlmnID{Mcc: "001", Mnc: "01"}, TAC: 1},
		{PlmnID: models.PlmnID{Mcc: "999", Mnc: "01"}, TAC: 7},
	}

	got := enbSupportedTAIs(tas)
	if len(got) != len(want) {
		t.Fatalf("enbSupportedTAIs: got %d TAIs, want %d", len(got), len(want))
	}

	for i := range want {
		if got[i] != want[i] {
			t.Errorf("enbSupportedTAIs[%d]: got %+v, want %+v", i, got[i], want[i])
		}
	}
}
