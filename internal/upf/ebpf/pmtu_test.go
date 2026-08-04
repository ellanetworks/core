// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package ebpf

import (
	"bytes"
	"encoding/binary"
	"net/netip"
	"testing"
	"time"
)

// TestDownlinkPMTU checks the path-MTU safety net: a downlink packet that would
// exceed the N3 MTU once GTP-encapsulated, with the Don't-Fragment bit set, is
// answered with an ICMP "fragmentation needed" toward the sender instead of
// being forwarded. The advertised next-hop MTU is the N3 MTU minus the GTP
// encapsulation overhead.
func TestDownlinkPMTU(t *testing.T) {
	requireProgTestRun(t)

	const (
		teid  = 0x504D5455 // "PMTU"
		n3MTU = 1280
	)

	f := setupT2(t, false)
	putDownlinkPDR(t, f.obj, ueIP, teid, testUPFN3IP, testGNBIP, 7)

	// Shrink the N3 MTU so the oversized downlink packet trips bpf_check_mtu.
	// The runtime check reads the live device MTU, so no reload is needed.
	if out, err := ipCmd("link", "set", t2N3Dev, "mtu", "1280"); err != nil {
		t.Fatalf("shrink N3 MTU: %v: %s", err, out)
	}

	// The ICMP error reflects back out the N6 side toward the sender.
	capFD := f.captureN6(t)

	big := withDF(ipv4Packet(serverIP, ueIP, 17, udpDatagram(4000, 4001, bytesOf(1400))))
	f.injectDownlink(t, ethFrame(0x0800, big))

	got := captureMatching(capFD, time.Second, func(fr []byte) bool {
		if !isInnerIPv4(fr, 1 /* ICMP */, serverIP) {
			return false
		}

		icmp := fr[ethHdrLen+20:]

		return len(icmp) >= 8 && icmp[0] == 3 && icmp[1] == 4 // dest-unreach / frag-needed
	})
	if got == nil {
		t.Fatal("did not capture an ICMP fragmentation-needed on the N6 side")
	}

	ip := got[ethHdrLen : ethHdrLen+20]
	icmp := got[ethHdrLen+20:]

	// A router sources an ICMP error from its own address (RFC 792), not from
	// the destination the sender was trying to reach: the UE address is
	// private here and would not survive BCP38 filtering on N6.
	if !bytes.Equal(ip[12:16], natPublicIP[:]) {
		t.Errorf("ICMP source = %v, want %v (the UPF's N6 address)", ip[12:16], natPublicIP)
	}

	if !validIPv4Checksum(ip) {
		t.Error("ICMP IPv4 header checksum invalid")
	}

	if !validICMPChecksum(icmp) {
		t.Error("ICMP checksum invalid")
	}

	if adv := binary.BigEndian.Uint16(icmp[6:8]); adv != n3MTU-gtpV4EncapLen {
		t.Errorf("advertised next-hop MTU = %d, want %d (N3 MTU - GTP overhead)", adv, n3MTU-gtpV4EncapLen)
	}

	// The ICMP embeds the original IPv4 header so the sender can match the flow.
	if embedded := icmp[8:]; len(embedded) < 20 || !bytes.Equal(embedded[16:20], ueIP[:]) {
		t.Error("ICMP does not embed the original packet's IPv4 header")
	}
}

