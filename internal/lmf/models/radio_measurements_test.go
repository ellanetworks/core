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
		{"NR RSRP below range", NRRSRPDBm, 0, -156},
		{"NR RSRP floor", NRRSRPDBm, 1, -156},
		{"NR RSRP", NRRSRPDBm, 101, -56},
		{"NR RSRP ceiling", NRRSRPDBm, 127, -30},
		{"NR RSRQ below range", NRRSRQDB, 0, -43},
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
