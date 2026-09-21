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

func stubNeighNetlink(t *testing.T, routes map[string][]netlink.Route) *[]netlink.Neigh {
	t.Helper()

	oldRouteGet, oldNeighSet := neighRouteGetWithOptions, neighSet

	t.Cleanup(func() {
		neighRouteGetWithOptions, neighSet = oldRouteGet, oldNeighSet
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
	})

	if err := AddNeighbour(context.Background(), netip.MustParseAddr("10.45.0.5"), ""); err != nil {
		t.Fatalf("AddNeighbour: %v", err)
	}

	if len(*seeded) != 1 || (*seeded)[0].LinkIndex != 6 || !(*seeded)[0].IP.Equal(gw) {
		t.Errorf("expected gateway %s seeded on link 6, got %+v", gw, *seeded)
	}
}

func TestAddNeighbour_VRFTableRoute(t *testing.T) {
	gw := net.ParseIP("10.3.0.1")

	seeded := stubNeighNetlink(t, map[string][]netlink.Route{
		"10.45.0.5|up-vrf": {{LinkIndex: 2, Gw: gw}},
	})

	if err := AddNeighbour(context.Background(), netip.MustParseAddr("10.45.0.5"), "up-vrf"); err != nil {
		t.Fatalf("AddNeighbour: %v", err)
	}

	if len(*seeded) != 1 || (*seeded)[0].LinkIndex != 2 || !(*seeded)[0].IP.Equal(gw) {
		t.Errorf("expected gateway %s seeded on link 2 from the up-vrf table, got %+v", gw, *seeded)
	}
}

func TestAddNeighbour_VRFLookupIgnoresMainTable(t *testing.T) {
	mgmtGW := net.ParseIP("10.200.0.254")

	seeded := stubNeighNetlink(t, map[string][]netlink.Route{
		"10.45.0.5|": {{LinkIndex: 5, Gw: mgmtGW}},
	})

	err := AddNeighbour(context.Background(), netip.MustParseAddr("10.45.0.5"), "up-vrf")
	if !errors.Is(err, errNoRouteToNeighbour) {
		t.Errorf("expected errNoRouteToNeighbour, got %v", err)
	}

	if len(*seeded) != 0 {
		t.Errorf("expected no neighbours seeded from the main table, got %+v", *seeded)
	}
}

func TestAddNeighbour_MainLookupIgnoresVRFTables(t *testing.T) {
	gw := net.ParseIP("10.3.0.1")

	seeded := stubNeighNetlink(t, map[string][]netlink.Route{
		"10.45.0.5|up-vrf": {{LinkIndex: 2, Gw: gw}},
	})

	err := AddNeighbour(context.Background(), netip.MustParseAddr("10.45.0.5"), "")
	if !errors.Is(err, errNoRouteToNeighbour) {
		t.Errorf("expected errNoRouteToNeighbour, got %v", err)
	}

	if len(*seeded) != 0 {
		t.Errorf("expected no neighbours seeded from a VRF table, got %+v", *seeded)
	}
}

func TestAddNeighbour_NoRouteAnywhere(t *testing.T) {
	seeded := stubNeighNetlink(t, nil)

	err := AddNeighbour(context.Background(), netip.MustParseAddr("10.45.0.5"), "up-vrf")
	if !errors.Is(err, errNoRouteToNeighbour) {
		t.Errorf("expected errNoRouteToNeighbour, got %v", err)
	}

	if len(*seeded) != 0 {
		t.Errorf("expected no neighbours seeded, got %+v", *seeded)
	}
}

func TestAddNeighbour_RouteGetError(t *testing.T) {
	oldRouteGet := neighRouteGetWithOptions

	t.Cleanup(func() { neighRouteGetWithOptions = oldRouteGet })

	errRouteGet := errors.New("netlink: route get failed")

	neighRouteGetWithOptions = func(dst net.IP, opts *netlink.RouteGetOptions) ([]netlink.Route, error) {
		return nil, errRouteGet
	}

	err := AddNeighbour(context.Background(), netip.MustParseAddr("10.45.0.5"), "up-vrf")
	if !errors.Is(err, errRouteGet) {
		t.Fatalf("expected wrapped route get error, got %v", err)
	}

	if errors.Is(err, errNoRouteToNeighbour) {
		t.Errorf("a netlink failure must not be reported as errNoRouteToNeighbour")
	}
}

func TestAddNeighbour_UnreachableIsNotAnError(t *testing.T) {
	for _, errno := range []error{unix.EHOSTUNREACH, unix.ENETUNREACH} {
		oldRouteGet := neighRouteGetWithOptions

		neighRouteGetWithOptions = func(dst net.IP, opts *netlink.RouteGetOptions) ([]netlink.Route, error) {
			return nil, errno
		}

		err := AddNeighbour(context.Background(), netip.MustParseAddr("10.45.0.5"), "up-vrf")

		neighRouteGetWithOptions = oldRouteGet

		if !errors.Is(err, errNoRouteToNeighbour) {
			t.Errorf("%v: expected errNoRouteToNeighbour, got %v", errno, err)
		}
	}
}

func TestRoutesToNeighbourSelectsTable(t *testing.T) {
	oldRouteGet := neighRouteGetWithOptions

	t.Cleanup(func() { neighRouteGetWithOptions = oldRouteGet })

	var gotOpts *netlink.RouteGetOptions

	neighRouteGetWithOptions = func(dst net.IP, opts *netlink.RouteGetOptions) ([]netlink.Route, error) {
		gotOpts = opts

		return nil, nil
	}

	for _, tc := range []struct{ device, wantVRF string }{
		{"", ""},
		{"up-vrf", "up-vrf"},
	} {
		if _, err := routesToNeighbour(net.ParseIP("10.45.0.5"), tc.device); err != nil {
			t.Fatalf("routesToNeighbour(%q): %v", tc.device, err)
		}

		if gotOpts == nil || !gotOpts.FIBMatch {
			t.Fatalf("routesToNeighbour(%q) did not request a FIB match", tc.device)
		}

		if gotOpts.VrfName != tc.wantVRF {
			t.Errorf("routesToNeighbour(%q) looked up VRF %q, want %q", tc.device, gotOpts.VrfName, tc.wantVRF)
		}
	}
}
