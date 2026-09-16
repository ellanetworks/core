// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1
//go:build linux && !386

package sctp

import (
	"errors"
	"net"
	"testing"

	"github.com/vishvananda/netlink"
)

type fakeLink struct {
	name   string
	index  int
	master int
	typ    string
}

func (f *fakeLink) Attrs() *netlink.LinkAttrs {
	return &netlink.LinkAttrs{Name: f.name, Index: f.index, MasterIndex: f.master}
}

func (f *fakeLink) Type() string { return f.typ }

func stubVRFNetlink(t *testing.T, links map[string]*fakeLink, addrs map[string]int) {
	t.Helper()

	byIndex := map[int]*fakeLink{}
	for _, l := range links {
		byIndex[l.index] = l
	}

	oldByName, oldByIndex, oldAddrList := vrfLinkByName, vrfLinkByIndex, vrfAddrList

	t.Cleanup(func() {
		vrfLinkByName, vrfLinkByIndex, vrfAddrList = oldByName, oldByIndex, oldAddrList
	})

	vrfLinkByName = func(name string) (netlink.Link, error) {
		if l, ok := links[name]; ok {
			return l, nil
		}

		return nil, errors.New("not found: " + name)
	}

	vrfLinkByIndex = func(index int) (netlink.Link, error) {
		if l, ok := byIndex[index]; ok {
			return l, nil
		}

		return nil, errors.New("no such index")
	}

	vrfAddrList = func(link netlink.Link, family int) ([]netlink.Addr, error) {
		var out []netlink.Addr

		for ipStr, idx := range addrs {
			out = append(out, netlink.Addr{
				IPNet:     &net.IPNet{IP: net.ParseIP(ipStr), Mask: net.CIDRMask(24, 32)},
				LinkIndex: idx,
			})
		}

		return out, nil
	}
}

func TestVRFMasterDevice(t *testing.T) {
	vrf := &fakeLink{name: "cp-vrf", index: 10, typ: "vrf"}
	slave := &fakeLink{name: "eth0", index: 2, master: 10, typ: "device"}
	plain := &fakeLink{name: "eth1", index: 3, typ: "device"}
	bridge := &fakeLink{name: "br0", index: 11, typ: "bridge"}
	bridged := &fakeLink{name: "eth2", index: 4, master: 11, typ: "device"}

	stubVRFNetlink(t, map[string]*fakeLink{
		"cp-vrf": vrf, "eth0": slave, "eth1": plain, "br0": bridge, "eth2": bridged,
	}, nil)

	for _, tc := range []struct {
		iface string
		want  string
	}{
		{"eth0", "cp-vrf"},
		{"eth1", ""},
		{"eth2", ""},
		{"cp-vrf", "cp-vrf"},
	} {
		got, err := vrfMasterDevice(tc.iface)
		if err != nil {
			t.Errorf("vrfMasterDevice(%q): %v", tc.iface, err)
			continue
		}

		if got != tc.want {
			t.Errorf("vrfMasterDevice(%q) = %q, want %q", tc.iface, got, tc.want)
		}
	}

	if _, err := vrfMasterDevice("nope"); err == nil {
		t.Error("vrfMasterDevice(missing interface): expected error, got nil")
	}
}

func TestVRFDeviceForAddress(t *testing.T) {
	vrf := &fakeLink{name: "cp-vrf", index: 10, typ: "vrf"}
	slave := &fakeLink{name: "eth0", index: 2, master: 10, typ: "device"}
	plain := &fakeLink{name: "eth1", index: 3, typ: "device"}

	stubVRFNetlink(t, map[string]*fakeLink{
		"cp-vrf": vrf, "eth0": slave, "eth1": plain,
	}, map[string]int{
		"10.3.0.2": 2,
		"10.9.0.2": 3,
	})

	got, err := vrfDeviceForAddress("10.3.0.2")
	if err != nil || got != "cp-vrf" {
		t.Errorf("vrfDeviceForAddress(VRF address) = %q, %v; want cp-vrf, nil", got, err)
	}

	got, err = vrfDeviceForAddress("10.9.0.2")
	if err != nil || got != "" {
		t.Errorf("vrfDeviceForAddress(plain address) = %q, %v; want \"\", nil", got, err)
	}

	if _, err := vrfDeviceForAddress("10.9.9.9"); err == nil {
		t.Error("vrfDeviceForAddress(unknown address): expected error, got nil")
	}

	if _, err := vrfDeviceForAddress("not-an-ip"); err == nil {
		t.Error("vrfDeviceForAddress(invalid IP): expected error, got nil")
	}

	for _, addr := range []string{"0.0.0.0", "::", ""} {
		got, err := vrfDeviceForAddress(addr)
		if err != nil || got != "" {
			t.Errorf("vrfDeviceForAddress(%q) = %q, %v; want \"\", nil", addr, got, err)
		}
	}
}

func TestVRFBindDevice(t *testing.T) {
	vrf := &fakeLink{name: "cp-vrf", index: 10, typ: "vrf"}
	slave := &fakeLink{name: "eth0", index: 2, master: 10, typ: "device"}
	plain := &fakeLink{name: "eth1", index: 3, typ: "device"}

	stubVRFNetlink(t, map[string]*fakeLink{
		"cp-vrf": vrf, "eth0": slave, "eth1": plain,
	}, map[string]int{
		"10.3.0.2": 2,
		"10.9.0.2": 3,
	})

	got, err := vrfBindDevice("eth0", "")
	if err != nil || got != "cp-vrf" {
		t.Errorf("vrfBindDevice(name=eth0) = %q, %v; want cp-vrf, nil", got, err)
	}

	got, err = vrfBindDevice("eth1", "")
	if err != nil || got != "" {
		t.Errorf("vrfBindDevice(name=eth1) = %q, %v; want \"\", nil", got, err)
	}

	got, err = vrfBindDevice("", "10.3.0.2")
	if err != nil || got != "cp-vrf" {
		t.Errorf("vrfBindDevice(address=10.3.0.2) = %q, %v; want cp-vrf, nil", got, err)
	}

	got, err = vrfBindDevice("", "10.9.0.2")
	if err != nil || got != "" {
		t.Errorf("vrfBindDevice(address=10.9.0.2) = %q, %v; want \"\", nil", got, err)
	}
}
