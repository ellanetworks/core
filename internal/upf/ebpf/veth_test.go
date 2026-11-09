// SPDX-FileCopyrightText: Ella Networks Inc.
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
)

const (
	vethInjDev  = "ellvethx"
	vethInjPeer = "ellveths"
	vethN3Dev   = "ellvethn3"
	vethN3Peer  = "ellvethn3p"
)

func TestVethRAEncapsulation(t *testing.T) {
	requireProgTestRun(t)

	const (
		teid = 0x56455448
		qfi  = 6
	)

	injPeer, n3Peer, vobj := setupVethRA(t)

	addAddr(t, vethN3Dev, addrCIDR(testUPFN3IP))
	addNeigh(t, vethN3Dev, testGNBIP, "02:00:00:00:00:aa")

	ueDst := netip.MustParseAddr("fe80::1")
	putRASession(t, vobj, ueDst, ipv4OuterDownlinkPDR(teid, testUPFN3IP, testGNBIP, qfi))

	capFD := openCapture(t, n3Peer.Index)

	ra := ipv6Packet(testUPFN3v6, ueDst.As16(), 58, routerAdvertisement(testUPFN3v6, ueDst.As16()))
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
	putRASession(t, vobj, ueDst, ipv6OuterDownlinkPDR(teid, testUPFN3v6, testGNBv6, qfi))

	capFD := openCapture(t, n3Peer.Index)

	ra := ipv6Packet(testUPFN3v6, ueDst.As16(), 58, routerAdvertisement(testUPFN3v6, ueDst.As16()))
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

var raSessionPrefix = netip.MustParsePrefix("2001:db8:5::/64")

func putRASession(t *testing.T, vobj *BpfObjects, ueDst netip.Addr, pdr PdrInfo) {
	t.Helper()

	if err := vobj.PutPdrDownlink(raSessionPrefix.Addr(), pdr); err != nil {
		t.Fatalf("install the session's downlink PDR: %v", err)
	}

	if err := vobj.PutTunnel(ueDst, raSessionPrefix); err != nil {
		t.Fatalf("put tunnel: %v", err)
	}
}

// TS 23.501 §5.8.2.7
func TestVethRAFollowsTheSessionDownlinkFAR(t *testing.T) {
	requireProgTestRun(t)

	injPeer, n3Peer, vobj := setupVethRA(t)

	addAddr(t, vethN3Dev, addrCIDR(testUPFN3IP))
	addNeigh(t, vethN3Dev, testGNBIP, "02:00:00:00:00:aa")

	ueDst := netip.MustParseAddr("fe80::3")

	signalling := ipv4OuterDownlinkPDR(0x56455450, testUPFN3IP, testGNBIP, 1)
	signalling.Far.TransportLevelMarking = uint16(0xA0|0x03)<<8 | 0xFC

	putRASession(t, vobj, ueDst, signalling)

	ra := ipv6Packet(testUPFN3v6, ueDst.As16(), 58, routerAdvertisement(testUPFN3v6, ueDst.As16()))

	sendRA := func() (gtpV4Frame, byte) {
		t.Helper()

		capFD := openCapture(t, n3Peer.Index)

		inject(t, injPeer.Index, ethFrame(0x86DD, ra))

		got := captureMatching(capFD, time.Second, isGTPv4Outer)
		if got == nil {
			t.Fatal("did not capture a GTP-encapsulated RA on the N3 side")
		}

		return parseGTPv4Frame(t, got), got[ethHdrLen+1]
	}

	f, tos := sendRA()

	if tos != 0xA0 {
		t.Errorf("outer IPv4 TOS = %#02x, want the DSCP bits of the session's downlink FAR 0xa0", tos)
	}

	if f.teid != signalling.Far.TeID || f.qfi != 1 {
		t.Errorf("RA tunnelled on TEID %#x QFI %d, want the session's TEID %#x QFI 1", f.teid, f.qfi, signalling.Far.TeID)
	}

	if !f.outerChecksumOK {
		t.Error("outer IPv4 header checksum invalid")
	}

	switched := ipv4OuterDownlinkPDR(0x56455451, testUPFN3IP, testGNBIP, 2)
	switched.Far.TransportLevelMarking = 0xB8FC

	if err := vobj.PutPdrDownlink(raSessionPrefix.Addr(), switched); err != nil {
		t.Fatalf("update the session's downlink PDR: %v", err)
	}

	f, tos = sendRA()

	if tos != 0xB8 {
		t.Errorf("outer IPv4 TOS after the FAR update = %#02x, want 0xb8", tos)
	}

	if f.teid != switched.Far.TeID || f.qfi != 2 {
		t.Errorf("RA tunnelled on TEID %#x QFI %d after the FAR update, want TEID %#x QFI 2", f.teid, f.qfi, switched.Far.TeID)
	}
}

func TestVethRADroppedWhileTheDownlinkDoesNotForward(t *testing.T) {
	requireProgTestRun(t)

	_, _, vobj := setupVethRA(t)

	ueDst := netip.MustParseAddr("fe80::4")

	buffering := ipv4OuterDownlinkPDR(0x56455452, testUPFN3IP, testGNBIP, 1)
	buffering.Far.Action = farBuffNocp

	putRASession(t, vobj, ueDst, buffering)

	ra := ipv6Packet(testUPFN3v6, ueDst.As16(), 58, routerAdvertisement(testUPFN3v6, ueDst.As16()))

	wantAction := uint32(ActionDrop)
	if vobj.UseTCX {
		wantAction = tcActShot
	}

	if action := runXDP(t, vobj.VethXdpFunc, ethFrame(0x86DD, ra)); action != wantAction {
		t.Fatalf("RA for a buffering session got action %d, want drop action %d", action, wantAction)
	}

	if err := vobj.DeleteTunnel(ueDst); err != nil {
		t.Fatalf("delete tunnel: %v", err)
	}

	if err := vobj.PutTunnel(ueDst, netip.MustParsePrefix("2001:db8:6::/64")); err != nil {
		t.Fatalf("put tunnel: %v", err)
	}

	if action := runXDP(t, vobj.VethXdpFunc, ethFrame(0x86DD, ra)); action != wantAction {
		t.Fatalf("RA for a prefix with no downlink PDR got action %d, want drop action %d", action, wantAction)
	}
}

func setupVethRA(t *testing.T) (injPeer, n3Peer *net.Interface, vobj *BpfObjects) {
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

	obj := loadAttachedProgramConfig(t, false, false, ifByName(t, vethN3Dev).Index, 0, 0, 0)

	attachDatapath(t, obj, obj.VethXdpFunc, injDev.Index)

	return ifByName(t, vethInjPeer), ifByName(t, vethN3Peer), obj
}

func isGTPv4Outer(fr []byte) bool {
	if len(fr) < ethHdrLen+gtpV4EncapLen || fr[12] != 0x08 || fr[13] != 0x00 {
		return false
	}

	return fr[ethHdrLen+9] == 17 && binary.BigEndian.Uint16(fr[ethHdrLen+22:ethHdrLen+24]) == GTPUDPPort
}

func routerAdvertisement(src, dst [16]byte) []byte {
	ra := make([]byte, 16)
	ra[0] = 134

	return icmpv6Checksummed(src, dst, ra)
}
