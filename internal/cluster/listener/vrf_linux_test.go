// SPDX-FileCopyrightText: Ella Networks Inc.
//go:build linux

// SPDX-License-Identifier: BUSL-1.1

package listener

import (
	"errors"
	"net"
	"testing"

	"github.com/vishvananda/netlink"
)

func stubClusterNetlink(t *testing.T, links map[string]netlink.Link, addrs map[string]int, routes map[string]int) {
	t.Helper()

	byIndex := map[int]netlink.Link{}
	for _, l := range links {
		byIndex[l.Attrs().Index] = l
	}

	oldByName, oldByIndex, oldAddrList, oldRouteGet := clusterLinkByName, clusterLinkByIndex, clusterAddrList, clusterRouteGet

	t.Cleanup(func() {
		clusterLinkByName, clusterLinkByIndex, clusterAddrList, clusterRouteGet = oldByName, oldByIndex, oldAddrList, oldRouteGet
	})

	clusterLinkByName = func(name string) (netlink.Link, error) {
		if l, ok := links[name]; ok {
			return l, nil
		}

		return nil, errors.New("not found: " + name)
	}

	clusterLinkByIndex = func(index int) (netlink.Link, error) {
		if l, ok := byIndex[index]; ok {
			return l, nil
		}

		return nil, errors.New("no such index")
	}

	clusterAddrList = func(link netlink.Link, family int) ([]netlink.Addr, error) {
		var out []netlink.Addr

		for ipStr, idx := range addrs {
			out = append(out, netlink.Addr{
				IPNet:     &net.IPNet{IP: net.ParseIP(ipStr), Mask: net.CIDRMask(24, 32)},
				LinkIndex: idx,
			})
		}

		return out, nil
	}

	clusterRouteGet = func(dst net.IP) ([]netlink.Route, error) {
		if idx, ok := routes[dst.String()]; ok {
			return []netlink.Route{{LinkIndex: idx}}, nil
		}

		return nil, errors.New("no route to " + dst.String())
	}
}

func clusterVRFTopo() (map[string]netlink.Link, map[string]int, map[string]int) {
	cpVRF := &netlink.Vrf{
		LinkAttrs: netlink.LinkAttrs{Name: "cp-vrf", Index: 20},
		Table:     1002,
	}
	upVRF := &netlink.Vrf{
		LinkAttrs: netlink.LinkAttrs{Name: "up-vrf", Index: 10},
		Table:     1001,
	}

	links := map[string]netlink.Link{
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
	}

	addrs := map[string]int{
		"10.9.0.2": 4,
		"10.9.0.9": 5,
	}

	routes := map[string]int{
		"10.6.0.3": 6,
		"10.9.0.9": 5,
	}

	return links, addrs, routes
}

func TestVRFDeviceForBindAddress(t *testing.T) {
	links, addrs, routes := clusterVRFTopo()
	stubClusterNetlink(t, links, addrs, routes)

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
	links, addrs, routes := clusterVRFTopo()
	stubClusterNetlink(t, links, addrs, routes)

	for _, tc := range []struct {
		addr string
		want string
	}{
		{"10.6.0.3:179", "up-vrf"},
		{"10.9.0.9:179", ""},
		{"127.0.0.1:179", ""},
		{"192.0.2.1:179", ""},
		{"bogus", ""},
	} {
		if got := vrfDeviceForDestination(tc.addr); got != tc.want {
			t.Errorf("vrfDeviceForDestination(%q) = %q, want %q", tc.addr, got, tc.want)
		}
	}
}
