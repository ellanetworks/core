// SPDX-FileCopyrightText: Ella Networks Inc.
//go:build linux

// SPDX-License-Identifier: BUSL-1.1

package api

import (
	"net"

	"github.com/vishvananda/netlink"
)

var (
	apiLinkByIndex = netlink.LinkByIndex
	apiAddrList    = netlink.AddrList
)

func vrfDeviceForAPIAddress(address string) string {
	if address == "" {
		return ""
	}

	ip := net.ParseIP(address)
	if ip == nil || ip.IsUnspecified() || ip.IsLoopback() {
		return ""
	}

	addrs, err := apiAddrList(nil, netlink.FAMILY_ALL)
	if err != nil {
		return ""
	}

	for _, addr := range addrs {
		if !addr.IP.Equal(ip) {
			continue
		}

		link, err := apiLinkByIndex(addr.LinkIndex)
		if err != nil {
			return ""
		}

		return vrfMasterOf(link)
	}

	return ""
}

func vrfMasterOf(link netlink.Link) string {
	if link == nil {
		return ""
	}

	if link.Type() == "vrf" {
		return link.Attrs().Name
	}

	masterIndex := link.Attrs().MasterIndex
	if masterIndex == 0 {
		return ""
	}

	master, err := apiLinkByIndex(masterIndex)
	if err != nil {
		return ""
	}

	if master.Type() != "vrf" {
		return ""
	}

	return master.Attrs().Name
}
