// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package engine

import (
	"net/netip"
	"testing"
)

// Downlink PDRs match on a UE address and allocate nothing; an unallocated PDR
// has no TEID to report.
func TestUplinkTEID(t *testing.T) {
	for _, tc := range []struct {
		name string
		pdrs []SPDRInfo
		want uint32
	}{
		{"uplink", []SPDRInfo{{PdrID: 1, TeID: 42, Allocated: true}}, 42},
		{"downlink only", []SPDRInfo{{PdrID: 2, UEIP: netip.MustParseAddr("10.0.0.1"), Allocated: true}}, 0},
		{"unallocated skipped", []SPDRInfo{
			{PdrID: 1, TeID: 42, Allocated: false},
			{PdrID: 2, TeID: 43, Allocated: true},
		}, 43},
		{"mixed", []SPDRInfo{
			{PdrID: 1, TeID: 100, Allocated: true},
			{PdrID: 2, UEIP: netip.MustParseAddr("10.0.0.5"), Allocated: true},
			{PdrID: 3, TeID: 200, Allocated: false},
			{PdrID: 4, UEIP: netip.MustParseAddr("2001:db8::1"), Allocated: true},
		}, 100},
		{"none", nil, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := uplinkTEID(tc.pdrs); got != tc.want {
				t.Errorf("uplinkTEID = %d, want %d", got, tc.want)
			}
		})
	}
}
