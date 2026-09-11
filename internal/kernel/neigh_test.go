// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package kernel

import (
	"net"
	"testing"

	"github.com/vishvananda/netlink"
)

func hopsEqual(got []nexthop, want []nexthop) bool {
	if len(got) != len(want) {
		return false
	}

	for i := range got {
		if got[i].ifindex != want[i].ifindex || !got[i].ip.Equal(want[i].ip) {
			return false
		}
	}

	return true
}

func TestNexthopsFromRoutes_DirectlyConnected(t *testing.T) {
	dst := net.ParseIP("33.33.33.7")

	got := nexthopsFromRoutes(dst, []netlink.Route{{LinkIndex: 3}})

	want := []nexthop{{ifindex: 3, ip: dst}}
	if !hopsEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestNexthopsFromRoutes_ViaGateway(t *testing.T) {
	dst := net.ParseIP("10.20.30.40")
	gw := net.ParseIP("192.168.1.1")

	got := nexthopsFromRoutes(dst, []netlink.Route{{LinkIndex: 2, Gw: gw}})

	want := []nexthop{{ifindex: 2, ip: gw}}
	if !hopsEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestNexthopsFromRoutes_ViaRFC5549(t *testing.T) {
	dst := net.ParseIP("10.20.30.40")
	via := &netlink.Via{AddrFamily: netlink.FAMILY_V6, Addr: net.ParseIP("2001:db8::1")}

	got := nexthopsFromRoutes(dst, []netlink.Route{{LinkIndex: 4, Via: via}})

	want := []nexthop{{ifindex: 4, ip: via.Addr}}
	if !hopsEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestNexthopsFromRoutes_GatewayWinsOverVia(t *testing.T) {
	dst := net.ParseIP("10.20.30.40")
	gw := net.ParseIP("192.168.1.1")
	via := &netlink.Via{AddrFamily: netlink.FAMILY_V6, Addr: net.ParseIP("2001:db8::1")}

	got := nexthopsFromRoutes(dst, []netlink.Route{{LinkIndex: 4, Gw: gw, Via: via}})

	want := []nexthop{{ifindex: 4, ip: gw}}
	if !hopsEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestNexthopsFromRoutes_MultiPath(t *testing.T) {
	dst := net.ParseIP("9.9.9.9")
	gw1 := net.ParseIP("10.0.1.2")
	gw2 := net.ParseIP("10.0.2.2")

	got := nexthopsFromRoutes(dst, []netlink.Route{{
		LinkIndex: 99,
		MultiPath: []*netlink.NexthopInfo{
			{LinkIndex: 2, Gw: gw1},
			{LinkIndex: 3, Gw: gw2},
		},
	}})

	want := []nexthop{{ifindex: 2, ip: gw1}, {ifindex: 3, ip: gw2}}
	if !hopsEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestNexthopsFromRoutes_MultiPathDirectlyConnected(t *testing.T) {
	dst := net.ParseIP("33.33.33.7")

	got := nexthopsFromRoutes(dst, []netlink.Route{{
		LinkIndex: 99,
		MultiPath: []*netlink.NexthopInfo{
			{LinkIndex: 2},
			{LinkIndex: 3},
		},
	}})

	want := []nexthop{{ifindex: 2, ip: dst}, {ifindex: 3, ip: dst}}
	if !hopsEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestNexthopsFromRoutes_DeduplicatesIdenticalHops(t *testing.T) {
	dst := net.ParseIP("10.20.30.40")
	gw := net.ParseIP("192.168.1.1")

	got := nexthopsFromRoutes(dst, []netlink.Route{
		{LinkIndex: 2, Gw: gw},
		{LinkIndex: 2, Gw: net.ParseIP("192.168.1.1")},
	})

	want := []nexthop{{ifindex: 2, ip: gw}}
	if !hopsEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestNexthopsFromRoutes_SkipsUnusableLinkIndex(t *testing.T) {
	dst := net.ParseIP("33.33.33.7")

	got := nexthopsFromRoutes(dst, []netlink.Route{{LinkIndex: 0}, {LinkIndex: -1}})
	if len(got) != 0 {
		t.Fatalf("expected no nexthops, got %v", got)
	}
}

func TestNexthopsFromRoutes_NoRoutes(t *testing.T) {
	got := nexthopsFromRoutes(net.ParseIP("33.33.33.7"), nil)
	if len(got) != 0 {
		t.Fatalf("expected no nexthops, got %v", got)
	}
}

func TestNeighbourForIsKernelManaged(t *testing.T) {
	n := neighbourFor(7, net.ParseIP("33.33.33.7"))

	if n.FlagsExt&netlink.NTF_EXT_MANAGED == 0 {
		t.Error("entry must carry NTF_EXT_MANAGED so the kernel auto-refreshes it")
	}

	if n.LinkIndex != 7 {
		t.Errorf("LinkIndex = %d, want 7", n.LinkIndex)
	}
}
