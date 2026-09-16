// SPDX-FileCopyrightText: Ella Networks Inc.
//go:build linux

// SPDX-License-Identifier: BUSL-1.1

package bgp

import (
	"fmt"
	"net"

	"github.com/vishvananda/netlink"
)

var (
	bgpLinkByName  = netlink.LinkByName
	bgpLinkByIndex = netlink.LinkByIndex
	bgpAddrList    = netlink.AddrList
)

func vrfMasterDevice(ifaceName string) (string, error) {
	link, err := bgpLinkByName(ifaceName)
	if err != nil {
		return "", fmt.Errorf("lookup interface %s: %w", ifaceName, err)
	}

	if link.Type() == "vrf" {
		return ifaceName, nil
	}

	masterIndex := link.Attrs().MasterIndex
	if masterIndex == 0 {
		return "", nil
	}

	master, err := bgpLinkByIndex(masterIndex)
	if err != nil {
		return "", fmt.Errorf("lookup master of interface %s: %w", ifaceName, err)
	}

	if master.Type() != "vrf" {
		return "", nil
	}

	return master.Attrs().Name, nil
}

func vrfDeviceForAddress(ipStr string) (string, error) {
	if ipStr == "" {
		return "", nil
	}

	ip := net.ParseIP(ipStr)
	if ip == nil {
		return "", fmt.Errorf("invalid IP address %q", ipStr)
	}

	if ip.IsUnspecified() || ip.IsLoopback() {
		return "", nil
	}

	addrs, err := bgpAddrList(nil, netlink.FAMILY_ALL)
	if err != nil {
		return "", fmt.Errorf("list interface addresses: %w", err)
	}

	for _, addr := range addrs {
		if addr.IP.Equal(ip) {
			link, err := bgpLinkByIndex(addr.LinkIndex)
			if err != nil {
				return "", fmt.Errorf("lookup interface for address %s: %w", ip, err)
			}

			return vrfMasterDevice(link.Attrs().Name)
		}
	}

	return "", fmt.Errorf("no interface holds address %s", ip)
}
