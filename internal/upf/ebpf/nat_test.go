// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package ebpf

import (
	"bytes"
	"encoding/binary"
	"net/netip"
	"strconv"
	"testing"
	"time"
)

// natProto describes one transport protocol for the NAT tests: the UE-side L4
// ports, how to build an inner L4 segment, and how to validate its checksum.
type natProto struct {
	name  string
	num   uint8
	sport uint16 // UE source port (uplink); 0 for ICMP
	dport uint16 // server port (uplink destination); 0 for ICMP
	build func(src, dst [4]byte, sport, dport uint16, payload []byte) []byte
	valid func(src, dst [4]byte, l4 []byte) bool
}

const natICMPID = 0xbeef

// Mirrors NAT_CT_CLOSED_UE / NAT_CT_CLOSED_REMOTE in nat.h.
const (
	natClosedUE     = 0x1
	natClosedRemote = 0x2
)

var natProtos = []natProto{
	{
		name:  "tcp",
		num:   6,
		sport: 1234,
		dport: 80,
		build: func(src, dst [4]byte, sp, dp uint16, p []byte) []byte {
			return tcpSegmentChecksummed(src, dst, sp, dp, p)
		},
		valid: func(src, dst [4]byte, l4 []byte) bool { return validIPv4L4Checksum(src, dst, 6, l4) },
	},
	{
		name:  "udp",
		num:   17,
		sport: 1234,
		dport: 53,
		build: func(src, dst [4]byte, sp, dp uint16, p []byte) []byte {
			return udpDatagramChecksummed(src, dst, sp, dp, p)
		},
		valid: func(src, dst [4]byte, l4 []byte) bool { return validIPv4L4Checksum(src, dst, 17, l4) },
	},
	{
		name:  "icmp",
		num:   1,
		build: func(_, _ [4]byte, _, _ uint16, p []byte) []byte { return icmpEchoRequest(natICMPID, 1, p) },
		valid: func(_, _ [4]byte, l4 []byte) bool { return validICMPChecksum(l4) },
	},
}

// l4ChecksumOffset returns the byte offset of the L4 checksum field for proto,
// or -1 if not applicable.
func l4ChecksumOffset(num uint8) int {
	switch num {
	case 6:
		return 16 // TCP
	case 17:
		return 6 // UDP
	case 1:
		return 2 // ICMP
	default:
		return -1
	}
}

// l4PreservedExceptChecksum reports whether got equals orig once the L4
// checksum field (the only L4 byte source-NAT legitimately rewrites) is masked.
// It catches corruption of ports, sequence numbers, flags, or payload that a
// checksum-only assertion would miss.
func l4PreservedExceptChecksum(num uint8, got, orig []byte) bool {
	if len(got) != len(orig) {
		return false
	}

	g := bytes.Clone(got)
	o := bytes.Clone(orig)

	if off := l4ChecksumOffset(num); off >= 0 && off+2 <= len(g) {
		g[off], g[off+1] = 0, 0
		o[off], o[off+1] = 0, 0
	}

	return bytes.Equal(g, o)
}

// payloadSizes brackets the historical bpf_csum_diff 512-byte limit and pushes
// past a typical MTU, so a size-dependent checksum defect cannot hide.
var payloadSizes = []int{0, 100, 500, 512, 600, 1000, 1400}

// TestSourceNATUplink verifies uplink source-NAT across protocols and payload
// sizes: the decapsulated inner packet leaves the N6 side with its source
// rewritten to the egress address and valid IPv4 and L4 checksums.
func TestSourceNATUplink(t *testing.T) {
	requireProgTestRun(t)

	const teid = 0x4E415431

	f := setupT2(t, true)
	putForwardingUplinkPDRUE(t, f.obj, teid, 0, netip.AddrFrom4(ueIP), netip.Addr{})

	for _, proto := range natProtos {
		for _, size := range payloadSizes {
			t.Run(proto.name+"/"+sizeLabel(size), func(t *testing.T) {
				capFD := f.captureN6(t)

				origL4 := proto.build(ueIP, serverIP, proto.sport, proto.dport, bytesOf(size))
				inner := ipv4Packet(ueIP, serverIP, proto.num, origL4)
				f.injectUplink(t, uplinkGPDU(teid, inner))

				got := captureMatching(capFD, time.Second, func(fr []byte) bool {
					return isInnerIPv4(fr, proto.num, serverIP)
				})
				if got == nil {
					t.Fatal("did not capture a NAT'd packet on the N6 side")
				}

				assertSourceNATd(t, got, proto, origL4)
			})
		}
	}
}

