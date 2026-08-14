// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package engine

import (
	"net/netip"
	"testing"
)

func TestUplinkTEID(t *testing.T) {
	for _, tc := range []struct {
		name string
		pdrs []SPDRInfo
		want uint32
	}{
		{"uplink", []SPDRInfo{{PdrID: 1, TeID: 42}}, 42},
		{"downlink only", []SPDRInfo{{PdrID: 2, UEIP: netip.MustParseAddr("10.0.0.1")}}, 0},
		{"mixed", []SPDRInfo{
			{PdrID: 1, TeID: 100},
			{PdrID: 2, UEIP: netip.MustParseAddr("10.0.0.5")},
			{PdrID: 3, UEIP: netip.MustParseAddr("2001:db8::1")},
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
