// SPDX-FileCopyrightText: Ella Networks Inc.
//go:build linux

// SPDX-License-Identifier: BUSL-1.1

package bgp

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"testing"

	api "github.com/osrg/gobgp/v4/api"
	"github.com/vishvananda/netlink"
	"go.uber.org/zap"
)

func stubBGPNetlink(t *testing.T, links map[string]netlink.Link, addrs map[string]int) {
	t.Helper()

	byIndex := map[int]netlink.Link{}
	for _, l := range links {
		byIndex[l.Attrs().Index] = l
	}

	oldByName, oldByIndex, oldAddrList := bgpLinkByName, bgpLinkByIndex, bgpAddrList

	t.Cleanup(func() {
		bgpLinkByName, bgpLinkByIndex, bgpAddrList = oldByName, oldByIndex, oldAddrList
	})

	bgpLinkByName = func(name string) (netlink.Link, error) {
		if l, ok := links[name]; ok {
			return l, nil
		}

		return nil, errors.New("not found: " + name)
	}

	bgpLinkByIndex = func(index int) (netlink.Link, error) {
		if l, ok := byIndex[index]; ok {
			return l, nil
		}

		return nil, errors.New("no such index")
	}

	bgpAddrList = func(link netlink.Link, family int) ([]netlink.Addr, error) {
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

func bgpVRFTopo() (map[string]netlink.Link, map[string]int) {
	upVRF := &netlink.Vrf{
		LinkAttrs: netlink.LinkAttrs{Name: "up-vrf", Index: 10},
		Table:     1001,
	}

	links := map[string]netlink.Link{
		"up-vrf": upVRF,
		"n6": &netlink.Device{
			LinkAttrs: netlink.LinkAttrs{Name: "n6", Index: 6, MasterIndex: 10},
		},
		"eth0": &netlink.Device{
			LinkAttrs: netlink.LinkAttrs{Name: "eth0", Index: 4},
		},
	}

	addrs := map[string]int{
		"10.6.0.2": 6,
		"10.9.0.2": 4,
	}

	return links, addrs
}

func TestVRFMasterDevice(t *testing.T) {
	links, _ := bgpVRFTopo()
	stubBGPNetlink(t, links, nil)

	for _, tc := range []struct {
		iface string
		want  string
	}{
		{"n6", "up-vrf"},
		{"eth0", ""},
		{"up-vrf", "up-vrf"},
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
	links, addrs := bgpVRFTopo()
	stubBGPNetlink(t, links, addrs)

	got, err := vrfDeviceForAddress("10.6.0.2")
	if err != nil || got != "up-vrf" {
		t.Errorf("vrfDeviceForAddress(VRF address) = %q, %v; want up-vrf, nil", got, err)
	}

	got, err = vrfDeviceForAddress("10.9.0.2")
	if err != nil || got != "" {
		t.Errorf("vrfDeviceForAddress(plain address) = %q, %v; want \"\", nil", got, err)
	}

	for _, addr := range []string{"0.0.0.0", "::", "", "127.0.0.1"} {
		got, err := vrfDeviceForAddress(addr)
		if err != nil || got != "" {
			t.Errorf("vrfDeviceForAddress(%q) = %q, %v; want \"\", nil", addr, got, err)
		}
	}

	for _, addr := range []string{"not-an-ip", "10.9.9.9"} {
		if _, err := vrfDeviceForAddress(addr); err == nil {
			t.Errorf("vrfDeviceForAddress(%q): expected error, got nil", addr)
		}
	}
}

func startVRFTestSpeaker(t *testing.T, settings BGPSettings, peers []BGPPeer) *BGPService {
	t.Helper()

	links, addrs := bgpVRFTopo()
	stubBGPNetlink(t, links, addrs)

	svc := New(
		netip.MustParseAddr("10.6.0.2"),
		netip.MustParseAddr("fd00::2"),
		zap.NewNop(),
		WithN6Interface("n6"),
	)
	svc.SetListenPort(-1)

	if err := svc.Start(context.Background(), settings, peers, true); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	t.Cleanup(func() { _ = svc.Stop() })

	return svc
}

func TestSpeakerBindsN6VRF(t *testing.T) {
	svc := startVRFTestSpeaker(t,
		BGPSettings{Enabled: true, LocalAS: 65000, RouterID: "10.6.0.2"},
		[]BGPPeer{{Address: "10.6.0.3", RemoteAS: 65001}},
	)

	resp, err := svc.server.GetBgp(context.Background(), &api.GetBgpRequest{})
	if err != nil {
		t.Fatalf("GetBgp: %v", err)
	}

	if resp.Global.BindToDevice != "up-vrf" {
		t.Errorf("Global.BindToDevice = %q, want up-vrf", resp.Global.BindToDevice)
	}

	found := false

	err = svc.server.ListPeer(context.Background(), &api.ListPeerRequest{}, func(p *api.Peer) {
		found = true

		if p.Transport == nil || p.Transport.BindInterface != "up-vrf" {
			t.Errorf("peer Transport.BindInterface = %v, want up-vrf", p.Transport)
		}
	})
	if err != nil {
		t.Fatalf("ListPeer: %v", err)
	}

	if !found {
		t.Error("expected one peer, found none")
	}
}

func TestSpeakerListenerBindFollowsListenAddress(t *testing.T) {
	svc := startVRFTestSpeaker(t,
		BGPSettings{Enabled: true, LocalAS: 65000, RouterID: "10.9.0.2", ListenAddress: "10.9.0.2:179"},
		nil,
	)

	resp, err := svc.server.GetBgp(context.Background(), &api.GetBgpRequest{})
	if err != nil {
		t.Fatalf("GetBgp: %v", err)
	}

	if resp.Global.BindToDevice != "" {
		t.Errorf("Global.BindToDevice = %q, want \"\" for non-VRF listen address", resp.Global.BindToDevice)
	}
}

func TestSpeakerNoBindWithoutVRF(t *testing.T) {
	stubBGPNetlink(t, map[string]netlink.Link{
		"n6": &netlink.Device{
			LinkAttrs: netlink.LinkAttrs{Name: "n6", Index: 6},
		},
	}, nil)

	svc := New(
		netip.MustParseAddr("10.6.0.2"),
		netip.MustParseAddr("fd00::2"),
		zap.NewNop(),
		WithN6Interface("n6"),
	)
	svc.SetListenPort(-1)

	if err := svc.Start(context.Background(), BGPSettings{Enabled: true, LocalAS: 65000, RouterID: "10.6.0.2"}, nil, true); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	t.Cleanup(func() { _ = svc.Stop() })

	resp, err := svc.server.GetBgp(context.Background(), &api.GetBgpRequest{})
	if err != nil {
		t.Fatalf("GetBgp: %v", err)
	}

	if resp.Global.BindToDevice != "" {
		t.Errorf("Global.BindToDevice = %q, want \"\" without VRF", resp.Global.BindToDevice)
	}
}
