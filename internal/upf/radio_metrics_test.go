// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package upf

import (
	"net/netip"
	"testing"

	"github.com/ellanetworks/core/internal/upf/ebpf"
)

func TestDropsAggregatePerSessionClassAndPerRadio(t *testing.T) {
	hall := Radio{ID: "gnb:001-01:000001@22", Name: "hall"}
	addrA, addrB, addrC := netip.MustParseAddr("10.3.0.11"), netip.MustParseAddr("10.3.0.12"), netip.MustParseAddr("10.3.0.13")

	peers := map[uint16]netip.Addr{1: addrA, 2: addrB, 3: addrC}
	radios := map[netip.Addr]Radio{addrA: hall, addrB: hall}

	drops := map[ebpf.TrafficClassDrop]uint64{
		{ClassID: 7, PeerID: 1, Direction: ebpf.Downlink, Reason: "qer_rate_limit"}: 3,
		{ClassID: 7, PeerID: 2, Direction: ebpf.Downlink, Reason: "qer_rate_limit"}: 2,
		{ClassID: 8, PeerID: 3, Direction: ebpf.Uplink, Reason: "sdf_filter"}:       1,
		{ClassID: 0, PeerID: 1, Direction: ebpf.Uplink, Reason: "sdf_filter"}:       4,
	}

	bySession, byRadio := aggregateDrops(drops, peers, radios)

	if got := bySession[sessionDropKey{7, ebpf.Downlink, "qer_rate_limit"}]; got != 5 {
		t.Errorf("class 7 rate-limit drops = %d, want 5 summed over both radios", got)
	}

	if _, ok := bySession[sessionDropKey{0, ebpf.Uplink, "sdf_filter"}]; ok {
		t.Error("drops without a traffic class were reported per class")
	}

	if got := byRadio[radioDropKey{hall, ebpf.Downlink, "qer_rate_limit"}]; got != 5 {
		t.Errorf("hall rate-limit drops = %d, want 5 summed over its two addresses", got)
	}

	if got := byRadio[radioDropKey{hall, ebpf.Uplink, "sdf_filter"}]; got != 4 {
		t.Errorf("hall uplink drops = %d, want 4 including traffic without a class", got)
	}

	if got := byRadio[radioDropKey{unknownRadio, ebpf.Uplink, "sdf_filter"}]; got != 1 {
		t.Errorf("unidentified radio drops = %d, want 1", got)
	}
}
