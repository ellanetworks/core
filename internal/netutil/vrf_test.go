// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package netutil

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

func stubVRFNetlink(t *testing.T, links map[string]netlink.Link, addrs map[string]int) {
	t.Helper()

	byIndex := map[int]netlink.Link{}
	for _, l := range links {
		byIndex[l.Attrs().Index] = l
	}

	oldByName, oldByIndex, oldAddrList := LinkByName, LinkByIndex, AddrList

	t.Cleanup(func() {
		LinkByName, LinkByIndex, AddrList = oldByName, oldByIndex, oldAddrList
	})

	LinkByName = func(name string) (netlink.Link, error) {
		if l, ok := links[name]; ok {
			return l, nil
		}

		return nil, errors.New("not found: " + name)
	}

	LinkByIndex = func(index int) (netlink.Link, error) {
		if l, ok := byIndex[index]; ok {
			return l, nil
		}

		return nil, errors.New("no such index")
	}

	AddrList = func(link netlink.Link, family int) ([]netlink.Addr, error) {
		var out []netlink.Addr

		for ipStr, idx := range addrs {
			ip := net.ParseIP(ipStr)

			mask := net.CIDRMask(24, 32)
			if ip.To4() == nil {
				mask = net.CIDRMask(64, 128)
			}

			out = append(out, netlink.Addr{
				IPNet:     &net.IPNet{IP: ip, Mask: mask},
				LinkIndex: idx,
			})
		}

		return out, nil
	}
}

func vrfTestTopo() map[string]netlink.Link {
	return map[string]netlink.Link{
		"cp-vrf":   &netlink.Vrf{LinkAttrs: netlink.LinkAttrs{Name: "cp-vrf", Index: 10}, Table: 1002},
		"eth0":     &netlink.Device{LinkAttrs: netlink.LinkAttrs{Name: "eth0", Index: 2, MasterIndex: 10}},
		"eth1":     &netlink.Device{LinkAttrs: netlink.LinkAttrs{Name: "eth1", Index: 3}},
		"br0":      &netlink.Bridge{LinkAttrs: netlink.LinkAttrs{Name: "br0", Index: 11}},
		"eth2":     &netlink.Device{LinkAttrs: netlink.LinkAttrs{Name: "eth2", Index: 4, MasterIndex: 11}},
		"orphan":   &netlink.Device{LinkAttrs: netlink.LinkAttrs{Name: "orphan", Index: 5, MasterIndex: 99}},
		"eth0.100": &netlink.Vlan{LinkAttrs: netlink.LinkAttrs{Name: "eth0.100", Index: 6, MasterIndex: 10, ParentIndex: 2}, VlanId: 100},
		"eth0.200": &netlink.Vlan{LinkAttrs: netlink.LinkAttrs{Name: "eth0.200", Index: 7, ParentIndex: 2}, VlanId: 200},
		"eth1.100": &netlink.Vlan{LinkAttrs: netlink.LinkAttrs{Name: "eth1.100", Index: 8, MasterIndex: 10, ParentIndex: 3}, VlanId: 100},
	}
}

func TestVRFMasterOf(t *testing.T) {
	links := vrfTestTopo()
	stubVRFNetlink(t, links, nil)

	master, err := VRFMasterOf(links["eth0"])
	if err != nil || master == nil || master.Attrs().Name != "cp-vrf" {
		t.Errorf("VRFMasterOf(enslaved) = %v, %v; want cp-vrf, nil", master, err)
	}

	master, err = VRFMasterOf(links["cp-vrf"])
	if err != nil || master == nil || master.Attrs().Name != "cp-vrf" {
		t.Errorf("VRFMasterOf(vrf itself) = %v, %v; want cp-vrf, nil", master, err)
	}

	master, err = VRFMasterOf(links["eth0.100"])
	if err != nil || master == nil || master.Attrs().Name != "cp-vrf" {
		t.Errorf("VRFMasterOf(enslaved vlan) = %v, %v; want cp-vrf, nil", master, err)
	}

	master, err = VRFMasterOf(links["eth1.100"])
	if err != nil || master == nil || master.Attrs().Name != "cp-vrf" {
		t.Errorf("VRFMasterOf(enslaved vlan on plain parent) = %v, %v; want cp-vrf, nil", master, err)
	}

	for _, name := range []string{"eth1", "eth2", "eth0.200"} {
		master, err := VRFMasterOf(links[name])
		if err != nil || master != nil {
			t.Errorf("VRFMasterOf(%s) = %v, %v; want nil, nil", name, master, err)
		}
	}

	master, err = VRFMasterOf(nil)
	if err != nil || master != nil {
		t.Errorf("VRFMasterOf(nil) = %v, %v; want nil, nil", master, err)
	}

	if _, err := VRFMasterOf(links["orphan"]); err == nil {
		t.Error("VRFMasterOf(missing master): expected error, got nil")
	}
}

