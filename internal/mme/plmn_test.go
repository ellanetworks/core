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
