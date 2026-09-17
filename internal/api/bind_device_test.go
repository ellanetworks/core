// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package api

import (
	"errors"
	"net"
	"testing"

	"github.com/ellanetworks/core/internal/config"
	"github.com/ellanetworks/core/internal/netutil"
	"github.com/vishvananda/netlink"
)

func stubBindDeviceNetlink(t *testing.T, addrs map[string]netlink.Link) {
	t.Helper()

	oldAddrList, oldLinkByIndex := netutil.AddrList, netutil.LinkByIndex

	t.Cleanup(func() {
		netutil.AddrList, netutil.LinkByIndex = oldAddrList, oldLinkByIndex
	})

	netutil.AddrList = func(link netlink.Link, family int) ([]netlink.Addr, error) {
		var out []netlink.Addr

		for ipStr, link := range addrs {
			out = append(out, netlink.Addr{
				IPNet:     &net.IPNet{IP: net.ParseIP(ipStr), Mask: net.CIDRMask(24, 32)},
				LinkIndex: link.Attrs().Index,
			})
		}

		return out, nil
	}

	netutil.LinkByIndex = func(index int) (netlink.Link, error) {
		for _, link := range addrs {
			if link.Attrs().Index == index {
				return link, nil
			}
		}

		return nil, errors.New("no such index")
	}
}

func TestAPIBindDevicePrefersConfiguredInterface(t *testing.T) {
	stubBindDeviceNetlink(t, nil)

	got := apiBindDevice(config.APIInterface{Name: "eth0", Address: "10.0.0.2"})
	if got != "eth0" {
		t.Errorf("apiBindDevice = %q, want the configured interface name", got)
	}
}

func TestAPIBindDeviceResolvesVRF(t *testing.T) {
	vrf := &netlink.Vrf{LinkAttrs: netlink.LinkAttrs{Name: "cp-vrf", Index: 10}, Table: 1002}

	stubBindDeviceNetlink(t, map[string]netlink.Link{"10.0.0.2": vrf})

	got := apiBindDevice(config.APIInterface{Address: "10.0.0.2"})
	if got != "cp-vrf" {
		t.Errorf("apiBindDevice = %q, want the resolved VRF device cp-vrf", got)
	}
}

func TestAPIBindDeviceResolvesVRFThroughVLAN(t *testing.T) {
	vrf := &netlink.Vrf{LinkAttrs: netlink.LinkAttrs{Name: "cp-vrf", Index: 10}, Table: 1002}
	vlan := &netlink.Vlan{
		LinkAttrs: netlink.LinkAttrs{Name: "eth0.100", Index: 6, MasterIndex: 10, ParentIndex: 2},
		VlanId:    100,
	}

	stubBindDeviceNetlink(t, map[string]netlink.Link{"10.0.0.2": vlan, "10.0.0.99": vrf})

	got := apiBindDevice(config.APIInterface{Address: "10.0.0.2"})
	if got != "cp-vrf" {
		t.Errorf("apiBindDevice = %q, want the VRF device of the enslaved VLAN link", got)
	}
}

func TestAPIBindDeviceFallsBackOnResolutionError(t *testing.T) {
	stubBindDeviceNetlink(t, nil)

	oldAddrList := netutil.AddrList

	t.Cleanup(func() { netutil.AddrList = oldAddrList })

	netutil.AddrList = func(link netlink.Link, family int) ([]netlink.Addr, error) {
		return nil, errors.New("netlink: dump failed")
	}

	got := apiBindDevice(config.APIInterface{Address: "10.0.0.2"})
	if got != "" {
		t.Errorf("apiBindDevice = %q, want unbound fallback on resolution error", got)
	}
}
