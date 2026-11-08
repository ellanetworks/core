// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package runtime

import (
	"maps"
	"net/netip"
	"slices"
	"testing"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/models"
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

func TestRoutableLeasesSkipsWithheldFamilies(t *testing.T) {
	leases := []db.IPLease{
		testLease("001010000000001", "nat", "10.45.0.1"),
		testLease("001010000000001", "nat", "2001:db8:1::"),
		testLease("001010000000002", "routed", "10.46.0.1"),
		testLease("001010000000003", "deleted", "10.47.0.1"),
		testLease("001010000000004", "instance", "10.48.0.1"),
		testLease("001010000000004", "instance", "2001:db8:4::"),
	}

	got := routableLeases(leases, map[string]db.LeaseWithholding{
		"001010000000001/nat":      {IPv4: true},
		"001010000000002/routed":   {},
		"001010000000004/instance": {IPv4: true, IPv6: true},
	})

	var addrs []string
	for _, l := range got {
		addrs = append(addrs, l.Address().String())
	}

	if want := []string{"2001:db8:1::", "10.46.0.1", "10.47.0.1"}; !slices.Equal(addrs, want) {
		t.Fatalf("routable leases %v, want %v", addrs, want)
	}
}

func TestWithheldOnEverySliceNeedsEverySliceToWithhold(t *testing.T) {
	got := withheldOnEverySlice([]db.SliceDataNetworkForwarding{
		{SliceID: "a", DataNetworkID: "all", NAT: true},
		{SliceID: "b", DataNetworkID: "all", NAT: true},
		{SliceID: "a", DataNetworkID: "mixed", NAT: true},
		{SliceID: "b", DataNetworkID: "mixed", NAT: false},
		{SliceID: "a", DataNetworkID: "none", NAT: false},
		{SliceID: "a", DataNetworkID: "ims", NAT: true, IMS: true},
		{SliceID: "a", DataNetworkID: "instance", NetworkInstance: "ims"},
		{SliceID: "a", DataNetworkID: "split", NetworkInstance: "ims"},
		{SliceID: "b", DataNetworkID: "split", NetworkInstance: models.DefaultNetworkInstanceName},
	})

	want := map[string]db.LeaseWithholding{
		"all":      {IPv4: true},
		"mixed":    {},
		"none":     {},
		"ims":      {},
		"instance": {IPv4: true, IPv6: true},
		"split":    {},
	}

	if !maps.Equal(got, want) {
		t.Fatalf("withheld on every slice = %v, want %v", got, want)
	}
}
