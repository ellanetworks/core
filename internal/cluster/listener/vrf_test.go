// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package listener

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/netutil"
	"github.com/vishvananda/netlink"
)

type clusterTopo struct {
	links     map[string]netlink.Link
	addrs     map[string]int
	routes    map[string]int
	vrfRoutes map[string]string
	hostnames map[string][]string
}

func stubClusterNetlink(t *testing.T, topo clusterTopo) {
	t.Helper()

	byIndex := map[int]netlink.Link{}
	allLinks := make([]netlink.Link, 0, len(topo.links))

	for _, l := range topo.links {
		byIndex[l.Attrs().Index] = l
		allLinks = append(allLinks, l)
	}

	oldByIndex, oldAddrList := netutil.LinkByIndex, netutil.AddrList
	oldRouteGet, oldRouteGetWithOptions := clusterRouteGet, clusterRouteGetWithOptions
	oldLinkList, oldLookupIP := clusterLinkList, clusterLookupIP

	t.Cleanup(func() {
		netutil.LinkByIndex, netutil.AddrList = oldByIndex, oldAddrList
		clusterRouteGet, clusterRouteGetWithOptions = oldRouteGet, oldRouteGetWithOptions
		clusterLinkList, clusterLookupIP = oldLinkList, oldLookupIP
	})

	netutil.LinkByIndex = func(index int) (netlink.Link, error) {
		if l, ok := byIndex[index]; ok {
			return l, nil
		}

		return nil, errors.New("no such index")
	}

	netutil.AddrList = func(link netlink.Link, family int) ([]netlink.Addr, error) {
		var out []netlink.Addr

		for ipStr, idx := range topo.addrs {
			out = append(out, netlink.Addr{
				IPNet:     &net.IPNet{IP: net.ParseIP(ipStr), Mask: net.CIDRMask(24, 32)},
				LinkIndex: idx,
			})
		}

		return out, nil
	}

	clusterRouteGet = func(dst net.IP) ([]netlink.Route, error) {
		if idx, ok := topo.routes[dst.String()]; ok {
			return []netlink.Route{{LinkIndex: idx}}, nil
		}

		return nil, errors.New("no route to " + dst.String())
	}

	clusterRouteGetWithOptions = func(dst net.IP, opts *netlink.RouteGetOptions) ([]netlink.Route, error) {
		if opts != nil && opts.VrfName != "" && topo.vrfRoutes[dst.String()] == opts.VrfName {
			return []netlink.Route{{LinkIndex: 0}}, nil
		}

		return nil, errors.New("no route to " + dst.String())
	}

	clusterLinkList = func() ([]netlink.Link, error) {
		return allLinks, nil
	}

	clusterLookupIP = func(ctx context.Context, network, host string) ([]net.IP, error) {
		if ipStrs, ok := topo.hostnames[host]; ok {
			ips := make([]net.IP, 0, len(ipStrs))

			for _, ipStr := range ipStrs {
				ips = append(ips, net.ParseIP(ipStr))
			}

			return ips, nil
		}

		return nil, errors.New("no such host: " + host)
	}
}

func clusterVRFTopo() clusterTopo {
	cpVRF := &netlink.Vrf{
		LinkAttrs: netlink.LinkAttrs{Name: "cp-vrf", Index: 20},
		Table:     1002,
	}
	upVRF := &netlink.Vrf{
		LinkAttrs: netlink.LinkAttrs{Name: "up-vrf", Index: 10},
		Table:     1001,
	}

	return clusterTopo{
		links: map[string]netlink.Link{
			"cp-vrf": cpVRF,
			"up-vrf": upVRF,
			"eth0": &netlink.Device{
				LinkAttrs: netlink.LinkAttrs{Name: "eth0", Index: 4, MasterIndex: 20},
			},
			"n6": &netlink.Device{
				LinkAttrs: netlink.LinkAttrs{Name: "n6", Index: 6, MasterIndex: 10},
			},
			"eth1": &netlink.Device{
				LinkAttrs: netlink.LinkAttrs{Name: "eth1", Index: 5},
			},
		},
		addrs: map[string]int{
			"10.9.0.2": 4,
			"10.9.0.9": 5,
		},
		routes: map[string]int{
			"10.6.0.3": 6,
			"10.9.0.9": 5,
		},
		vrfRoutes: map[string]string{
			"10.100.0.12": "cp-vrf",
		},
		hostnames: map[string][]string{
			"node2": {"10.100.0.12"},
			"node9": {"192.0.2.9"},
			"node3": {"10.9.0.9", "10.100.0.12"},
		},
	}
}

func TestVRFDeviceForBindAddress(t *testing.T) {
	topo := clusterVRFTopo()
	stubClusterNetlink(t, topo)

	for _, tc := range []struct {
		addr string
		want string
	}{
		{"10.9.0.2:5002", "cp-vrf"},
		{"10.9.0.9:5002", ""},
		{":5002", ""},
		{"127.0.0.1:5002", ""},
		{"node1:5002", ""},
		{"bogus", ""},
		{"10.9.9.9:5002", ""},
	} {
		if got := vrfDeviceForBindAddress(tc.addr); got != tc.want {
			t.Errorf("vrfDeviceForBindAddress(%q) = %q, want %q", tc.addr, got, tc.want)
		}
	}
}

func TestVRFDeviceForDestination(t *testing.T) {
	topo := clusterVRFTopo()
	stubClusterNetlink(t, topo)

	for _, tc := range []struct {
		addr string
		want string
	}{
		{"10.6.0.3:179", "up-vrf"},
		{"10.9.0.9:179", ""},
		{"127.0.0.1:179", ""},
		{"192.0.2.1:179", ""},
		{"bogus", ""},
		{"10.100.0.12:7000", "cp-vrf"},
		{"node2:7000", "cp-vrf"},
		{"node9:7000", ""},
		{"node3:7000", "cp-vrf"},
		{"unresolvable:7000", ""},
	} {
		if got := vrfDeviceForDestination(tc.addr); got != tc.want {
			t.Errorf("vrfDeviceForDestination(%q) = %q, want %q", tc.addr, got, tc.want)
		}
	}
}

func TestResolveDestinationIPBoundsLookup(t *testing.T) {
	oldLookupIP := clusterLookupIP

	t.Cleanup(func() { clusterLookupIP = oldLookupIP })

	var gotDeadline time.Time

	clusterLookupIP = func(ctx context.Context, network, host string) ([]net.IP, error) {
		deadline, ok := ctx.Deadline()
		if !ok {
			t.Error("resolver context has no deadline")
			return nil, errors.New("no such host")
		}

		gotDeadline = deadline

		return nil, errors.New("no such host")
	}

	if got := resolveDestinationIPs("node2"); len(got) != 0 {
		t.Errorf("resolveDestinationIPs(unresolvable) = %v, want no addresses", got)
	}

	if until := time.Until(gotDeadline); until <= 0 || until > resolveTimeout {
		t.Errorf("resolver deadline in %v, want within (0, %v]", until, resolveTimeout)
	}
}
