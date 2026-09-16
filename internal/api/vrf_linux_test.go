// SPDX-FileCopyrightText: Ella Networks Inc.
//go:build linux

// SPDX-License-Identifier: BUSL-1.1

package api

import (
	"errors"
	"net"
	"testing"

	"github.com/vishvananda/netlink"
)

func TestVRFDeviceForAPIAddress(t *testing.T) {
	upVRF := &netlink.Vrf{
		LinkAttrs: netlink.LinkAttrs{Name: "up-vrf", Index: 10},
		Table:     1001,
	}
	n3 := &netlink.Device{
		LinkAttrs: netlink.LinkAttrs{Name: "n3", Index: 2, MasterIndex: 10},
	}
	plain := &netlink.Device{
		LinkAttrs: netlink.LinkAttrs{Name: "eth1", Index: 3},
	}

	byIndex := map[int]netlink.Link{10: upVRF, 2: n3, 3: plain}
	addrs := map[string]int{"10.3.0.2": 2, "10.9.0.2": 3}

	oldByIndex, oldAddrList := apiLinkByIndex, apiAddrList

	t.Cleanup(func() { apiLinkByIndex, apiAddrList = oldByIndex, oldAddrList })

	apiLinkByIndex = func(index int) (netlink.Link, error) {
		if l, ok := byIndex[index]; ok {
			return l, nil
		}

		return nil, errors.New("no such index")
	}

	apiAddrList = func(link netlink.Link, family int) ([]netlink.Addr, error) {
		var out []netlink.Addr

		for ipStr, idx := range addrs {
			out = append(out, netlink.Addr{
				IPNet:     &net.IPNet{IP: net.ParseIP(ipStr), Mask: net.CIDRMask(24, 32)},
				LinkIndex: idx,
			})
		}

		return out, nil
	}

	for _, tc := range []struct {
		address string
		want    string
	}{
		{"10.3.0.2", "up-vrf"},
		{"10.9.0.2", ""},
		{"", ""},
		{"0.0.0.0", ""},
		{"::", ""},
		{"127.0.0.1", ""},
		{"not-an-ip", ""},
		{"10.9.9.9", ""},
	} {
		if got := vrfDeviceForAPIAddress(tc.address); got != tc.want {
			t.Errorf("vrfDeviceForAPIAddress(%q) = %q, want %q", tc.address, got, tc.want)
		}
	}
}
