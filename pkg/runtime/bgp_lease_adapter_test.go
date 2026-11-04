// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package runtime

import (
	"net/netip"
	"slices"
	"testing"

	"github.com/ellanetworks/core/internal/db"
)

func testLease(imsi, dataNetworkID, addr string) db.IPLease {
	bin := netip.MustParseAddr(addr).As16()

	return db.IPLease{IMSI: imsi, PoolID: dataNetworkID, AddressBin: bin[:]}
}

func TestBGPLeasesAttachesFramedRoutesByOwnerAndFamily(t *testing.T) {
	leases := []db.IPLease{
		testLease("001010000000001", "dn-a", "10.45.0.1"),
		testLease("001010000000001", "dn-a", "2001:db8:1::"),
		testLease("001010000000002", "dn-a", "10.45.0.2"),
		testLease("001010000000001", "dn-b", "10.46.0.1"),
		{IMSI: "001010000000003", PoolID: "dn-a"},
	}

	routes := []db.SubscriberFramedRoute{
		{Imsi: "001010000000001", DataNetworkID: "dn-a", Prefix: "192.168.10.0/24"},
		{Imsi: "001010000000001", DataNetworkID: "dn-a", Prefix: "192.168.11.0/24"},
		{Imsi: "001010000000001", DataNetworkID: "dn-a", Prefix: "2001:db8:ff::/48"},
		{Imsi: "001010000000001", DataNetworkID: "dn-c", Prefix: "192.168.12.0/24"},
		{Imsi: "001010000000009", DataNetworkID: "dn-a", Prefix: "192.168.13.0/24"},
		{Imsi: "001010000000002", DataNetworkID: "dn-a", Prefix: "not-a-prefix"},
	}

	got := bgpLeases(leases, routes)

	want := []struct {
		addr   string
		imsi   string
		framed []string
	}{
		{"10.45.0.1", "001010000000001", []string{"192.168.10.0/24", "192.168.11.0/24"}},
		{"2001:db8:1::", "001010000000001", []string{"2001:db8:ff::/48"}},
		{"10.45.0.2", "001010000000002", nil},
		{"10.46.0.1", "001010000000001", nil},
	}

	if len(got) != len(want) {
		t.Fatalf("got %d leases, want %d: %+v", len(got), len(want), got)
	}

	for i, w := range want {
		if got[i].Address != netip.MustParseAddr(w.addr) || got[i].IMSI != w.imsi {
			t.Fatalf("lease %d = %s/%s, want %s/%s", i, got[i].Address, got[i].IMSI, w.addr, w.imsi)
		}

		framed := make([]string, 0, len(got[i].FramedRoutes))
		for _, p := range got[i].FramedRoutes {
			framed = append(framed, p.String())
		}

		if !slices.Equal(framed, w.framed) && (len(framed) != 0 || len(w.framed) != 0) {
			t.Fatalf("lease %d framed routes = %v, want %v", i, framed, w.framed)
		}
	}
}
