// SPDX-FileCopyrightText: Ella Networks Inc.
//go:build linux

// SPDX-License-Identifier: BUSL-1.1

package listener

import (
	"fmt"
	"net"
	"syscall"

	"github.com/vishvananda/netlink"
)

var (
	clusterLinkByName  = netlink.LinkByName
	clusterLinkByIndex = netlink.LinkByIndex
	clusterAddrList    = netlink.AddrList
	clusterRouteGet    = netlink.RouteGet
)

func vrfMasterOf(link netlink.Link) (string, error) {
	if link.Type() == "vrf" {
		return link.Attrs().Name, nil
	}

	masterIndex := link.Attrs().MasterIndex
	if masterIndex == 0 {
		return "", nil
	}

	master, err := clusterLinkByIndex(masterIndex)
	if err != nil {
		return "", fmt.Errorf("lookup master of interface %s: %w", link.Attrs().Name, err)
	}

	if master.Type() != "vrf" {
		return "", nil
	}

	return master.Attrs().Name, nil
}

func linkOwningAddress(ip net.IP) (netlink.Link, error) {
	addrs, err := clusterAddrList(nil, netlink.FAMILY_ALL)
	if err != nil {
		return nil, fmt.Errorf("list interface addresses: %w", err)
	}

	for _, addr := range addrs {
		if addr.IP.Equal(ip) {
			link, err := clusterLinkByIndex(addr.LinkIndex)
			if err != nil {
				return nil, fmt.Errorf("lookup interface for address %s: %w", ip, err)
			}

			return link, nil
		}
	}

	return nil, fmt.Errorf("no interface holds address %s", ip)
}

func vrfDeviceForBindAddress(bindAddr string) string {
	host, _, err := net.SplitHostPort(bindAddr)
	if err != nil || host == "" {
		return ""
	}

	ip := net.ParseIP(host)
	if ip == nil || ip.IsUnspecified() || ip.IsLoopback() {
		return ""
	}

	link, err := linkOwningAddress(ip)
	if err != nil {
		return ""
	}

	device, err := vrfMasterOf(link)
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

	ip := net.ParseIP(host)
	if ip == nil || ip.IsUnspecified() || ip.IsLoopback() {
		return ""
	}

	routes, err := clusterRouteGet(ip)
	if err != nil || len(routes) == 0 {
		return ""
	}

	link, err := clusterLinkByIndex(routes[0].LinkIndex)
	if err != nil {
		return ""
	}

	device, err := vrfMasterOf(link)
	if err != nil {
		return ""
	}

	return device
}

func bindToDeviceControl(device string) func(network, address string, c syscall.RawConn) error {
	return func(network, address string, c syscall.RawConn) error {
		var setErr error

		if err := c.Control(func(fd uintptr) {
			setErr = syscall.SetsockoptString(int(fd), syscall.SOL_SOCKET, syscall.SO_BINDTODEVICE, device)
		}); err != nil {
			return err
		}

		return setErr
	}
}