// TestNATRoundTrip verifies the full NAT conntrack cycle: an uplink packet
// establishes the mapping, then the matching downlink reply is destination-NAT'd
// back to the UE address, re-encapsulated toward the gNB, and leaves the N3 side
// with valid checksums and the UE as the inner destination.
func TestNATRoundTrip(t *testing.T) {
	requireProgTestRun(t)

	const (
		ulTEID = 0x52545431
		dlTEID = 0x52545432
		qfi    = 7
	)

	f := setupT2(t, true)
	putForwardingUplinkPDRUE(t, f.obj, ulTEID, 0, netip.AddrFrom4(ueIP), netip.Addr{})
	putDownlinkPDR(t, f.obj, ueIP, dlTEID, testUPFN3IP, testGNBIP, qfi)

	for _, proto := range natProtos {
		for _, size := range []int{40, 1000} {
			t.Run(proto.name+"/"+sizeLabel(size), func(t *testing.T) {
				// Uplink first: establish the conntrack mapping.
				uplinkInner := ipv4Packet(ueIP, serverIP, proto.num, proto.build(ueIP, serverIP, proto.sport, proto.dport, bytesOf(40)))
				f.injectUplink(t, uplinkGPDU(ulTEID, uplinkInner))

				time.Sleep(100 * time.Millisecond)

				// Capture on N3 only after the uplink egress (which lands on N6)
				// has drained, so this socket sees only the downlink reply.
				capFD := f.captureN3(t)

				reply := ipv4Packet(serverIP, natPublicIP, proto.num, downlinkReply(proto, bytesOf(size)))
				f.injectDownlink(t, ethFrame(0x0800, reply))

				got := captureMatching(capFD, time.Second, func(fr []byte) bool {
					inner := gtpInner(fr)

					return inner != nil && inner[9] == proto.num
				})
				if got == nil {
					t.Fatal("did not capture a re-encapsulated downlink packet on the N3 side")
				}

				assertDestinationNATd(t, got, proto)
			})
		}
	}
}

// TestNATIPv6PassThrough checks that with NAT enabled, an uplink IPv6 packet is
// decapsulated unchanged: IPv6 traffic is never source-NAT'd (each UE owns its
// own /64). This is a T1 verdict/transform check; routing is host-dependent.
func TestNATIPv6PassThrough(t *testing.T) {
	requireProgTestRun(t)

	const teid = 0x4E363650 // "N66P"

	obj := loadProgramConfig(t, false, true /* masquerade */, 0, 1, 0, 0)
	putForwardingUplinkPDR(t, obj, teid, 0)

	inner := innerIPv6UDP(testUEv6, 53)

	action, out := runXDPOut(t, obj.UpfEntryFunc, uplinkGPDU(teid, inner))
	if action == XDP_DROP || action == XDP_ABORTED {
		t.Fatalf("IPv6 uplink with NAT enabled got XDP action %d, want a forwarding action", action)
	}

	if !bytes.Equal(out[ethHdrLen:], inner) {
		t.Errorf("IPv6 inner packet altered with NAT enabled (must be untouched):\n got %x\nwant %x", out[ethHdrLen:], inner)
	}
}

// TestNATPortCollision checks source-port remapping: when two UEs use the same
// source port to the same destination, the second flow is rewritten to a
// different egress port (the first keeps its port).
func TestNATPortCollision(t *testing.T) {
	requireProgTestRun(t)

	const (
		teid1   = 0x4E415401
		teid2   = 0x4E415402
		srcPort = 1234
	)

	ue1 := ueIP
	ue2 := [4]byte{10, 45, 0, 2}

	f := setupT2(t, true)
	putForwardingUplinkPDRUE(t, f.obj, teid1, 0, netip.AddrFrom4(ue1), netip.Addr{})
	putForwardingUplinkPDRUE(t, f.obj, teid2, 0, netip.AddrFrom4(ue2), netip.Addr{})

	egressPort := func(t *testing.T, ueSrc [4]byte, teid uint32) uint16 {
		t.Helper()

		capFD := f.captureN6(t)

		inner := ipv4Packet(ueSrc, serverIP, 6, tcpSegmentChecksummed(ueSrc, serverIP, srcPort, 80, []byte{1, 2}))
		f.injectUplink(t, uplinkGPDU(teid, inner))

		got := captureMatching(capFD, time.Second, func(fr []byte) bool {
			return isInnerIPv4(fr, 6, serverIP)
		})
		if got == nil {
			t.Fatal("did not capture a NAT'd packet on the N6 side")
		}

		ip := got[ethHdrLen : ethHdrLen+20]
		tcp := got[ethHdrLen+20:]

		if !bytes.Equal(ip[12:16], natPublicIP[:]) {
			t.Errorf("inner src = %v, want %v (source-NAT'd)", ip[12:16], natPublicIP)
		}

		if !bytes.Equal(ip[16:20], serverIP[:]) {
			t.Errorf("inner dst = %v, want %v (preserved)", ip[16:20], serverIP)
		}

		if dp := binary.BigEndian.Uint16(tcp[2:4]); dp != 80 {
			t.Errorf("inner dest port = %d, want 80 (only the source port may be remapped)", dp)
		}

		if !validIPv4L4Checksum(natPublicIP, serverIP, 6, tcp) {
			t.Error("inner TCP checksum invalid after NAT")
		}

		return binary.BigEndian.Uint16(tcp[0:2])
	}

	port1 := egressPort(t, ue1, teid1)
	if port1 != srcPort {
		t.Errorf("first UE egress port = %d, want %d (no remap needed)", port1, srcPort)
	}

	time.Sleep(100 * time.Millisecond)

	port2 := egressPort(t, ue2, teid2)
	if port2 == srcPort {
		t.Errorf("second UE egress port = %d, want a remapped port (collision not resolved)", port2)
	}
}

