// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package listener

import (
	"context"
	"net"
	"time"

	"github.com/ellanetworks/core/internal/netutil"
	"github.com/vishvananda/netlink"
)

const resolveTimeout = 5 * time.Second

var (
	clusterRouteGet            = netlink.RouteGet
	clusterRouteGetWithOptions = netlink.RouteGetWithOptions
	clusterLinkList            = netlink.LinkList
	clusterLookupIP            = net.DefaultResolver.LookupIP
)

func vrfDeviceForBindAddress(bindAddr string) string {
	host, _, err := net.SplitHostPort(bindAddr)
	if err != nil || host == "" {
		return ""
	}

	device, err := netutil.VRFDeviceForAddress(host)
	if err != nil {
		return ""
	}

	return device
}

func vrfDeviceForDestination(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}

	for _, ip := range resolveDestinationIPs(host) {
		if ip.IsUnspecified() || ip.IsLoopback() {
			continue
		}

		if device := vrfDeviceForIP(ip); device != "" {
			return device
		}
	}

	return ""
}

func resolveDestinationIPs(host string) []net.IP {
	if ip := net.ParseIP(host); ip != nil {
		return []net.IP{ip}
	}

	ctx, cancel := context.WithTimeout(context.Background(), resolveTimeout)
	defer cancel()

	ips, _ := clusterLookupIP(ctx, "ip", host)

	return ips
}

func vrfDeviceForIP(ip net.IP) string {
	routes, err := clusterRouteGet(ip)
	if err == nil && len(routes) > 0 {
		link, err := netutil.LinkByIndex(routes[0].LinkIndex)
		if err != nil {
			return ""
		}

		master, err := netutil.VRFMasterOf(link)
		if err != nil || master == nil {
			return ""
		}

		return master.Attrs().Name
	}

	return vrfDeviceFromVRFTables(ip)
}

func vrfDeviceFromVRFTables(ip net.IP) string {
	links, err := clusterLinkList()
	if err != nil {
		return ""
	}

	for _, link := range links {
		if link.Type() != "vrf" {
			continue
		}

		name := link.Attrs().Name

		routes, err := clusterRouteGetWithOptions(ip, &netlink.RouteGetOptions{VrfName: name, FIBMatch: true})
		if err == nil && len(routes) > 0 {
			return name
		}
	}

	return ""
}
