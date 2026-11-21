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

	oldRouteGet, oldNeighSet, oldNeighGet := neighRouteGetWithOptions, neighSet, neighGet

	t.Cleanup(func() {
		neighRouteGetWithOptions, neighSet, neighGet = oldRouteGet, oldNeighSet, oldNeighGet
	})

	neighGet = func(int, net.IP) (*netlink.Neigh, error) { return nil, nil }

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

func stubNeighTable(t *testing.T, existing []netlink.Neigh, listErr error) *[]netlink.Neigh {
	t.Helper()

	oldNeighSet, oldNeighGet := neighSet, neighGet

	t.Cleanup(func() {
		neighSet, neighGet = oldNeighSet, oldNeighGet
	})

	neighGet = func(linkIndex int, ip net.IP) (*netlink.Neigh, error) {
		if listErr != nil {
			return nil, listErr
		}

		for _, n := range existing {
			if n.LinkIndex == linkIndex && n.IP.Equal(ip) {
				return &n, nil
			}
		}

		return nil, nil
	}

	seeded := &[]netlink.Neigh{}

	neighSet = func(n *netlink.Neigh) error {
		*seeded = append(*seeded, *n)
		return nil
	}

	return seeded
}

func TestSetNeighbour_KeepsStaticEntries(t *testing.T) {
	gw := net.ParseIP("169.254.1.1")
	gw6 := net.ParseIP("fe80::1")

	for _, tc := range []struct {
		name  string
		ip    net.IP
		state int
	}{
		{"permanent v4", gw, netlink.NUD_PERMANENT},
		{"noarp v4", gw, netlink.NUD_NOARP},
		{"permanent v6", gw6, netlink.NUD_PERMANENT},
	} {
		t.Run(tc.name, func(t *testing.T) {
			seeded := stubNeighTable(t, []netlink.Neigh{
				{LinkIndex: 2, IP: tc.ip, State: tc.state},
			}, nil)

			if err := setNeighbour(2, tc.ip); err != nil {
				t.Fatalf("setNeighbour: %v", err)
			}

			if len(*seeded) != 0 {
				t.Errorf("expected the static entry to be left alone, got %+v", *seeded)
			}
		})
	}
}

func TestSetNeighbour_ManagesDynamicEntry(t *testing.T) {
	gw := net.ParseIP("10.6.0.3")

	seeded := stubNeighTable(t, []netlink.Neigh{
		{LinkIndex: 2, IP: gw, State: netlink.NUD_REACHABLE},
	}, nil)

	if err := setNeighbour(2, gw); err != nil {
		t.Fatalf("setNeighbour: %v", err)
	}

	if len(*seeded) != 1 || (*seeded)[0].FlagsExt != netlink.NTF_EXT_MANAGED || !(*seeded)[0].IP.Equal(gw) {
		t.Errorf("expected a managed entry for %s, got %+v", gw, *seeded)
	}
}

func TestSetNeighbour_CreatesMissingEntry(t *testing.T) {
	gw := net.ParseIP("10.6.0.3")

	seeded := stubNeighTable(t, []netlink.Neigh{
		{LinkIndex: 2, IP: net.ParseIP("10.6.0.4"), State: netlink.NUD_PERMANENT},
		{LinkIndex: 3, IP: gw, State: netlink.NUD_PERMANENT},
	}, nil)

	if err := setNeighbour(2, gw); err != nil {
		t.Fatalf("setNeighbour: %v", err)
	}

	if len(*seeded) != 1 || (*seeded)[0].LinkIndex != 2 || (*seeded)[0].FlagsExt != netlink.NTF_EXT_MANAGED {
		t.Errorf("expected a managed entry for %s on link 2, got %+v", gw, *seeded)
	}
}

func TestSetNeighbour_ListError(t *testing.T) {
	errList := errors.New("netlink: get failed")

	seeded := stubNeighTable(t, nil, errList)

	if err := setNeighbour(2, net.ParseIP("10.6.0.3")); !errors.Is(err, errList) {
		t.Fatalf("expected wrapped list error, got %v", err)
	}

	if len(*seeded) != 0 {
		t.Errorf("expected no entry written when the lookup fails, got %+v", *seeded)
	}
}

func TestNeighFamilyAndAddr(t *testing.T) {
	for _, tc := range []struct {
		ip         string
		wantFamily uint8
		wantLen    int
	}{
		{"169.254.1.1", unix.AF_INET, 4},
		{"fe80::1", unix.AF_INET6, 16},
	} {
		family, addr := neighFamilyAndAddr(net.ParseIP(tc.ip))
		if family != tc.wantFamily || len(addr) != tc.wantLen {
			t.Errorf("%s: got family %d with %d-byte address, want %d with %d bytes", tc.ip, family, len(addr), tc.wantFamily, tc.wantLen)
		}
	}
}