func TestVRFDeviceForInterface(t *testing.T) {
	stubVRFNetlink(t, vrfTestTopo(), nil)

	for _, tc := range []struct {
		iface string
		want  string
	}{
		{"eth0", "cp-vrf"},
		{"eth1", ""},
		{"eth2", ""},
		{"cp-vrf", "cp-vrf"},
		{"eth0.100", "cp-vrf"},
		{"eth1.100", "cp-vrf"},
		{"eth0.200", ""},
	} {
		got, err := VRFDeviceForInterface(tc.iface)
		if err != nil {
			t.Errorf("VRFDeviceForInterface(%q): %v", tc.iface, err)
			continue
		}

		if got != tc.want {
			t.Errorf("VRFDeviceForInterface(%q) = %q, want %q", tc.iface, got, tc.want)
		}
	}

	if _, err := VRFDeviceForInterface("orphan"); err == nil {
		t.Error("VRFDeviceForInterface(missing master): expected error, got nil")
	}

	if _, err := VRFDeviceForInterface("nope"); err == nil {
		t.Error("VRFDeviceForInterface(missing interface): expected error, got nil")
	}
}

func TestVRFDeviceForAddress(t *testing.T) {
	stubVRFNetlink(t, vrfTestTopo(), map[string]int{
		"10.3.0.2": 2,
		"10.9.0.2": 3,
		"fd00::2":  2,
		"10.3.1.2": 6,
		"10.3.2.2": 7,
	})

	got, err := VRFDeviceForAddress("10.3.0.2")
	if err != nil || got != "cp-vrf" {
		t.Errorf("VRFDeviceForAddress(VRF address) = %q, %v; want cp-vrf, nil", got, err)
	}

	got, err = VRFDeviceForAddress("fd00::2")
	if err != nil || got != "cp-vrf" {
		t.Errorf("VRFDeviceForAddress(IPv6 VRF address) = %q, %v; want cp-vrf, nil", got, err)
	}

	got, err = VRFDeviceForAddress("10.3.1.2")
	if err != nil || got != "cp-vrf" {
		t.Errorf("VRFDeviceForAddress(enslaved VLAN address) = %q, %v; want cp-vrf, nil", got, err)
	}

	got, err = VRFDeviceForAddress("10.9.0.2")
	if err != nil || got != "" {
		t.Errorf("VRFDeviceForAddress(plain address) = %q, %v; want \"\", nil", got, err)
	}

	got, err = VRFDeviceForAddress("10.3.2.2")
	if err != nil || got != "" {
		t.Errorf("VRFDeviceForAddress(unenslaved VLAN over VRF parent) = %q, %v; want \"\", nil", got, err)
	}

	for _, addr := range []string{"", "0.0.0.0", "::", "127.0.0.1", "::1", "10.9.9.9", "fd00::9"} {
		got, err := VRFDeviceForAddress(addr)
		if err != nil || got != "" {
			t.Errorf("VRFDeviceForAddress(%q) = %q, %v; want \"\", nil", addr, got, err)
		}
	}

	if _, err := VRFDeviceForAddress("not-an-ip"); err == nil {
		t.Error("VRFDeviceForAddress(invalid IP): expected error, got nil")
	}
}

