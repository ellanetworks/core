// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package diameternode

import (
	"net/netip"
	"slices"
	"testing"
)

func TestToDiameterPeersDialsAnAddressOrAName(t *testing.T) {
	peers := toDiameterPeers([]PeerConfig{
		{ID: "ipv4", Address: "192.0.2.10", Port: 3868},
		{ID: "mapped", Address: "::ffff:192.0.2.11", Port: 3868},
		{ID: "name", Address: "smsc-0.smsc.ella.svc.cluster.local", Port: 3869},
	})

	if p := peers[0]; p.Name != "" || !slices.Equal(p.Addresses, []netip.Addr{netip.MustParseAddr("192.0.2.10")}) || p.Dial.Port != 3868 {
		t.Fatalf("ipv4 peer = %+v", p)
	}

	if p := peers[1]; p.Name != "" || !slices.Equal(p.Addresses, []netip.Addr{netip.MustParseAddr("192.0.2.11")}) {
		t.Fatalf("mapped peer = %+v", p)
	}

	if p := peers[2]; p.Name != "smsc-0.smsc.ella.svc.cluster.local" || len(p.Addresses) != 0 || p.Dial.Port != 3869 {
		t.Fatalf("named peer = %+v", p)
	}
}
