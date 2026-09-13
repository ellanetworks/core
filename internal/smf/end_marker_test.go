// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"net"
	"testing"
)

func TestAnchorBindingSwitchedFrom(t *testing.T) {
	source := AnchorBinding{TEID: 0x11, IPv4: net.ParseIP("10.0.0.2")}
	target := AnchorBinding{TEID: 0x22, IPv4: net.ParseIP("10.0.0.3")}
	retuned := AnchorBinding{TEID: 0x99, IPv4: net.ParseIP("10.0.0.2")}

	tests := []struct {
		name       string
		prev, next AnchorBinding
		want       bool
	}{
		{"handover to another radio", source, target, true},
		{"same radio, new teid", source, retuned, true},
		{"initial bind", AnchorBinding{}, target, false},
		{"unchanged", source, source, false},
		{"going idle", source, AnchorBinding{}, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.next.switchedFrom(tc.prev); got != tc.want {
				t.Errorf("switchedFrom = %v, want %v", got, tc.want)
			}
		})
	}
}