// TestNATICMPError checks that an inbound ICMP error (destination unreachable)
// is destination-NAT'd: the outer destination and the embedded original packet
// are both rewritten to the UE so the UE can match the error to its flow.
func TestNATICMPError(t *testing.T) {
	requireProgTestRun(t)

	const (
		ulTEID = 0x4E415403
		dlTEID = 0x4E415404
		qfi    = 7
		ueSP   = 1234
		srvDP  = 53
	)

	f := setupT2(t, true)
	putForwardingUplinkPDRUE(t, f.obj, ulTEID, 0, netip.AddrFrom4(ueIP), netip.Addr{})
	putDownlinkPDR(t, f.obj, ueIP, dlTEID, testUPFN3IP, testGNBIP, qfi)

	// Establish the conntrack mapping with an uplink UDP packet.
	uplinkInner := ipv4Packet(ueIP, serverIP, 17, udpDatagramChecksummed(ueIP, serverIP, ueSP, srvDP, []byte{9, 9}))
	f.injectUplink(t, uplinkGPDU(ulTEID, uplinkInner))

	time.Sleep(100 * time.Millisecond)

	capFD := f.captureN3(t)

	// Server returns an ICMP port-unreachable embedding the NAT'd packet
	// (src = the public address, the UDP header the server saw).
	embeddedUDP := udpDatagramChecksummed(natPublicIP, serverIP, ueSP, srvDP, nil)
	embeddedIP := ipv4Packet(natPublicIP, serverIP, 17, embeddedUDP)

	icmpErr := make([]byte, 8+len(embeddedIP))
	icmpErr[0] = 3 // destination unreachable
	icmpErr[1] = 3 // port unreachable
	copy(icmpErr[8:], embeddedIP)
	binary.BigEndian.PutUint16(icmpErr[2:4], onesComplement16(icmpErr))

	f.injectDownlink(t, ethFrame(0x0800, ipv4Packet(serverIP, natPublicIP, 1, icmpErr)))

	got := captureMatching(capFD, time.Second, func(fr []byte) bool {
		inner := gtpInner(fr)

		return inner != nil && inner[9] == 1 // ICMP
	})
	if got == nil {
		t.Fatal("did not capture a re-encapsulated ICMP error on the N3 side")
	}

	inner := gtpInner(got)
	icmp := inner[20:]

	if !bytes.Equal(inner[16:20], ueIP[:]) {
		t.Errorf("ICMP error inner dst = %v, want %v (destination-NAT'd to UE)", inner[16:20], ueIP)
	}

	if !validIPv4Checksum(inner[:20]) {
		t.Error("inner IPv4 header checksum invalid after NAT")
	}

	if !validICMPChecksum(icmp) {
		t.Error("ICMP checksum invalid after NAT")
	}

	// The embedded original packet's source must be rewritten to the UE, and
	// its destination (the server) left untouched.
	embedded := icmp[8:]
	if len(embedded) < 28 || !bytes.Equal(embedded[12:16], ueIP[:]) {
		t.Fatalf("embedded packet source not rewritten to the UE: %x", embedded)
	}

	if !bytes.Equal(embedded[16:20], serverIP[:]) {
		t.Errorf("embedded packet dst = %v, want %v (must be preserved)", embedded[16:20], serverIP)
	}

	if !validIPv4Checksum(embedded[:20]) {
		t.Error("embedded IPv4 header checksum invalid after NAT")
	}

	if !validIPv4L4Checksum(ueIP, serverIP, 17, embedded[20:28]) {
		t.Error("embedded UDP checksum invalid after NAT")
	}
}

