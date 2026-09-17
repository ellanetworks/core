// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package netutil

import (
	"fmt"
	"net"
	"syscall"

	"github.com/vishvananda/netlink"
)

var (
	LinkByName  = netlink.LinkByName
	LinkByIndex = netlink.LinkByIndex
	AddrList    = netlink.AddrList
)

// VRFMasterOf returns the VRF device link is enslaved to, or nil.
func VRFMasterOf(link netlink.Link) (netlink.Link, error) {
	if link == nil {
		return nil, nil
	}

	if link.Type() == "vrf" {
		return link, nil
	}

	masterIndex := link.Attrs().MasterIndex
	if masterIndex == 0 {
		return nil, nil
	}

	master, err := LinkByIndex(masterIndex)
	if err != nil {
		return nil, fmt.Errorf("lookup master of interface %s: %w", link.Attrs().Name, err)
	}

	if master.Type() != "vrf" {
		return nil, nil
	}

	return master, nil
}

// VRFDeviceForInterface returns the name of the VRF device ifaceName is enslaved to.
func VRFDeviceForInterface(ifaceName string) (string, error) {
	link, err := LinkByName(ifaceName)
	if err != nil {
		return "", fmt.Errorf("lookup interface %s: %w", ifaceName, err)
	}

	master, err := VRFMasterOf(link)
	if err != nil {
		return "", err
	}

	if master == nil {
		return "", nil
	}

	return master.Attrs().Name, nil
}

// VRFDeviceForAddress returns the name of the VRF device whose slave interface
// holds ipStr, or "" when no VRF binding applies.
func VRFDeviceForAddress(ipStr string) (string, error) {
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

	addrs, err := AddrList(nil, netlink.FAMILY_ALL)
	if err != nil {
		return "", fmt.Errorf("list interface addresses: %w", err)
	}

	for _, addr := range addrs {
		if !addr.IP.Equal(ip) {
			continue
		}

		link, err := LinkByIndex(addr.LinkIndex)
		if err != nil {
			return "", fmt.Errorf("lookup interface for address %s: %w", ip, err)
		}

		master, err := VRFMasterOf(link)
		if err != nil {
			return "", err
		}

		if master == nil {
			return "", nil
		}

		return master.Attrs().Name, nil
	}

	return "", nil
}

// VRFBindDevice resolves the SO_BINDTODEVICE target for a listener configured
// with an interface name or a bind address.
func VRFBindDevice(interfaceName, address string) (string, error) {
	if interfaceName != "" {
		return VRFDeviceForInterface(interfaceName)
	}

	return VRFDeviceForAddress(address)
}

func BindToDeviceControl(device string) func(network, address string, c syscall.RawConn) error {
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
