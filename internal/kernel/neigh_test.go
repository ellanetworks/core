// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package kernel

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"testing"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
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

func stubNeighNetlink(t *testing.T, routes map[string][]netlink.Route, links []netlink.Link) *[]netlink.Neigh {
	t.Helper()

	oldRouteGet, oldLinkList, oldNeighSet := neighRouteGetWithOptions, neighLinkList, neighSet

	t.Cleanup(func() {
		neighRouteGetWithOptions, neighLinkList, neighSet = oldRouteGet, oldLinkList, oldNeighSet
	})

	neighRouteGetWithOptions = func(dst net.IP, opts *netlink.RouteGetOptions) ([]netlink.Route, error) {
		vrf := ""
		if opts != nil {
			vrf = opts.VrfName
		}

		if routes, ok := routes[dst.String()+"|"+vrf]; ok {
			return routes, nil
		}

		return nil, unix.ENETUNREACH
	}

	neighLinkList = func() ([]netlink.Link, error) {
		return links, nil
	}

	seeded := &[]netlink.Neigh{}

	neighSet = func(n *netlink.Neigh) error {
		*seeded = append(*seeded, *n)
		return nil
	}

	return seeded
}

func TestAddNeighbour_MainTableRoute(t *testing.T) {
	gw := net.ParseIP("10.6.0.3")

	seeded := stubNeighNetlink(t, map[string][]netlink.Route{
		"10.45.0.5|": {{LinkIndex: 6, Gw: gw}},
	}, nil)

	if err := AddNeighbour(context.Background(), netip.MustParseAddr("10.45.0.5")); err != nil {
		t.Fatalf("AddNeighbour: %v", err)
	}

	if len(*seeded) != 1 || (*seeded)[0].LinkIndex != 6 || !(*seeded)[0].IP.Equal(gw) {
		t.Errorf("expected gateway %s seeded on link 6, got %+v", gw, *seeded)
	}
}

func TestAddNeighbour_VRFTableRoute(t *testing.T) {
	upVRF := &netlink.Vrf{LinkAttrs: netlink.LinkAttrs{Name: "up-vrf", Index: 10}, Table: 1001}
	n3 := &netlink.Device{LinkAttrs: netlink.LinkAttrs{Name: "n3", Index: 2, MasterIndex: 10}}
	gw := net.ParseIP("10.3.0.1")

	seeded := stubNeighNetlink(t, map[string][]netlink.Route{
		"10.45.0.5|up-vrf": {{LinkIndex: 2, Gw: gw}},
	}, []netlink.Link{upVRF, n3})

	if err := AddNeighbour(context.Background(), netip.MustParseAddr("10.45.0.5")); err != nil {
		t.Fatalf("AddNeighbour: %v", err)
	}

	if len(*seeded) != 1 || (*seeded)[0].LinkIndex != 2 || !(*seeded)[0].IP.Equal(gw) {
		t.Errorf("expected gateway %s seeded on link 2 via VRF fallback, got %+v", gw, *seeded)
	}
}

func TestAddNeighbour_VRFTableRouteVLAN(t *testing.T) {
	upVRF := &netlink.Vrf{LinkAttrs: netlink.LinkAttrs{Name: "up-vrf", Index: 10}, Table: 1001}
	n3 := &netlink.Device{LinkAttrs: netlink.LinkAttrs{Name: "n3", Index: 2, MasterIndex: 10}}
	vlan := &netlink.Vlan{LinkAttrs: netlink.LinkAttrs{Name: "n3.100", Index: 3, MasterIndex: 10, ParentIndex: 2}, VlanId: 100}
	gw := net.ParseIP("10.3.0.1")

	seeded := stubNeighNetlink(t, map[string][]netlink.Route{
		"10.45.0.5|up-vrf": {{LinkIndex: 3, Gw: gw}},
	}, []netlink.Link{upVRF, n3, vlan})

	if err := AddNeighbour(context.Background(), netip.MustParseAddr("10.45.0.5")); err != nil {
		t.Fatalf("AddNeighbour: %v", err)
	}

	if len(*seeded) != 1 || (*seeded)[0].LinkIndex != 3 || !(*seeded)[0].IP.Equal(gw) {
		t.Errorf("expected gateway %s seeded on VLAN link 3 via VRF fallback, got %+v", gw, *seeded)
	}
}

func TestAddNeighbour_NoRouteAnywhere(t *testing.T) {
	upVRF := &netlink.Vrf{LinkAttrs: netlink.LinkAttrs{Name: "up-vrf", Index: 10}, Table: 1001}

	seeded := stubNeighNetlink(t, nil, []netlink.Link{upVRF})

	err := AddNeighbour(context.Background(), netip.MustParseAddr("10.45.0.5"))
	if !errors.Is(err, errNoRouteToNeighbour) {
		t.Errorf("expected errNoRouteToNeighbour, got %v", err)
	}

	if len(*seeded) != 0 {
		t.Errorf("expected no neighbours seeded, got %+v", *seeded)
	}
}

func TestAddNeighbour_LinkListError(t *testing.T) {
	oldRouteGet, oldLinkList := neighRouteGetWithOptions, neighLinkList

	t.Cleanup(func() {
		neighRouteGetWithOptions, neighLinkList = oldRouteGet, oldLinkList
	})

	neighRouteGetWithOptions = func(dst net.IP, opts *netlink.RouteGetOptions) ([]netlink.Route, error) {
		return nil, unix.ENETUNREACH
	}

	errLinkList := errors.New("netlink: dump failed")

	neighLinkList = func() ([]netlink.Link, error) {
		return nil, errLinkList
	}

	err := AddNeighbour(context.Background(), netip.MustParseAddr("10.45.0.5"))
	if !errors.Is(err, errLinkList) {
		t.Fatalf("expected wrapped link list error, got %v", err)
	}

	if errors.Is(err, errNoRouteToNeighbour) {
		t.Errorf("link list failure must not be reported as errNoRouteToNeighbour")
	}
}