// TestNATUnsolicitedInboundDrop verifies that with masquerade enabled, a
// downlink packet addressed directly to the UE address with no conntrack entry
// is dropped: nothing egresses on N3 and nat_unsolicited_drop_ip4 increments.
func TestNATUnsolicitedInboundDrop(t *testing.T) {
	requireProgTestRun(t)

	const (
		dlTEID = 0x4E415405
		qfi    = 7
	)

	f := setupT2(t, true)
	putDownlinkPDR(t, f.obj, ueIP, dlTEID, testUPFN3IP, testGNBIP, qfi)

	for _, proto := range natProtos {
		t.Run(proto.name, func(t *testing.T) {
			capFD := f.captureN3(t)

			before := GetN6NatUnsolicitedDropIPv4(f.obj)

			l4 := proto.build(serverIP, ueIP, proto.dport, proto.sport, []byte{1, 2})
			f.injectDownlink(t, ethFrame(0x0800, ipv4Packet(serverIP, ueIP, proto.num, l4)))

			if got := captureMatching(capFD, 500*time.Millisecond, func(fr []byte) bool {
				return gtpInner(fr) != nil
			}); got != nil {
				t.Fatalf("unsolicited %s packet egressed on N3: %x", proto.name, got)
			}

			if after := GetN6NatUnsolicitedDropIPv4(f.obj); after != before+1 {
				t.Errorf("nat_unsolicited_drop_ip4 = %d, want %d", after, before+1)
			}
		})
	}
}

// TestUnsolicitedInboundForwardedWithoutNAT verifies that with masquerade
// disabled, a downlink packet addressed to the UE address is forwarded to N3:
// direct routing to UE addresses is the supported non-NAT mode.
func TestUnsolicitedInboundForwardedWithoutNAT(t *testing.T) {
	requireProgTestRun(t)

	const (
		dlTEID = 0x4E415406
		qfi    = 7
	)

	f := setupT2(t, false)
	putDownlinkPDR(t, f.obj, ueIP, dlTEID, testUPFN3IP, testGNBIP, qfi)

	for _, proto := range natProtos {
		t.Run(proto.name, func(t *testing.T) {
			capFD := f.captureN3(t)

			l4 := proto.build(serverIP, ueIP, proto.dport, proto.sport, []byte{1, 2})
			f.injectDownlink(t, ethFrame(0x0800, ipv4Packet(serverIP, ueIP, proto.num, l4)))

			got := captureMatching(capFD, time.Second, func(fr []byte) bool {
				inner := gtpInner(fr)

				return inner != nil && inner[9] == proto.num && bytes.Equal(inner[16:20], ueIP[:])
			})
			if got == nil {
				t.Fatalf("downlink %s packet to the UE address did not egress on N3 with NAT disabled", proto.name)
			}
		})
	}
}

// TestNATICMPErrorFromRouter checks that an ICMP error from an intermediate
// hop is translated to the UE: the flow is identified by the packet quoted in
// the error, not by who reported it, so PMTUD and traceroute work.
func TestNATICMPErrorFromRouter(t *testing.T) {
	requireProgTestRun(t)

	const (
		ulTEID = 0x4E41540E
		dlTEID = 0x4E41540F
		qfi    = 7
		ueSP   = 1234
		srvDP  = 53
	)

	routerIP := [4]byte{198, 51, 100, 9}

	f := setupT2(t, true)
	putForwardingUplinkPDRUE(t, f.obj, ulTEID, 0, netip.AddrFrom4(ueIP), netip.Addr{})
	putDownlinkPDR(t, f.obj, ueIP, dlTEID, testUPFN3IP, testGNBIP, qfi)

	uplinkInner := ipv4Packet(ueIP, serverIP, 17, udpDatagramChecksummed(ueIP, serverIP, ueSP, srvDP, []byte{9, 9}))
	f.injectUplink(t, uplinkGPDU(ulTEID, uplinkInner))
	time.Sleep(100 * time.Millisecond)

	capFD := f.captureN3(t)

	// A hop on the path reports TTL exceeded, quoting the NAT'd packet.
	embeddedUDP := udpDatagramChecksummed(natPublicIP, serverIP, ueSP, srvDP, nil)
	embeddedIP := ipv4Packet(natPublicIP, serverIP, 17, embeddedUDP)

	icmpErr := make([]byte, 8+len(embeddedIP))
	icmpErr[0] = 11 // time exceeded
	copy(icmpErr[8:], embeddedIP)
	binary.BigEndian.PutUint16(icmpErr[2:4], onesComplement16(icmpErr))

	f.injectDownlink(t, ethFrame(0x0800, ipv4Packet(routerIP, natPublicIP, 1, icmpErr)))

	got := captureMatching(capFD, time.Second, func(fr []byte) bool {
		inner := gtpInner(fr)

		return inner != nil && inner[9] == 1
	})
	if got == nil {
		t.Fatal("ICMP error from an intermediate hop was not delivered to the UE")
	}

	inner := gtpInner(got)
	icmp := inner[20:]

	if !bytes.Equal(inner[16:20], ueIP[:]) {
		t.Errorf("inner dst = %v, want %v (destination-NAT'd)", inner[16:20], ueIP)
	}

	if !bytes.Equal(inner[12:16], routerIP[:]) {
		t.Errorf("inner src = %v, want %v (the reporting hop, preserved)", inner[12:16], routerIP)
	}

	if !validIPv4Checksum(inner[:20]) {
		t.Error("inner IPv4 header checksum invalid after NAT")
	}

	if !validICMPChecksum(icmp) {
		t.Error("ICMP checksum invalid after NAT")
	}

	embedded := icmp[8:]
	if len(embedded) < 28 || !bytes.Equal(embedded[12:16], ueIP[:]) {
		t.Fatalf("embedded packet source not rewritten to the UE: %x", embedded)
	}
}

