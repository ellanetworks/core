// SPDX-FileCopyrightText: Ella Networks Inc.
//go:build linux

// SPDX-License-Identifier: BUSL-1.1

package sctp

import (
	"fmt"
	"net"
	"syscall"

	"github.com/vishvananda/netlink"
)

var (
	vrfLinkByName  = netlink.LinkByName
	vrfLinkByIndex = netlink.LinkByIndex
	vrfAddrList    = netlink.AddrList
)

func vrfMasterDevice(ifaceName string) (string, error) {
	link, err := vrfLinkByName(ifaceName)
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

	master, err := vrfLinkByIndex(masterIndex)
	if err != nil {
		return "", fmt.Errorf("lookup master of interface %s: %w", ifaceName, err)
	}

	if master.Type() != "vrf" {
		return "", nil
	}

	return master.Attrs().Name, nil
}

func vrfBindDevice(interfaceName, address string) (string, error) {
	if interfaceName != "" {
		return vrfMasterDevice(interfaceName)
	}

	return vrfDeviceForAddress(address)
}

func vrfDeviceForAddress(ipStr string) (string, error) {
	if ipStr == "" {
		return "", nil
	}

	ip := net.ParseIP(ipStr)
	if ip == nil {
		return "", fmt.Errorf("invalid IP address %q", ipStr)
	}

	if ip.IsUnspecified() {
		return "", nil
	}

	link, err := linkOwningAddress(ip)
	if err != nil {
		return "", err
	}

	return vrfMasterDevice(link.Attrs().Name)
}

func linkOwningAddress(ip net.IP) (netlink.Link, error) {
	addrs, err := vrfAddrList(nil, netlink.FAMILY_ALL)
	if err != nil {
		return nil, fmt.Errorf("list interface addresses: %w", err)
	}

	for _, addr := range addrs {
		if addr.IP.Equal(ip) {
			link, err := vrfLinkByIndex(addr.LinkIndex)
			if err != nil {
				return nil, fmt.Errorf("lookup interface for address %s: %w", ip, err)
			}

			return link, nil
		}
	}

	return nil, fmt.Errorf("no interface holds address %s", ip)
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
