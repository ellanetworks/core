// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package models

import "testing"

func TestReportValueConversions(t *testing.T) {
	cases := []struct {
		name    string
		convert func(int64) float64
		in      int64
		want    float64
	}{
		{"E-UTRA RSRP floor", EUTRARSRPDBm, 0, -141},
		{"E-UTRA RSRP", EUTRARSRPDBm, 80, -61},
		{"E-UTRA RSRP ceiling", EUTRARSRPDBm, 97, -44},
		{"E-UTRA RSRQ floor", EUTRARSRQDB, 0, -20},
		{"E-UTRA RSRQ", EUTRARSRQDB, 25, -7.5},
		{"E-UTRA RSRQ ceiling", EUTRARSRQDB, 34, -3},
		{"E-UTRA extended RSRP below range", EUTRARSRPExtendedDBm, -17, -157},
		{"E-UTRA extended RSRP floor", EUTRARSRPExtendedDBm, -16, -156},
		{"E-UTRA extended RSRP ceiling", EUTRARSRPExtendedDBm, -1, -141},
		{"E-UTRA extended RSRQ below range", EUTRARSRQExtendedDB, -30, -34.5},
		{"E-UTRA extended RSRQ floor", EUTRARSRQExtendedDB, -29, -34},
		{"E-UTRA extended RSRQ low ceiling", EUTRARSRQExtendedDB, -1, -20},
		{"E-UTRA extended RSRQ legacy floor", EUTRARSRQExtendedDB, 0, -20},
		{"E-UTRA extended RSRQ legacy", EUTRARSRQExtendedDB, 20, -10},
		{"E-UTRA extended RSRQ legacy ceiling", EUTRARSRQExtendedDB, 34, -3},
		{"E-UTRA extended RSRQ high floor", EUTRARSRQExtendedDB, 35, -3},
		{"E-UTRA extended RSRQ high ceiling", EUTRARSRQExtendedDB, 46, 2.5},
		{"NR RSRQ below range", NRRSRQDB, 0, -43.5},
		{"NR RSRQ floor", NRRSRQDB, 1, -43},
		{"NR RSRQ", NRRSRQDB, 66, -10.5},
		{"NR RSRQ ceiling", NRRSRQDB, 127, 20},
	}

	for _, tc := range cases {
		if got := tc.convert(tc.in); got != tc.want {
			t.Errorf("%s: %d -> %v, want %v", tc.name, tc.in, got, tc.want)
		}
	}
}

func TestNRRSRPReportMapping(t *testing.T) {
	cases := []struct {
		in   int64
		want float64
	}{
		{0, -157},
		{1, -156},
		{101, -56},
		{126, -31},
	}

	for _, tc := range cases {
		if got := NRRSRPDBm(tc.in); got == nil || *got != tc.want {
			t.Errorf("NRRSRPDBm(%d) = %v, want %v", tc.in, got, tc.want)
		}
	}

	if got := NRRSRPDBm(127); got != nil {
		t.Errorf("NRRSRPDBm(127) = %v, want nil: RSRP_127 is a threshold value, not a measurement", *got)
	}
}