// natFiveTuple builds a nat_ct key as the datapath stores it: addresses and
// ports in network byte order, protocol as a plain value.
func natFiveTuple(saddr, daddr [4]byte, sport, dport uint16, proto uint16) N3N6EntrypointFiveTuple { //nolint:unparam // general key builder; ports are configurable
	return N3N6EntrypointFiveTuple{
		Saddr: binary.NativeEndian.Uint32(saddr[:]),
		Daddr: binary.NativeEndian.Uint32(daddr[:]),
		Sport: htons(sport),
		Dport: htons(dport),
		Proto: proto,
	}
}

// tcpSegmentWithFlags builds a checksummed 20-byte TCP segment with the given
// flag byte (FIN=0x01, SYN=0x02, RST=0x04, ACK=0x10).
func tcpSegmentWithFlags(src, dst [4]byte, sport, dport uint16, flags byte) []byte {
	seg := make([]byte, 20)

	binary.BigEndian.PutUint16(seg[0:2], sport)
	binary.BigEndian.PutUint16(seg[2:4], dport)
	seg[12] = 0x50 // data offset = 5
	seg[13] = flags
	binary.BigEndian.PutUint16(seg[16:18], ipv4L4Checksum(src, dst, 6, seg))

	return seg
}

func lookupNatEntry(t *testing.T, f *t2, key N3N6EntrypointFiveTuple) N3N6EntrypointNatEntry {
	t.Helper()

	var val N3N6EntrypointNatEntry
	if err := f.obj.NatCt.Lookup(&key, &val); err != nil {
		t.Fatalf("nat_ct lookup: %v", err)
	}

	return val
}

