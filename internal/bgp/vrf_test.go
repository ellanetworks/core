// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package bgp

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"testing"

	"github.com/ellanetworks/core/internal/netutil"
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

	oldByName, oldByIndex, oldAddrList := netutil.LinkByName, netutil.LinkByIndex, netutil.AddrList

	t.Cleanup(func() {
		netutil.LinkByName, netutil.LinkByIndex, netutil.AddrList = oldByName, oldByIndex, oldAddrList
	})

	netutil.LinkByName = func(name string) (netlink.Link, error) {
		if l, ok := links[name]; ok {
			return l, nil
		}

		return nil, errors.New("not found: " + name)
	}

	netutil.LinkByIndex = func(index int) (netlink.Link, error) {
		if l, ok := byIndex[index]; ok {
			return l, nil
		}

		return nil, errors.New("no such index")
	}

	netutil.AddrList = func(link netlink.Link, family int) ([]netlink.Addr, error) {
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
		"fd00::2":  6,
	}

	return links, addrs
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

func TestSpeakerListenerBindFollowsVRFListenAddress(t *testing.T) {
	svc := startVRFTestSpeaker(t,
		BGPSettings{Enabled: true, LocalAS: 65000, RouterID: "10.6.0.2", ListenAddress: "10.6.0.2:179"},
		nil,
	)

	resp, err := svc.server.GetBgp(context.Background(), &api.GetBgpRequest{})
	if err != nil {
		t.Fatalf("GetBgp: %v", err)
	}

	if resp.Global.BindToDevice != "up-vrf" {
		t.Errorf("Global.BindToDevice = %q, want up-vrf for VRF-resident listen address", resp.Global.BindToDevice)
	}
}

func TestSpeakerListenerBindFollowsVRFListenAddressV6(t *testing.T) {
	svc := startVRFTestSpeaker(t,
		BGPSettings{Enabled: true, LocalAS: 65000, RouterID: "10.6.0.2", ListenAddress: "[fd00::2]:179"},
		nil,
	)

	resp, err := svc.server.GetBgp(context.Background(), &api.GetBgpRequest{})
	if err != nil {
		t.Fatalf("GetBgp: %v", err)
	}

	if resp.Global.BindToDevice != "up-vrf" {
		t.Errorf("Global.BindToDevice = %q, want up-vrf for VRF-resident IPv6 listen address", resp.Global.BindToDevice)
	}
}

func peerBindInterfaces(t *testing.T, svc *BGPService) []string {
	t.Helper()

	var got []string

	err := svc.server.ListPeer(context.Background(), &api.ListPeerRequest{}, func(p *api.Peer) {
		if p.Transport == nil {
			got = append(got, "")
		} else {
			got = append(got, p.Transport.BindInterface)
		}
	})
	if err != nil {
		t.Fatalf("ListPeer: %v", err)
	}

	if len(got) == 0 {
		t.Fatal("expected at least one peer, found none")
	}

	return got
}

func TestSpeakerPeersFollowListenAddressVRF(t *testing.T) {
	svc := startVRFTestSpeaker(t,
		BGPSettings{Enabled: true, LocalAS: 65000, RouterID: "10.6.0.2", ListenAddress: "10.6.0.2:179"},
		[]BGPPeer{{Address: "10.6.0.3", RemoteAS: 65001}},
	)

	for _, got := range peerBindInterfaces(t, svc) {
		if got != "up-vrf" {
			t.Errorf("peer Transport.BindInterface = %q, want up-vrf (the listen address's VRF)", got)
		}
	}
}

func TestSpeakerPeersUnboundForPlainListenAddress(t *testing.T) {
	svc := startVRFTestSpeaker(t,
		BGPSettings{Enabled: true, LocalAS: 65000, RouterID: "10.9.0.2", ListenAddress: "10.9.0.2:179"},
		[]BGPPeer{{Address: "10.9.0.3", RemoteAS: 65001}},
	)

	for _, got := range peerBindInterfaces(t, svc) {
		if got != "" {
			t.Errorf("peer Transport.BindInterface = %q, want \"\" for a non-VRF listen address", got)
		}
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
