// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package ebpf

import (
	"bytes"
	"encoding/binary"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/cilium/ebpf/link"
)

const (
	vethInjDev  = "ellvethx"  // veth_xdp_func attaches here (production: veth-xdp)
	vethInjPeer = "ellveths"  // RA injected here (production: veth-smf)
	vethN3Dev   = "ellvethn3" // the gNB route resolves here; encapsulated RA egresses here
	vethN3Peer  = "ellvethn3p"
)

// TestVethRAEncapsulation checks the veth XDP program (veth_xdp_func), the IPv6
// Router Advertisement / SLAAC injection path over an IPv4 N3 transport: an IPv6
// packet injected on the veth, matching a veth_tunnels entry, is GTP-U
// encapsulated toward the gNB and forwarded to the interface the routing table
// resolves for it. The program trusts the FIB result, so the test does not
// assume any particular configured ifindex — it relies only on the gNB route.
func TestVethRAEncapsulation(t *testing.T) {
	requireProgTestRun(t)

	const (
		teid = 0x56455448 // "VETH"
		qfi  = 6
	)

	injPeer, n3Peer, vobj := setupVethRA(t)

	addAddr(t, vethN3Dev, addrCIDR(testUPFN3IP, 24))
	addNeigh(t, vethN3Dev, testGNBIP, "02:00:00:00:00:aa")

	ueDst := netip.MustParseAddr("fe80::1")
	if err := vobj.PutTunnel(ueDst, VethTunnelInfo{
		TEID:       teid,
		LocalAddr:  netip.AddrFrom4(testUPFN3IP),
		RemoteAddr: netip.AddrFrom4(testGNBIP),
		QFI:        qfi,
	}); err != nil {
		t.Fatalf("put tunnel: %v", err)
	}

	capFD := openCapture(t, n3Peer.Index)

	ra := ipv6Packet(testUPFN3v6, ueDst.As16(), 58, routerAdvertisement())
	inject(t, injPeer.Index, ethFrame(0x86DD, ra))

	got := captureMatching(capFD, time.Second, isGTPv4Outer)
	if got == nil {
		t.Fatal("did not capture a GTP-encapsulated RA on the N3 side")
	}

	f := parseGTPv4Frame(t, got)

	if !f.outerChecksumOK {
		t.Error("outer IPv4 header checksum invalid")
	}

	if f.outerSrc != testUPFN3IP {
		t.Errorf("outer src = %v, want %v (tunnel local address)", f.outerSrc, testUPFN3IP)
	}

	if f.outerDst != testGNBIP {
		t.Errorf("outer dst = %v, want %v (tunnel remote address)", f.outerDst, testGNBIP)
	}

	if f.teid != teid {
		t.Errorf("TEID = %#x, want %#x", f.teid, uint32(teid))
	}

	if f.qfi != qfi {
		t.Errorf("QFI = %d, want %d", f.qfi, qfi)
	}

	if !bytes.Equal(f.inner, ra) {
		t.Errorf("inner RA altered by encapsulation:\n got %x\nwant %x", f.inner, ra)
	}
}

