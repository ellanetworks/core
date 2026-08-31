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
		{"001", "01", s1ap.PLMNIdentity{0x00, 0xf1, 0x10}},
		{"310", "260", s1ap.PLMNIdentity{0x13, 0x00, 0x62}},
	}
	for _, c := range cases {
		got, err := EncodePLMN(models.PlmnID{Mcc: c.mcc, Mnc: c.mnc})
		if err != nil {
			t.Fatalf("%s/%s: %v", c.mcc, c.mnc, err)
		}

		if got != c.want {
			t.Fatalf("%s/%s: got % x, want % x", c.mcc, c.mnc, got, c.want)
		}
	}
}

func TestEncodePLMNInvalid(t *testing.T) {
	if _, err := EncodePLMN(models.PlmnID{Mcc: "1", Mnc: "01"}); err == nil {
		t.Fatal("expected error for malformed MCC")
	}
}

func TestDecodePLMNRoundTrip(t *testing.T) {
	cases := []models.PlmnID{
		{Mcc: "999", Mnc: "01"},
		{Mcc: "001", Mnc: "01"},
		{Mcc: "310", Mnc: "260"},
		{Mcc: "208", Mnc: "10"},
	}

	for _, want := range cases {
		t.Run(want.Mcc+"-"+want.Mnc, func(t *testing.T) {
			encoded, err := EncodePLMN(want)
			if err != nil {
				t.Fatal(err)
			}

			if got := decodePLMN(encoded); got != want {
				t.Errorf("decodePLMN round-trip: got %+v, want %+v", got, want)
			}
		})
	}
}

// TS 36.413 §8.7.3.2
func TestENBSupportedTAIs(t *testing.T) {
	plmnA, err := EncodePLMN(models.PlmnID{Mcc: "999", Mnc: "01"})
	if err != nil {
		t.Fatal(err)
	}

	plmnB, err := EncodePLMN(models.PlmnID{Mcc: "001", Mnc: "01"})
	if err != nil {
		t.Fatal(err)
	}

	tas := s1ap.SupportedTAs{
		{TAC: 1, BroadcastPLMNs: s1ap.BPLMNs{plmnA, plmnB}},
		{TAC: 7, BroadcastPLMNs: s1ap.BPLMNs{plmnA}},
	}

	want := []struct{ mcc, mnc, tac string }{
		{"999", "01", "000001"},
		{"001", "01", "000001"},
		{"999", "01", "000007"},
	}

	got := EnbSupportedTAIs(tas)
	if len(got) != len(want) {
		t.Fatalf("EnbSupportedTAIs: got %d TAIs, want %d", len(got), len(want))
	}

	for i := range want {
		g := got[i].Tai
		if g.PlmnID == nil || g.PlmnID.Mcc != want[i].mcc || g.PlmnID.Mnc != want[i].mnc || g.Tac != want[i].tac {
			t.Errorf("EnbSupportedTAIs[%d]: got %+v, want mcc=%s mnc=%s tac=%s", i, got[i], want[i].mcc, want[i].mnc, want[i].tac)
		}
	}
}