// TestDownlinkPMTUMasquerade checks that with NAT enabled the ICMP
// fragmentation-needed quotes the packet as the server sent it: the server
// matches the error against its own socket, and the UE address must not
// appear on N6.
func TestDownlinkPMTUMasquerade(t *testing.T) {
	requireProgTestRun(t)

	const (
		ulTEID = 0x504D5501
		dlTEID = 0x504D5502
		ueSP   = 1234
		srvDP  = 4000
	)

	f := setupT2(t, true)
	putForwardingUplinkPDRUE(t, f.obj, ulTEID, 0, netip.AddrFrom4(ueIP), netip.Addr{})
	putDownlinkPDR(t, f.obj, ueIP, dlTEID, testUPFN3IP, testGNBIP, 7)

	// Uplink first so the reply has a conntrack mapping to translate.
	uplink := ipv4Packet(ueIP, serverIP, 17, udpDatagramChecksummed(ueIP, serverIP, ueSP, srvDP, []byte{1}))
	f.injectUplink(t, uplinkGPDU(ulTEID, uplink))
	time.Sleep(100 * time.Millisecond)

	if out, err := ipCmd("link", "set", t2N3Dev, "mtu", "1280"); err != nil {
		t.Fatalf("shrink N3 MTU: %v: %s", err, out)
	}

	capFD := f.captureN6(t)

	big := withDF(ipv4Packet(serverIP, natPublicIP, 17, udpDatagramChecksummed(serverIP, natPublicIP, srvDP, ueSP, bytesOf(1400))))
	f.injectDownlink(t, ethFrame(0x0800, big))

	got := captureMatching(capFD, time.Second, func(fr []byte) bool {
		if !isInnerIPv4(fr, 1 /* ICMP */, serverIP) {
			return false
		}

		icmp := fr[ethHdrLen+20:]

		return len(icmp) >= 8 && icmp[0] == 3 && icmp[1] == 4
	})
	if got == nil {
		t.Fatal("did not capture an ICMP fragmentation-needed on the N6 side")
	}

	ip := got[ethHdrLen : ethHdrLen+20]
	icmp := got[ethHdrLen+20:]

	if !bytes.Equal(ip[12:16], natPublicIP[:]) {
		t.Errorf("ICMP source = %v, want %v (the address the sender targeted)", ip[12:16], natPublicIP)
	}

	if !validIPv4Checksum(ip) {
		t.Error("ICMP IPv4 header checksum invalid")
	}

	if !validICMPChecksum(icmp) {
		t.Error("ICMP checksum invalid")
	}

	embedded := icmp[8:]
	if len(embedded) < 28 {
		t.Fatalf("ICMP quote too short: %d bytes", len(embedded))
	}

	if !bytes.Equal(embedded[16:20], natPublicIP[:]) {
		t.Errorf("quoted destination = %v, want %v (pre-translation)", embedded[16:20], natPublicIP)
	}

	if dp := binary.BigEndian.Uint16(embedded[22:24]); dp != ueSP {
		t.Errorf("quoted destination port = %d, want %d (pre-translation)", dp, ueSP)
	}
}

// TestDownlinkPMTUIPv6 is the IPv6 counterpart: an oversized downlink IPv6
// packet is answered with an ICMPv6 "packet too big" toward the sender.
func TestDownlinkPMTUIPv6(t *testing.T) {
	requireProgTestRun(t)

	const (
		teid  = 0x504D5436
		qfi   = 7
		n3MTU = 1280
	)

	serverV6 := [16]byte{0x20, 0x01, 0x48, 0x60, 0x48, 0x60, 0, 0, 0, 0, 0, 0, 0, 0, 0x88, 0x88}

	f := setupT2(t, false)
	putDownlinkPDRv6UE(t, f.obj, netip.MustParseAddr("2001:db8::"), teid, testUPFN3IP, testGNBIP, qfi)

	if out, err := ipCmd("link", "set", t2N3Dev, "mtu", "1280"); err != nil {
		t.Fatalf("shrink N3 MTU: %v: %s", err, out)
	}

	capFD := f.captureN6(t)

	big := ipv6Packet(serverV6, testUEv6, 17, udpDatagram(4000, 53, bytesOf(1300)))
	f.injectDownlink(t, ethFrame(0x86DD, big))

	got := captureMatching(capFD, time.Second, func(fr []byte) bool {
		if len(fr) < ethHdrLen+40+8 || fr[12] != 0x86 || fr[13] != 0xDD {
			return false
		}

		return fr[ethHdrLen+6] == 58 && fr[ethHdrLen+40] == 2 // ICMPv6 Packet Too Big
	})
	if got == nil {
		t.Fatal("did not capture an ICMPv6 packet-too-big on the N6 side")
	}

	var src, dst [16]byte
	copy(src[:], got[ethHdrLen+8:ethHdrLen+24])
	copy(dst[:], got[ethHdrLen+24:ethHdrLen+40])
	icmp6 := got[ethHdrLen+40:]

	// RFC 4443 §2.2: the source must be a unicast address belonging to the
	// node. The UE address the sender targeted is neither ours nor routable
	// back through N6.
	wantSrc := netip.MustParseAddr(t2N6IPv6).As16()
	if src != wantSrc {
		t.Errorf("ICMPv6 source = %x, want %x (the UPF's N6 address)", src, wantSrc)
	}

	if dst != serverV6 {
		t.Errorf("ICMPv6 dest = %x, want %x (the sender)", dst, serverV6)
	}

	if !validICMPv6Checksum(src, dst, icmp6) {
		t.Error("ICMPv6 checksum invalid")
	}

	if adv := binary.BigEndian.Uint32(icmp6[4:8]); adv != n3MTU-gtpV4EncapLen {
		t.Errorf("advertised MTU = %d, want %d (N3 MTU - GTP overhead)", adv, n3MTU-gtpV4EncapLen)
	}

	// The ICMPv6 embeds the original IPv6 header (dst = the UE prefix address).
	if embedded := icmp6[8:]; len(embedded) < 40 || !bytes.Equal(embedded[24:40], testUEv6[:]) {
		t.Error("ICMPv6 does not embed the original packet's IPv6 header")
	}
}
