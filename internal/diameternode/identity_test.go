// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package diameternode_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/diameternode"
)

func TestDiameterRealm(t *testing.T) {
	cases := []struct {
		mcc, mnc string
		want     string
	}{
		{"001", "01", "epc.mnc001.mcc001.3gppnetwork.org"},
		{"310", "410", "epc.mnc410.mcc310.3gppnetwork.org"},
		{"234", "15", "epc.mnc015.mcc234.3gppnetwork.org"},
	}

	for _, tc := range cases {
		got, err := diameternode.DiameterRealm(tc.mcc, tc.mnc)
		if err != nil {
			t.Fatalf("DiameterRealm(%s, %s): %v", tc.mcc, tc.mnc, err)
		}

		if got != tc.want {
			t.Fatalf("DiameterRealm(%s, %s) = %q, want %q", tc.mcc, tc.mnc, got, tc.want)
		}
	}
}

func TestDiameterRealmRejectsInvalidPLMN(t *testing.T) {
	for _, plmn := range [][2]string{{"01", "01"}, {"001", "1"}, {"001", "0001"}, {"0a1", "01"}, {"001", ""}} {
		if _, err := diameternode.DiameterRealm(plmn[0], plmn[1]); err == nil {
			t.Fatalf("DiameterRealm(%s, %s) accepted an invalid PLMN", plmn[0], plmn[1])
		}
	}
}

func TestMMEHost(t *testing.T) {
	got := diameternode.MMEHost("epc.mnc001.mcc001.3gppnetwork.org", 0x8204, 0x1)
	want := "mmec01.mmegi8204.mme.epc.mnc001.mcc001.3gppnetwork.org"

	if got != want {
		t.Fatalf("MMEHost = %q, want %q", got, want)
	}

	got = diameternode.MMEHost("epc.mnc001.mcc001.3gppnetwork.org", 0x4, 0xab)
	want = "mmecab.mmegi0004.mme.epc.mnc001.mcc001.3gppnetwork.org"

	if got != want {
		t.Fatalf("MMEHost = %q, want %q", got, want)
	}
}
