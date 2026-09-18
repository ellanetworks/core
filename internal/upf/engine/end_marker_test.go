// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package engine

import (
	"encoding/hex"
	"errors"
	"net"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/netutil"
	"github.com/ellanetworks/core/internal/upf/ebpf"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

func senderOrSkip(t *testing.T, local netip.Addr) *endMarkerSockets {
	t.Helper()

	socks := &endMarkerSockets{}

	t.Cleanup(func() { _ = socks.Close() })

	if _, err := socks.get(local); err != nil {
		t.Skipf("cannot bind %s:%d to send End Markers: %v", local, gtpuPort, err)
	}

	return socks
}

func TestEndMarkerPDUMatchesTS29281(t *testing.T) {
	got := hex.EncodeToString(endMarkerPDU(0x1000))

	if want := "30fe000000001000"; got != want {
		t.Errorf("End Marker PDU = %s, want %s", got, want)
	}
}

func gtpFar(teid uint32, peer, local netip.Addr) ebpf.FarInfo {
	return ebpf.FarInfo{
		Action:              farForward,
		OuterHeaderCreation: 1,
		TeID:                teid,
		RemoteIP:            ebpf.IPToIn6Addr(peer),
		LocalIP:             ebpf.IPToIn6Addr(local),
	}
}

func TestEndMarkerTargetFor(t *testing.T) {
	var (
		upf    = netip.MustParseAddr("10.0.0.1")
		source = netip.MustParseAddr("10.0.0.2")
		target = netip.MustParseAddr("10.0.0.3")
	)

	tests := []struct {
		name     string
		old, new ebpf.FarInfo
		want     bool
	}{
		{"peer changed", gtpFar(0x11, source, upf), gtpFar(0x22, target, upf), true},
		{"teid changed on the same peer", gtpFar(0x11, source, upf), gtpFar(0x22, source, upf), true},
		{"unchanged", gtpFar(0x11, source, upf), gtpFar(0x11, source, upf), false},
		{"no previous tunnel", ebpf.FarInfo{}, gtpFar(0x22, target, upf), false},
		{"no new tunnel", gtpFar(0x11, source, upf), ebpf.FarInfo{}, false},
		{"previous tunnel had no peer", gtpFar(0x11, netip.Addr{}, upf), gtpFar(0x22, target, upf), false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := endMarkerTargetFor(tc.old, tc.new)
			if ok != tc.want {
				t.Fatalf("endMarkerTargetFor ok = %v, want %v", ok, tc.want)
			}

			if !ok {
				return
			}

			if got.teid != tc.old.TeID {
				t.Errorf("target TEID = %#x, want the old tunnel's %#x", got.teid, tc.old.TeID)
			}

			if got.peer != ebpf.In6AddrToIP(tc.old.RemoteIP).Unmap() {
				t.Errorf("target peer = %s, want the old tunnel's peer", got.peer)
			}
		})
	}
}

func TestEndMarkerSocketsSendToMultiplePeers(t *testing.T) {
	local := netip.MustParseAddr("127.0.0.1")

	peers := []netip.Addr{netip.MustParseAddr("127.0.0.2"), netip.MustParseAddr("127.0.0.3")}
	got := make(chan string, len(peers))

	for _, peer := range peers {
		ln, err := net.ListenUDP("udp", net.UDPAddrFromAddrPort(netip.AddrPortFrom(peer, gtpuPort)))
		if err != nil {
			t.Skipf("cannot bind %s:%d: %v", peer, gtpuPort, err)
		}

		t.Cleanup(func() { _ = ln.Close() })

		go func() {
			buf := make([]byte, 64)

			n, from, err := ln.ReadFromUDP(buf)
			if err != nil {
				return
			}

			if from.Port != gtpuPort {
				t.Errorf("End Marker source port = %d, want %d", from.Port, gtpuPort)
			}

			got <- hex.EncodeToString(buf[:n])
		}()
	}

	socks := senderOrSkip(t, local)

	for i, peer := range peers {
		target := endMarkerTarget{teid: uint32(0x1000 + i), local: local, peer: peer}
		if err := socks.send(target, 1); err != nil {
			t.Fatalf("send to %s: %v", peer, err)
		}
	}

	want := map[string]bool{"30fe000000001000": true, "30fe000000001001": true}

	for range peers {
		select {
		case pdu := <-got:
			if !want[pdu] {
				t.Errorf("unexpected End Marker %s", pdu)
			}

			delete(want, pdu)
		case <-time.After(3 * time.Second):
			t.Fatal("timed out waiting for an End Marker")
		}
	}
}

func TestEndMarkerSocketsReuseOneBinding(t *testing.T) {
	local := netip.MustParseAddr("127.0.0.1")
	peer := netip.MustParseAddr("127.0.0.4")

	socks := senderOrSkip(t, local)

	first, err := socks.get(local)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	second, err := socks.get(local)
	if err != nil {
		t.Fatalf("second get: %v", err)
	}

	if first != second {
		t.Error("a second End Marker send rebound the port instead of reusing the socket")
	}

	if err := socks.send(endMarkerTarget{teid: 1, local: local, peer: peer}, 2); err != nil {
		t.Fatalf("send: %v", err)
	}
}

