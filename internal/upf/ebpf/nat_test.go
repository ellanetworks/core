// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package ebpf

import (
	"bytes"
	"encoding/binary"
	"strconv"
	"testing"
	"time"
)

// natProto describes one transport protocol for the NAT tests: how to build an
// inner L4 segment between two IPv4 endpoints and how to validate its checksum
// after rewrite.
type natProto struct {
	name  string
	num   uint8
	build func(src, dst [4]byte, payload []byte) []byte
	valid func(src, dst [4]byte, l4 []byte) bool
}

var natProtos = []natProto{
	{
		name:  "tcp",
		num:   6,
		build: func(src, dst [4]byte, p []byte) []byte { return tcpSegmentChecksummed(src, dst, 1234, 80, p) },
		valid: func(src, dst [4]byte, l4 []byte) bool { return validIPv4L4Checksum(src, dst, 6, l4) },
	},
	{
		name:  "udp",
		num:   17,
		build: func(src, dst [4]byte, p []byte) []byte { return udpDatagramChecksummed(src, dst, 1234, 53, p) },
		valid: func(src, dst [4]byte, l4 []byte) bool { return validIPv4L4Checksum(src, dst, 17, l4) },
	},
	{
		name:  "icmp",
		num:   1,
		build: func(_, _ [4]byte, p []byte) []byte { return icmpEchoRequest(0xbeef, 1, p) },
		valid: func(_, _ [4]byte, l4 []byte) bool { return validICMPChecksum(l4) },
	},
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
	putForwardingUplinkPDR(t, f.obj, teid, 0)

	for _, proto := range natProtos {
		for _, size := range payloadSizes {
			t.Run(proto.name+"/"+sizeLabel(size), func(t *testing.T) {
				capFD := f.captureN6(t)

				inner := ipv4Packet(ueIP, serverIP, proto.num, proto.build(ueIP, serverIP, bytesOf(size)))
				f.injectUplink(t, uplinkGPDU(teid, inner))

				got := captureMatching(capFD, time.Second, func(fr []byte) bool {
					return isInnerIPv4(fr, proto.num, serverIP)
				})
				if got == nil {
					t.Fatal("did not capture a NAT'd packet on the N6 side")
				}

				assertSourceNATd(t, got, proto)
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
	putForwardingUplinkPDR(t, f.obj, ulTEID, 0)
	putDownlinkPDR(t, f.obj, ueIP, dlTEID, testUPFN3IP, testGNBIP, qfi)

	for _, proto := range natProtos {
		for _, size := range []int{40, 1000} {
			t.Run(proto.name+"/"+sizeLabel(size), func(t *testing.T) {
				// Uplink first: establish the conntrack mapping.
				uplinkInner := ipv4Packet(ueIP, serverIP, proto.num, proto.build(ueIP, serverIP, bytesOf(40)))
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

	action, out := runXDPOut(t, obj.UpfN3N6EntrypointFunc, uplinkGPDU(teid, inner))
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
	putForwardingUplinkPDR(t, f.obj, teid1, 0)
	putForwardingUplinkPDR(t, f.obj, teid2, 0)

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

		tcp := got[ethHdrLen+20:]

		if !bytes.Equal(got[ethHdrLen+12:ethHdrLen+16], natPublicIP[:]) {
			t.Errorf("inner src = %v, want %v (source-NAT'd)", got[ethHdrLen+12:ethHdrLen+16], natPublicIP)
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
	putForwardingUplinkPDR(t, f.obj, ulTEID, 0)
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

	// The embedded original packet's source must be rewritten to the UE.
	embedded := icmp[8:]
	if len(embedded) < 28 || !bytes.Equal(embedded[12:16], ueIP[:]) {
		t.Fatalf("embedded packet source not rewritten to the UE: %x", embedded)
	}

	if !validIPv4Checksum(embedded[:20]) {
		t.Error("embedded IPv4 header checksum invalid after NAT")
	}

	if !validIPv4L4Checksum(ueIP, serverIP, 17, embedded[20:28]) {
		t.Error("embedded UDP checksum invalid after NAT")
	}
}

// downlinkReply builds the L4 segment of a server→UE reply: ports are the
// mirror of the uplink segment so the conntrack reverse lookup matches.
func downlinkReply(proto natProto, payload []byte) []byte {
	switch proto.num {
	case 6:
		return tcpSegmentChecksummed(serverIP, natPublicIP, 80, 1234, payload)
	case 17:
		return udpDatagramChecksummed(serverIP, natPublicIP, 53, 1234, payload)
	default:
		return icmpEchoReply(0xbeef, 1, payload)
	}
}

func assertSourceNATd(t *testing.T, frame []byte, proto natProto) {
	t.Helper()

	ip := frame[ethHdrLen : ethHdrLen+20]
	l4 := frame[ethHdrLen+20:]

	if !bytes.Equal(ip[12:16], natPublicIP[:]) {
		t.Errorf("inner src = %v, want %v (source-NAT'd)", ip[12:16], natPublicIP)
	}

	if !validIPv4Checksum(ip) {
		t.Error("inner IPv4 header checksum invalid after NAT")
	}

	if !proto.valid(natPublicIP, serverIP, l4) {
		t.Errorf("inner %s checksum invalid after NAT", proto.name)
	}
}

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

	if !validIPv4Checksum(ip) {
		t.Error("inner IPv4 header checksum invalid after NAT")
	}

	if !proto.valid(serverIP, ueIP, l4) {
		t.Errorf("inner %s checksum invalid after NAT", proto.name)
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