// TestNATConntrackDirectionAndState verifies the conntrack pair layout and
// state machine: SYN creates both entries (new, unreplied), a downlink hit
// sets replied, the next uplink packet establishes, FIN closes.
func TestNATConntrackDirectionAndState(t *testing.T) {
	requireProgTestRun(t)

	const (
		ulTEID = 0x4E415407
		dlTEID = 0x4E415408
		qfi    = 7
		ueSP   = 1234
		srvDP  = 80
	)

	f := setupT2(t, true)
	putForwardingUplinkPDRUE(t, f.obj, ulTEID, 0, netip.AddrFrom4(ueIP), netip.Addr{})
	putDownlinkPDR(t, f.obj, ueIP, dlTEID, testUPFN3IP, testGNBIP, qfi)

	ueKey := natFiveTuple(ueIP, serverIP, ueSP, srvDP, 6)
	natKey := natFiveTuple(natPublicIP, serverIP, ueSP, srvDP, 6)

	sendUplink := func(flags byte) {
		capFD := f.captureN6(t)
		seg := tcpSegmentWithFlags(ueIP, serverIP, ueSP, srvDP, flags)
		f.injectUplink(t, uplinkGPDU(ulTEID, ipv4Packet(ueIP, serverIP, 6, seg)))

		if captureMatching(capFD, time.Second, func(fr []byte) bool {
			return isInnerIPv4(fr, 6, serverIP)
		}) == nil {
			t.Fatal("uplink packet did not egress on N6")
		}
	}

	// SYN: both entries exist, UE-side is new and unreplied.
	sendUplink(0x02)

	ue := lookupNatEntry(t, f, ueKey)
	if ue.State != 0 || ue.Replied != 0 || ue.UeSide != 1 {
		t.Errorf("after SYN: state=%d replied=%d ue_side=%d, want state=0 replied=0 ue_side=1", ue.State, ue.Replied, ue.UeSide)
	}

	if ue.Src != natKey {
		t.Errorf("UE-side entry src = %+v, want the NAT-side tuple %+v", ue.Src, natKey)
	}

	natSide := lookupNatEntry(t, f, natKey)
	if natSide.Src != ueKey {
		t.Errorf("NAT-side entry src = %+v, want the UE tuple %+v", natSide.Src, ueKey)
	}

	if natSide.UeSide != 0 {
		t.Errorf("NAT-side entry ue_side = %d, want 0", natSide.UeSide)
	}

	synRefreshTs := ue.RefreshTs

	// SYN-ACK reply: replied is set and the lifetime refreshed.
	capFD := f.captureN3(t)
	f.injectDownlink(t, ethFrame(0x0800, ipv4Packet(serverIP, natPublicIP, 6, tcpSegmentWithFlags(serverIP, natPublicIP, srvDP, ueSP, 0x12))))

	if captureMatching(capFD, time.Second, func(fr []byte) bool { return gtpInner(fr) != nil }) == nil {
		t.Fatal("downlink SYN-ACK did not egress on N3")
	}

	if ue = lookupNatEntry(t, f, ueKey); ue.Replied != 1 {
		t.Errorf("after SYN-ACK: replied=%d, want 1", ue.Replied)
	}

	if ue.RefreshTs <= synRefreshTs {
		t.Errorf("after SYN-ACK: refresh_ts=%d not past %d (downlink hit must refresh)", ue.RefreshTs, synRefreshTs)
	}

	// ACK completing the handshake: established.
	sendUplink(0x10)

	if ue = lookupNatEntry(t, f, ueKey); ue.State != 1 {
		t.Errorf("after handshake ACK: state=%d, want 1 (established)", ue.State)
	}

	// UE FIN: the UE direction closes, the remote one stays open.
	sendUplink(0x11)

	if ue = lookupNatEntry(t, f, ueKey); ue.Closed != natClosedUE {
		t.Errorf("after UE FIN: closed=%#x, want %#x (UE side only)", ue.Closed, natClosedUE)
	}

	// The close handshake's final ACK must not clear the closed bits.
	sendUplink(0x10)

	if ue = lookupNatEntry(t, f, ueKey); ue.Closed != natClosedUE {
		t.Errorf("after post-FIN ACK: closed=%#x, want %#x", ue.Closed, natClosedUE)
	}

	// Server FIN: both directions closed.
	capFD = f.captureN3(t)
	f.injectDownlink(t, ethFrame(0x0800, ipv4Packet(serverIP, natPublicIP, 6, tcpSegmentWithFlags(serverIP, natPublicIP, srvDP, ueSP, 0x11))))

	if captureMatching(capFD, time.Second, func(fr []byte) bool { return gtpInner(fr) != nil }) == nil {
		t.Fatal("downlink FIN did not egress on N3")
	}

	if ue = lookupNatEntry(t, f, ueKey); ue.Closed != natClosedUE|natClosedRemote {
		t.Errorf("after server FIN: closed=%#x, want %#x (both sides)", ue.Closed, natClosedUE|natClosedRemote)
	}
}

// TestNATInboundResetDoesNotFullyClose verifies that an unsolicited inbound
// RST closes only the remote direction: without sequence validation it must
// not drop a subscriber's connection into the short closed timeout.
func TestNATInboundResetDoesNotFullyClose(t *testing.T) {
	requireProgTestRun(t)

	const (
		ulTEID = 0x4E41540C
		dlTEID = 0x4E41540D
		qfi    = 7
		ueSP   = 1234
		srvDP  = 80
	)

	f := setupT2(t, true)
	putForwardingUplinkPDRUE(t, f.obj, ulTEID, 0, netip.AddrFrom4(ueIP), netip.Addr{})
	putDownlinkPDR(t, f.obj, ueIP, dlTEID, testUPFN3IP, testGNBIP, qfi)

	uplink := ipv4Packet(ueIP, serverIP, 6, tcpSegmentWithFlags(ueIP, serverIP, ueSP, srvDP, 0x02))
	f.injectUplink(t, uplinkGPDU(ulTEID, uplink))
	time.Sleep(100 * time.Millisecond)

	capFD := f.captureN3(t)
	f.injectDownlink(t, ethFrame(0x0800, ipv4Packet(serverIP, natPublicIP, 6, tcpSegmentWithFlags(serverIP, natPublicIP, srvDP, ueSP, 0x04))))

	if captureMatching(capFD, time.Second, func(fr []byte) bool { return gtpInner(fr) != nil }) == nil {
		t.Fatal("downlink RST did not egress on N3")
	}

	ueKey := natFiveTuple(ueIP, serverIP, ueSP, srvDP, 6)
	if ue := lookupNatEntry(t, f, ueKey); ue.Closed != natClosedRemote {
		t.Errorf("after inbound RST: closed=%#x, want %#x (remote side only)", ue.Closed, natClosedRemote)
	}
}

