// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package config

import (
	"net"
	"net/netip"

	"github.com/vishvananda/netlink"
)

type AddressSource interface {
	IPs(name string) ([]string, error)
	IP(name string, family AddressFamily) (string, error)
	Subnets(name string) []netip.Prefix
	LinkUp(name string) bool
}

func (c Config) Addresses() AddressSource {
	if c.AddressSource != nil {
		return c.AddressSource
	}

	return hostAddresses{}
}

type hostAddresses struct{}

func (hostAddresses) IPs(name string) ([]string, error) {
	return GetInterfaceIPs(name)
}

func (hostAddresses) IP(name string, family AddressFamily) (string, error) {
	return GetInterfaceIP(name, family)
}

func (hostAddresses) Subnets(name string) []netip.Prefix {
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return nil
	}

	addrs, err := iface.Addrs()
	if err != nil {
		return nil
	}

	var subnets []netip.Prefix

	for _, addr := range addrs {
		p, err := netip.ParsePrefix(addr.String())
		if err != nil {
			continue
		}

		subnets = append(subnets, p.Masked())
	}

	return subnets
}

func (hostAddresses) LinkUp(name string) bool {
	link, err := netlink.LinkByName(name)
	if err != nil {
		return false
	}

	return link.Attrs().Flags&net.FlagUp != 0
}
