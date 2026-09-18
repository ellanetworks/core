// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package listener

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/netutil"
	"github.com/vishvananda/netlink"
)

type clusterTopo struct {
	links     map[string]netlink.Link
	addrs     map[string]int
	hostnames map[string][]string
}

func stubClusterNetlink(t *testing.T, topo clusterTopo) {
	t.Helper()

	byIndex := map[int]netlink.Link{}

	for _, l := range topo.links {
		byIndex[l.Attrs().Index] = l
	}

	oldByIndex, oldAddrList := netutil.LinkByIndex, netutil.AddrList
	oldLookupIP := clusterLookupIP

	t.Cleanup(func() {
		netutil.LinkByIndex, netutil.AddrList = oldByIndex, oldAddrList
		clusterLookupIP = oldLookupIP
	})

	netutil.LinkByIndex = func(index int) (netlink.Link, error) {
		if l, ok := byIndex[index]; ok {
			return l, nil
		}

		return nil, errors.New("no such index")
	}

	netutil.AddrList = func(link netlink.Link, family int) ([]netlink.Addr, error) {
		var out []netlink.Addr

		for ipStr, idx := range topo.addrs {
			out = append(out, netlink.Addr{
				IPNet:     &net.IPNet{IP: net.ParseIP(ipStr), Mask: net.CIDRMask(24, 32)},
				LinkIndex: idx,
			})
		}

		return out, nil
	}

	clusterLookupIP = func(ctx context.Context, network, host string) ([]net.IP, error) {
		if ipStrs, ok := topo.hostnames[host]; ok {
			ips := make([]net.IP, 0, len(ipStrs))

			for _, ipStr := range ipStrs {
				ips = append(ips, net.ParseIP(ipStr))
			}

			return ips, nil
		}

		return nil, errors.New("no such host: " + host)
	}
}

func clusterVRFTopo() clusterTopo {
	cpVRF := &netlink.Vrf{
		LinkAttrs: netlink.LinkAttrs{Name: "cp-vrf", Index: 20},
		Table:     1002,
	}
	upVRF := &netlink.Vrf{
		LinkAttrs: netlink.LinkAttrs{Name: "up-vrf", Index: 10},
		Table:     1001,
	}

	return clusterTopo{
		links: map[string]netlink.Link{
			"cp-vrf": cpVRF,
			"up-vrf": upVRF,
			"eth0": &netlink.Device{
				LinkAttrs: netlink.LinkAttrs{Name: "eth0", Index: 4, MasterIndex: 20},
			},
			"n6": &netlink.Device{
				LinkAttrs: netlink.LinkAttrs{Name: "n6", Index: 6, MasterIndex: 10},
			},
			"mgmt0": &netlink.Device{
				LinkAttrs: netlink.LinkAttrs{Name: "mgmt0", Index: 5},
			},
		},
		addrs: map[string]int{
			"10.100.0.11": 4,
			"10.6.0.11":   6,
			"10.200.0.1":  5,
		},
		hostnames: map[string][]string{
			"node1":     {"10.100.0.11"},
			"unmanaged": {"192.0.2.9"},
			"multi":     {"192.0.2.9", "10.100.0.11"},
		},
	}
}

func TestVRFDeviceForBindAddress(t *testing.T) {
	stubClusterNetlink(t, clusterVRFTopo())

	for _, tc := range []struct {
		addr string
		want string
	}{
		{"10.100.0.11:7000", "cp-vrf"},
		{"10.6.0.11:7000", "up-vrf"},
		{"10.200.0.1:7000", ""},
		{"0.0.0.0:7000", ""},
		{":7000", ""},
		{"127.0.0.1:7000", ""},
		{"10.9.9.9:7000", ""},
		{"bogus", ""},
		{"node1:7000", "cp-vrf"},
		{"unmanaged:7000", ""},
		{"multi:7000", "cp-vrf"},
		{"unresolvable:7000", ""},
	} {
		if got := vrfDeviceForBindAddress(context.Background(), tc.addr); got != tc.want {
			t.Errorf("vrfDeviceForBindAddress(%q) = %q, want %q", tc.addr, got, tc.want)
		}
	}
}

func TestVRFDeviceForBindAddressIgnoresDestination(t *testing.T) {
	stubClusterNetlink(t, clusterVRFTopo())

	const bindAddr = "10.100.0.11:7000"

	want := vrfDeviceForBindAddress(context.Background(), bindAddr)
	if want != "cp-vrf" {
		t.Fatalf("bind address resolved to %q, want cp-vrf", want)
	}

	ln := &Listener{cfg: Config{BindAddress: bindAddr}}

	for _, peer := range []string{"10.100.0.12:7000", "10.6.0.3:7000", "192.0.2.1:7000"} {
		if got := vrfDeviceForBindAddress(context.Background(), ln.cfg.BindAddress); got != want {
			t.Errorf("dial to %s resolved to %q, want %q", peer, got, want)
		}
	}
}

func TestResolveBindHostBoundsLookup(t *testing.T) {
	oldLookupIP := clusterLookupIP

	t.Cleanup(func() { clusterLookupIP = oldLookupIP })

	var gotDeadline time.Time

	clusterLookupIP = func(ctx context.Context, network, host string) ([]net.IP, error) {
		deadline, ok := ctx.Deadline()
		if !ok {
			t.Error("resolver context has no deadline")
			return nil, errors.New("no such host")
		}

		gotDeadline = deadline

		return nil, errors.New("no such host")
	}

	if got := resolveBindHost(context.Background(), "node1"); len(got) != 0 {
		t.Errorf("resolveBindHost(unresolvable) = %v, want no addresses", got)
	}

	if until := time.Until(gotDeadline); until <= 0 || until > resolveTimeout {
		t.Errorf("resolver deadline in %v, want within (0, %v]", until, resolveTimeout)
	}
}

func TestResolveBindHostHonoursCallerDeadline(t *testing.T) {
	oldLookupIP := clusterLookupIP

	t.Cleanup(func() { clusterLookupIP = oldLookupIP })

	var gotDeadline time.Time

	clusterLookupIP = func(ctx context.Context, network, host string) ([]net.IP, error) {
		gotDeadline, _ = ctx.Deadline()

		return nil, errors.New("no such host")
	}

	const callerTimeout = 50 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), callerTimeout)
	defer cancel()

	resolveBindHost(ctx, "node1")

	if until := time.Until(gotDeadline); until <= 0 || until > callerTimeout {
		t.Errorf("resolver deadline in %v, want within (0, %v]", until, callerTimeout)
	}
}

func TestResolveBindHostSkipsLookupForIPLiteral(t *testing.T) {
	oldLookupIP := clusterLookupIP

	t.Cleanup(func() { clusterLookupIP = oldLookupIP })

	clusterLookupIP = func(ctx context.Context, network, host string) ([]net.IP, error) {
		t.Errorf("resolver called for IP literal %q", host)

		return nil, errors.New("unexpected lookup")
	}

	got := resolveBindHost(context.Background(), "10.100.0.11")
	if len(got) != 1 || !got[0].Equal(net.ParseIP("10.100.0.11")) {
		t.Errorf("resolveBindHost(10.100.0.11) = %v, want [10.100.0.11]", got)
	}
}