func TestEndMarkerSendRejectsMismatchedFamily(t *testing.T) {
	target := endMarkerTarget{
		teid:  1,
		local: netip.MustParseAddr("10.0.0.1"),
		peer:  netip.MustParseAddr("2001:db8::1"),
	}

	var socks endMarkerSockets

	if err := socks.send(target, 1); err == nil {
		t.Error("sending an End Marker across address families should fail")
	}
}

func TestEndMarkerSocketsRefuseSendAfterClose(t *testing.T) {
	local := netip.MustParseAddr("127.0.0.1")

	socks := senderOrSkip(t, local)

	if err := socks.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	target := endMarkerTarget{teid: 1, local: local, peer: netip.MustParseAddr("127.0.0.5")}
	if err := socks.send(target, 1); err == nil {
		t.Error("a send after shutdown reopened a socket nothing will close")
	}
}

func nonLoopbackIPv4(t *testing.T) netip.Addr {
	t.Helper()

	addrs, err := net.InterfaceAddrs()
	if err != nil {
		t.Skipf("cannot list host addresses: %v", err)
	}

	for _, a := range addrs {
		ipNet, ok := a.(*net.IPNet)
		if !ok {
			continue
		}

		ip, ok := netip.AddrFromSlice(ipNet.IP)
		if !ok {
			continue
		}

		if ip = ip.Unmap(); ip.Is4() && !ip.IsLoopback() {
			return ip
		}
	}

	t.Skip("no non-loopback IPv4 address on this host")

	return netip.Addr{}
}

func boundDevice(t *testing.T, sock *net.UDPConn) string {
	t.Helper()

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

	return strings.TrimRight(device, "\x00")
}

func stubEndMarkerVRF(t *testing.T, local netip.Addr, addrListErr error) {
	t.Helper()

	oldAddrList, oldLinkByIndex := netutil.AddrList, netutil.LinkByIndex

	t.Cleanup(func() {
		netutil.AddrList, netutil.LinkByIndex = oldAddrList, oldLinkByIndex
	})

	netutil.AddrList = func(link netlink.Link, family int) ([]netlink.Addr, error) {
		if addrListErr != nil {
			return nil, addrListErr
		}

		return []netlink.Addr{{
			IPNet:     &net.IPNet{IP: local.AsSlice(), Mask: net.CIDRMask(24, 32)},
			LinkIndex: 2,
		}}, nil
	}

	netutil.LinkByIndex = func(index int) (netlink.Link, error) {
		if index == 2 {
			return &netlink.Vrf{LinkAttrs: netlink.LinkAttrs{Name: "lo", Index: 2}, Table: 1001}, nil
		}

		return nil, errors.New("no such index")
	}
}

func TestEndMarkerSocketBindsToResolvedVRFDevice(t *testing.T) {
	local := nonLoopbackIPv4(t)

	stubEndMarkerVRF(t, local, nil)

	socks := senderOrSkip(t, local)

	sock, err := socks.get(local)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if dev := boundDevice(t, sock); dev != "lo" {
		t.Errorf("End Marker socket bound to %q, want the resolved VRF device lo", dev)
	}
}

func TestEndMarkerSocketFallsBackWhenVRFResolutionFails(t *testing.T) {
	local := nonLoopbackIPv4(t)

	stubEndMarkerVRF(t, local, errors.New("netlink: dump failed"))

	socks := senderOrSkip(t, local)

	sock, err := socks.get(local)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if dev := boundDevice(t, sock); dev != "" {
		t.Errorf("End Marker socket bound to %q, want unbound fallback on resolution error", dev)
	}
}

func queuedBytes(t *testing.T, sock *net.UDPConn) int {
	t.Helper()

	raw, err := sock.SyscallConn()
	if err != nil {
		t.Fatalf("syscall conn: %v", err)
	}

	var (
		queued  int
		ctrlErr error
	)

	if err := raw.Control(func(fd uintptr) {
		queued, ctrlErr = unix.IoctlGetInt(int(fd), unix.SIOCINQ)
	}); err != nil {
		t.Fatalf("control: %v", err)
	}

	if ctrlErr != nil {
		t.Fatalf("SIOCINQ: %v", ctrlErr)
	}

	return queued
}

func TestEndMarkerSocketDrainsInboundGTPU(t *testing.T) {
	local := netip.MustParseAddr("127.0.0.1")

	socks := senderOrSkip(t, local)

	sock, err := socks.get(local)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	peer, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("bind a peer: %v", err)
	}

	defer func() { _ = peer.Close() }()

	to := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: gtpuPort}

	for range 512 {
		if _, err := peer.WriteToUDP(endMarkerPDU(0x99), to); err != nil {
			t.Fatalf("write to the End Marker socket: %v", err)
		}
	}

	deadline := time.Now().Add(2 * time.Second)
	for queuedBytes(t, sock) != 0 {
		if time.Now().After(deadline) {
			t.Fatal("the End Marker socket still holds queued datagrams: nothing drains it, so inbound GTP-U the datapath passes to the stack accumulates until the receive buffer overflows")
		}

		time.Sleep(5 * time.Millisecond)
	}
}
