// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package kernel

import (
	"net"
	"testing"

	"github.com/vishvananda/netlink"
)

func assertHops(t *testing.T, got []nexthop, want []nexthop) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("got %d nexthops, want %d: %v", len(got), len(want), got)
	}

	for i := range got {
		if got[i].ifindex != want[i].ifindex || !got[i].ip.Equal(want[i].ip) {
			t.Fatalf("nexthop %d = {%d %s}, want {%d %s}",
				i, got[i].ifindex, got[i].ip, want[i].ifindex, want[i].ip)
		}
	}
}

func TestNexthopsFromRoutes_DirectlyConnected(t *testing.T) {
	dst := net.ParseIP("33.33.33.7")

	got := nexthopsFromRoutes(dst, []netlink.Route{{LinkIndex: 3}})

	assertHops(t, got, []nexthop{{ifindex: 3, ip: dst}})
}

func TestNexthopsFromRoutes_ViaGateway(t *testing.T) {
	dst := net.ParseIP("10.20.30.40")
	gw := net.ParseIP("192.168.1.1")

	got := nexthopsFromRoutes(dst, []netlink.Route{{LinkIndex: 2, Gw: gw}})

	assertHops(t, got, []nexthop{{ifindex: 2, ip: gw}})
}

func TestNexthopsFromRoutes_ViaRFC5549(t *testing.T) {
	dst := net.ParseIP("10.20.30.40")
	via := &netlink.Via{AddrFamily: netlink.FAMILY_V6, Addr: net.ParseIP("2001:db8::1")}

	got := nexthopsFromRoutes(dst, []netlink.Route{{LinkIndex: 4, Via: via}})

	assertHops(t, got, []nexthop{{ifindex: 4, ip: via.Addr}})
}

func TestNexthopsFromRoutes_MultiPath(t *testing.T) {
	dst := net.ParseIP("9.9.9.9")
	gw1 := net.ParseIP("10.0.1.2")
	gw2 := net.ParseIP("10.0.2.2")

	got := nexthopsFromRoutes(dst, []netlink.Route{{
		MultiPath: []*netlink.NexthopInfo{
			{LinkIndex: 2, Gw: gw1},
			{LinkIndex: 3, Gw: gw2},
		},
	}})

	assertHops(t, got, []nexthop{{ifindex: 2, ip: gw1}, {ifindex: 3, ip: gw2}})
}

func TestNexthopsFromRoutes_NoUsableRoute(t *testing.T) {
	dst := net.ParseIP("33.33.33.7")

	if got := nexthopsFromRoutes(dst, nil); len(got) != 0 {
		t.Fatalf("no routes must yield no nexthops, got %v", got)
	}

	if got := nexthopsFromRoutes(dst, []netlink.Route{{LinkIndex: 0}}); len(got) != 0 {
		t.Fatalf("a route without an output interface must yield no nexthops, got %v", got)
	}
}
