// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package interworking

import (
	"errors"
	"fmt"
	"net"

	"github.com/ellanetworks/core/internal/tester/scenarios"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

func assignedSource(env scenarios.Env, iface string, addrs ueAddresses) (string, error) {
	if !wantsIPv6Probe(env) {
		return addrs.v4, nil
	}

	return globalFromAssignedIID(iface, addrs.v6)
}

func globalFromAssignedIID(iface, linkLocal string) (string, error) {
	assigned := net.ParseIP(linkLocal).To16()
	if assigned == nil {
		return "", fmt.Errorf("the assigned interface identifier %q is not an IPv6 address", linkLocal)
	}

	link, err := netlink.LinkByName(iface)
	if err != nil {
		return "", fmt.Errorf("find %s: %w", iface, err)
	}

	present, err := netlink.AddrList(link, netlink.FAMILY_V6)
	if err != nil {
		return "", fmt.Errorf("list the addresses of %s: %w", iface, err)
	}

	for _, addr := range present {
		if !addr.IP.IsGlobalUnicast() {
			continue
		}

		global := addr.IP.Mask(net.CIDRMask(64, 128))
		copy(global[8:], assigned[8:])

		if global.Equal(addr.IP) {
			return global.String(), nil
		}

		add := &netlink.Addr{
			IPNet: &net.IPNet{IP: global, Mask: net.CIDRMask(64, 128)},
			Flags: unix.IFA_F_NODAD,
		}

		if err := netlink.AddrAdd(link, add); err != nil && !errors.Is(err, unix.EEXIST) {
			return "", fmt.Errorf("give %s the address %s: %w", iface, global, err)
		}

		return global.String(), nil
	}

	return "", fmt.Errorf("%s autoconfigured no global address to take a prefix from", iface)
}