func TestVRFDeviceForAddressSkipsFailedLinkLookup(t *testing.T) {
	links := vrfTestTopo()
	stubVRFNetlink(t, links, nil)

	AddrList = func(link netlink.Link, family int) ([]netlink.Addr, error) {
		ip := net.ParseIP("10.3.0.2")

		return []netlink.Addr{
			{IPNet: &net.IPNet{IP: ip, Mask: net.CIDRMask(24, 32)}, LinkIndex: 42},
			{IPNet: &net.IPNet{IP: ip, Mask: net.CIDRMask(24, 32)}, LinkIndex: 2},
		}, nil
	}

	got, err := VRFDeviceForAddress("10.3.0.2")
	if err != nil || got != "cp-vrf" {
		t.Errorf("VRFDeviceForAddress(first link lookup fails) = %q, %v; want cp-vrf, nil", got, err)
	}

	AddrList = func(link netlink.Link, family int) ([]netlink.Addr, error) {
		ip := net.ParseIP("10.3.0.2")

		return []netlink.Addr{
			{IPNet: &net.IPNet{IP: ip, Mask: net.CIDRMask(24, 32)}, LinkIndex: 42},
		}, nil
	}

	got, err = VRFDeviceForAddress("10.3.0.2")
	if err != nil || got != "" {
		t.Errorf("VRFDeviceForAddress(only link lookup fails) = %q, %v; want \"\", nil", got, err)
	}
}

func TestVRFBindDevice(t *testing.T) {
	stubVRFNetlink(t, vrfTestTopo(), map[string]int{
		"10.3.0.2": 2,
		"10.9.0.2": 3,
	})

	got, err := VRFBindDevice("eth0", "")
	if err != nil || got != "cp-vrf" {
		t.Errorf("VRFBindDevice(name=eth0) = %q, %v; want cp-vrf, nil", got, err)
	}

	got, err = VRFBindDevice("eth1", "")
	if err != nil || got != "" {
		t.Errorf("VRFBindDevice(name=eth1) = %q, %v; want \"\", nil", got, err)
	}

	got, err = VRFBindDevice("", "10.3.0.2")
	if err != nil || got != "cp-vrf" {
		t.Errorf("VRFBindDevice(address=10.3.0.2) = %q, %v; want cp-vrf, nil", got, err)
	}

	got, err = VRFBindDevice("", "10.9.0.2")
	if err != nil || got != "" {
		t.Errorf("VRFBindDevice(address=10.9.0.2) = %q, %v; want \"\", nil", got, err)
	}

	if _, err := VRFBindDevice("nope", ""); err == nil {
		t.Error("VRFBindDevice(missing interface): expected error, got nil")
	}
}

func TestBindToDeviceControl(t *testing.T) {
	lc := net.ListenConfig{Control: BindToDeviceControl("lo")}

	conn, err := lc.ListenPacket(context.Background(), "udp", "127.0.0.1:0")
	if err != nil {
		if errors.Is(err, unix.EPERM) {
			t.Skipf("SO_BINDTODEVICE requires CAP_NET_RAW: %v", err)
		}

		t.Fatalf("listen with SO_BINDTODEVICE: %v", err)
	}

	defer func() { _ = conn.Close() }()

	sock, ok := conn.(*net.UDPConn)
	if !ok {
		t.Fatalf("expected a UDP socket, got %T", conn)
	}

	raw, err := sock.SyscallConn()
	if err != nil {
		t.Fatalf("syscall conn: %v", err)
	}

	var (
		device  string
		ctrlErr error
	)

	if err := raw.Control(func(fd uintptr) {
		device, ctrlErr = unix.GetsockoptString(int(fd), unix.SOL_SOCKET, unix.SO_BINDTODEVICE)
	}); err != nil {
		t.Fatalf("control: %v", err)
	}

	if ctrlErr != nil {
		t.Fatalf("getsockopt SO_BINDTODEVICE: %v", ctrlErr)
	}

	if got := strings.TrimRight(device, "\x00"); got != "lo" {
		t.Errorf("socket bound to %q, want lo", got)
	}
}