// TestNATRemapOnForeignReservation verifies that when a flow's NAT-side tuple
// is held by another flow (after LRU eviction and reuse), the next uplink
// packet abandons the stale mapping and allocates a fresh port, leaving the
// other flow's reservation intact.
func TestNATRemapOnForeignReservation(t *testing.T) {
	requireProgTestRun(t)

	const (
		ulTEID = 0x4E41540B
		ueSP   = 4321
		srvDP  = 80
	)

	f := setupT2(t, true)
	putForwardingUplinkPDRUE(t, f.obj, ulTEID, 0, netip.AddrFrom4(ueIP), netip.Addr{})

	uplink := ipv4Packet(ueIP, serverIP, 6, tcpSegmentChecksummed(ueIP, serverIP, ueSP, srvDP, nil))
	f.injectUplink(t, uplinkGPDU(ulTEID, uplink))
	time.Sleep(100 * time.Millisecond)

	ueKey := natFiveTuple(ueIP, serverIP, ueSP, srvDP, 6)
	natKey := natFiveTuple(natPublicIP, serverIP, ueSP, srvDP, 6)
	foreignKey := natFiveTuple([4]byte{10, 45, 0, 2}, serverIP, ueSP, srvDP, 6)

	foreign := N3N6EntrypointNatEntry{Src: foreignKey}
	if err := f.obj.NatCt.Put(&natKey, &foreign); err != nil {
		t.Fatalf("seed foreign reservation: %v", err)
	}

	capFD := f.captureN6(t)
	f.injectUplink(t, uplinkGPDU(ulTEID, uplink))

	got := captureMatching(capFD, time.Second, func(fr []byte) bool {
		return isInnerIPv4(fr, 6, serverIP)
	})
	if got == nil {
		t.Fatal("uplink packet did not egress on N6")
	}

	if sp := binary.BigEndian.Uint16(got[ethHdrLen+20 : ethHdrLen+22]); sp == ueSP {
		t.Errorf("egress source port = %d, want a remapped port", sp)
	}

	if cur := lookupNatEntry(t, f, natKey); cur.Src != foreignKey {
		t.Errorf("foreign reservation src = %+v, want %+v (must stay intact)", cur.Src, foreignKey)
	}

	if ue := lookupNatEntry(t, f, ueKey); ue.Src == natKey {
		t.Error("UE-side entry still references the foreign-held tuple")
	}
}

// TestNATRepairEvictedNATSideEntry verifies pair repair: with the NAT-side
// entry removed (as LRU eviction would), a downlink reply is dropped, and
// the next uplink packet restores the pair so the reply translates again.
func TestNATRepairEvictedNATSideEntry(t *testing.T) {
	requireProgTestRun(t)

	const (
		ulTEID = 0x4E415409
		dlTEID = 0x4E41540A
		qfi    = 7
		ueSP   = 1234
		srvDP  = 80
	)

	f := setupT2(t, true)
	putForwardingUplinkPDRUE(t, f.obj, ulTEID, 0, netip.AddrFrom4(ueIP), netip.Addr{})
	putDownlinkPDR(t, f.obj, ueIP, dlTEID, testUPFN3IP, testGNBIP, qfi)

	natKey := natFiveTuple(natPublicIP, serverIP, ueSP, srvDP, 6)

	uplink := ipv4Packet(ueIP, serverIP, 6, tcpSegmentChecksummed(ueIP, serverIP, ueSP, srvDP, nil))
	reply := ethFrame(0x0800, ipv4Packet(serverIP, natPublicIP, 6, tcpSegmentChecksummed(serverIP, natPublicIP, srvDP, ueSP, nil)))

	f.injectUplink(t, uplinkGPDU(ulTEID, uplink))
	time.Sleep(100 * time.Millisecond)

	if err := f.obj.NatCt.Delete(&natKey); err != nil {
		t.Fatalf("delete NAT-side entry: %v", err)
	}

	capFD := f.captureN3(t)
	f.injectDownlink(t, reply)

	if got := captureMatching(capFD, 500*time.Millisecond, func(fr []byte) bool { return gtpInner(fr) != nil }); got != nil {
		t.Fatalf("reply with the NAT-side entry removed egressed on N3: %x", got)
	}

	f.injectUplink(t, uplinkGPDU(ulTEID, uplink))
	time.Sleep(100 * time.Millisecond)

	if err := f.obj.NatCt.Lookup(&natKey, new(N3N6EntrypointNatEntry)); err != nil {
		t.Fatalf("NAT-side entry not restored by the uplink packet: %v", err)
	}

	capFD = f.captureN3(t)
	f.injectDownlink(t, reply)

	if captureMatching(capFD, time.Second, func(fr []byte) bool { return gtpInner(fr) != nil }) == nil {
		t.Fatal("reply after pair repair did not egress on N3")
	}
}

// downlinkReply builds the L4 segment of a server→UE reply: ports are the
// mirror of the uplink segment so the conntrack reverse lookup matches.
func downlinkReply(proto natProto, payload []byte) []byte {
	switch proto.num {
	case 6:
		return tcpSegmentChecksummed(serverIP, natPublicIP, proto.dport, proto.sport, payload)
	case 17:
		return udpDatagramChecksummed(serverIP, natPublicIP, proto.dport, proto.sport, payload)
	default:
		return icmpEchoReply(natICMPID, 1, payload)
	}
}