// TestVethRAEncapsulationIPv6Transport checks the same path over an IPv6 N3
// transport.
func TestVethRAEncapsulationIPv6Transport(t *testing.T) {
	requireProgTestRun(t)

	const (
		teid = 0x56455449
		qfi  = 7
	)

	injPeer, n3Peer, vobj := setupVethRA(t)

	upfV6 := netip.AddrFrom16(testUPFN3v6)
	gnbV6 := netip.AddrFrom16(testGNBv6)

	if out, err := ipCmd("addr", "add", upfV6.String()+"/64", "dev", vethN3Dev, "nodad"); err != nil {
		t.Fatalf("add N3 IPv6 addr: %v: %s", err, out)
	}

	if out, err := ipCmd("neigh", "add", gnbV6.String(), "dev", vethN3Dev, "lladdr", "02:00:00:00:00:bb", "nud", "permanent"); err != nil {
		t.Fatalf("add N3 IPv6 neigh: %v: %s", err, out)
	}

	ueDst := netip.MustParseAddr("fe80::2")
	if err := vobj.PutTunnel(ueDst, VethTunnelInfo{
		TEID:       teid,
		LocalAddr:  upfV6,
		RemoteAddr: gnbV6,
		QFI:        qfi,
	}); err != nil {
		t.Fatalf("put tunnel: %v", err)
	}

	capFD := openCapture(t, n3Peer.Index)

	ra := ipv6Packet(testUPFN3v6, ueDst.As16(), 58, routerAdvertisement())
	inject(t, injPeer.Index, ethFrame(0x86DD, ra))

	got := captureMatching(capFD, time.Second, func(fr []byte) bool {
		return len(fr) >= ethHdrLen+gtpV6EncapLen && fr[12] == 0x86 && fr[13] == 0xDD &&
			fr[ethHdrLen+6] == 17 && binary.BigEndian.Uint16(fr[ethHdrLen+40+2:ethHdrLen+40+4]) == GTPUDPPort
	})
	if got == nil {
		t.Fatal("did not capture a GTP-over-IPv6 encapsulated RA on the N3 side")
	}

	f := parseGTPv6Frame(t, got)

	if !f.udpChecksumOK {
		t.Error("outer UDP-over-IPv6 checksum invalid")
	}

	if f.outerSrc != testUPFN3v6 {
		t.Errorf("outer src = %x, want %x (tunnel local address)", f.outerSrc, testUPFN3v6)
	}

	if f.outerDst != testGNBv6 {
		t.Errorf("outer dst = %x, want %x (tunnel remote address)", f.outerDst, testGNBv6)
	}

	if f.teid != teid {
		t.Errorf("TEID = %#x, want %#x", f.teid, uint32(teid))
	}

	if f.qfi != qfi {
		t.Errorf("QFI = %d, want %d", f.qfi, qfi)
	}

	if !bytes.Equal(f.inner, ra) {
		t.Errorf("inner RA altered by encapsulation:\n got %x\nwant %x", f.inner, ra)
	}
}

// setupVethRA builds the injection and N3 veth pairs, enables forwarding, and
// loads + attaches the veth program exactly as production does (no configured
// ifindex — the program forwards to whatever interface the FIB resolves).
func setupVethRA(t *testing.T) (injPeer, n3Peer *net.Interface, vobj *VethBpfObjects) {
	t.Helper()

	_, _ = ipCmd("link", "del", vethInjDev)
	_, _ = ipCmd("link", "del", vethN3Dev)

	addVethPair(t, vethInjDev, vethInjPeer)
	addVethPair(t, vethN3Dev, vethN3Peer)

	if err := writeSysctl("net.ipv4.ip_forward", "1"); err != nil {
		t.Fatalf("enable ip_forward: %v", err)
	}

	_ = writeSysctl("net.ipv6.conf.all.forwarding", "1")

	injDev := ifByName(t, vethInjDev)

	obj, err := LoadVethBpfObjects()
	if err != nil {
		t.Fatalf("load veth objects: %v", err)
	}

	t.Cleanup(func() { _ = obj.Close() })

	l, err := link.AttachXDP(link.XDPOptions{
		Program:   obj.VethXdpFunc,
		Interface: injDev.Index,
		Flags:     link.XDPGenericMode,
	})
	if err != nil {
		t.Fatalf("attach XDP to veth: %v", err)
	}

	t.Cleanup(func() { _ = l.Close() })

	return ifByName(t, vethInjPeer), ifByName(t, vethN3Peer), obj
}

func isGTPv4Outer(fr []byte) bool {
	if len(fr) < ethHdrLen+gtpV4EncapLen || fr[12] != 0x08 || fr[13] != 0x00 {
		return false
	}

	return fr[ethHdrLen+9] == 17 && binary.BigEndian.Uint16(fr[ethHdrLen+22:ethHdrLen+24]) == GTPUDPPort
}

// routerAdvertisement builds a minimal ICMPv6 Router Advertisement message
// (type 134). The veth program treats it as opaque inner payload.
func routerAdvertisement() []byte {
	ra := make([]byte, 16)
	ra[0] = 134 // Router Advertisement

	return ra
}