// assertSourceNATd checks an uplink source-NAT egress packet: the source is
// rewritten to the egress address, the destination and the whole L4 segment
// (beyond the checksum) are preserved, and both checksums are valid.
func assertSourceNATd(t *testing.T, frame []byte, proto natProto, origL4 []byte) {
	t.Helper()

	ip := frame[ethHdrLen : ethHdrLen+20]
	l4 := frame[ethHdrLen+20:]

	if !bytes.Equal(ip[12:16], natPublicIP[:]) {
		t.Errorf("inner src = %v, want %v (source-NAT'd)", ip[12:16], natPublicIP)
	}

	if !bytes.Equal(ip[16:20], serverIP[:]) {
		t.Errorf("inner dst = %v, want %v (source-NAT must not touch the destination)", ip[16:20], serverIP)
	}

	if !validIPv4Checksum(ip) {
		t.Error("inner IPv4 header checksum invalid after NAT")
	}

	if !proto.valid(natPublicIP, serverIP, l4) {
		t.Errorf("inner %s checksum invalid after NAT", proto.name)
	}

	if !l4PreservedExceptChecksum(proto.num, l4, origL4) {
		t.Errorf("source-NAT altered the L4 segment beyond its checksum:\n got %x\nwant %x (except checksum)", l4, origL4)
	}
}

// assertDestinationNATd checks a downlink destination-NAT egress packet (inside
// the re-encapsulated GTP frame): the destination is rewritten to the UE, the
// source (server) is preserved, the L4 port semantics hold (server source port
// preserved; destination port restored to the UE's original source port), and
// the checksums are valid.
func assertDestinationNATd(t *testing.T, frame []byte, proto natProto) {
	t.Helper()

	inner := gtpInner(frame)
	if inner == nil {
		t.Fatal("captured frame is not a GTP-U G-PDU")
	}

	ip := inner[:20]
	l4 := inner[20:]

	if !bytes.Equal(ip[16:20], ueIP[:]) {
		t.Errorf("inner dst = %v, want %v (destination-NAT'd)", ip[16:20], ueIP)
	}

	if !bytes.Equal(ip[12:16], serverIP[:]) {
		t.Errorf("inner src = %v, want %v (destination-NAT must not touch the source)", ip[12:16], serverIP)
	}

	if !validIPv4Checksum(ip) {
		t.Error("inner IPv4 header checksum invalid after NAT")
	}

	if !proto.valid(serverIP, ueIP, l4) {
		t.Errorf("inner %s checksum invalid after NAT", proto.name)
	}

	switch proto.num {
	case 6, 17:
		if sp := binary.BigEndian.Uint16(l4[0:2]); sp != proto.dport {
			t.Errorf("inner L4 source port = %d, want %d (server port, preserved)", sp, proto.dport)
		}

		if dp := binary.BigEndian.Uint16(l4[2:4]); dp != proto.sport {
			t.Errorf("inner L4 dest port = %d, want %d (restored to UE's original source port)", dp, proto.sport)
		}
	case 1:
		if id := binary.BigEndian.Uint16(l4[4:6]); id != natICMPID {
			t.Errorf("inner ICMP echo id = %#x, want %#x (preserved)", id, natICMPID)
		}
	}
}

// isInnerIPv4 reports whether frame is an Ethernet/IPv4 packet of the given
// protocol addressed to dst.
func isInnerIPv4(frame []byte, proto uint8, dst [4]byte) bool {
	if len(frame) < ethHdrLen+20 || frame[12] != 0x08 || frame[13] != 0x00 {
		return false
	}

	ip := frame[ethHdrLen : ethHdrLen+20]

	return ip[9] == proto && bytes.Equal(ip[16:20], dst[:])
}

// gtpInner returns the inner IPv4 packet of a captured GTP-U/IPv4 G-PDU frame,
// or nil if frame is not one.
func gtpInner(frame []byte) []byte {
	const headersLen = ethHdrLen + gtpV4EncapLen
	if len(frame) < headersLen+20 || frame[12] != 0x08 || frame[13] != 0x00 {
		return nil
	}

	if frame[ethHdrLen+9] != 17 || binary.BigEndian.Uint16(frame[ethHdrLen+22:ethHdrLen+24]) != GTPUDPPort {
		return nil
	}

	return frame[headersLen:]
}

func bytesOf(n int) []byte {
	p := make([]byte, n)
	for i := range p {
		p[i] = byte(i)
	}

	return p
}

func sizeLabel(n int) string { return strconv.Itoa(n) + "B" }
