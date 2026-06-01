// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package ebpf

import (
	"encoding/binary"
	"testing"
)

// Packet-building and -parsing helpers shared by the data-plane tests. They
// operate on raw byte slices so a test can assert on the exact bytes the XDP
// program produced.

const (
	// ethHdrLen is the size of an Ethernet header, the offset of the inner
	// packet in a decapsulated frame.
	ethHdrLen = 14

	// GTPUDPPort is the GTP-U UDP port.
	GTPUDPPort = 2152

	// gtpV4EncapLen is the GTP-U/UDP/IPv4 + PDU-session-extension overhead added
	// by the downlink encapsulation path: IPv4(20) + UDP(8) + GTP(8) + ext(8).
	gtpV4EncapLen = 44

	// gtpV6EncapLen is the same overhead with an IPv6 outer header:
	// IPv6(40) + UDP(8) + GTP(8) + ext(8).
	gtpV6EncapLen = 64
)

var (
	testGNBIP   = [4]byte{10, 0, 0, 1}
	testUPFN3IP = [4]byte{10, 0, 0, 2}

	testGNBv6   = [16]byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0xaa, 0, 0, 0, 0, 0, 0, 0, 0x01}
	testUPFN3v6 = [16]byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0xaa, 0, 0, 0, 0, 0, 0, 0, 0x02}
)

// onesComplement16 is the 16-bit one's-complement sum used by IP/UDP/TCP
// checksums. Over a header that already contains its checksum it returns 0 when
// the checksum is valid.
func onesComplement16(b []byte) uint16 {
	var sum uint32

	for i := 0; i+1 < len(b); i += 2 {
		sum += uint32(binary.BigEndian.Uint16(b[i:]))
	}

	if len(b)%2 == 1 {
		sum += uint32(b[len(b)-1]) << 8
	}

	for sum>>16 != 0 {
		sum = (sum & 0xffff) + (sum >> 16)
	}

	return ^uint16(sum)
}

func ipv4HeaderChecksum(header []byte) uint16 { return onesComplement16(header) }

func validIPv4Checksum(header []byte) bool { return onesComplement16(header) == 0 }

// ethFrame prepends an Ethernet header (fixed locally-administered MACs) with
// the given ethertype to l3.
func ethFrame(etherType uint16, l3 []byte) []byte {
	frame := make([]byte, ethHdrLen+len(l3))

	copy(frame[0:6], []byte{0x02, 0x00, 0x00, 0x00, 0x00, 0x02})
	copy(frame[6:12], []byte{0x02, 0x00, 0x00, 0x00, 0x00, 0x01})
	binary.BigEndian.PutUint16(frame[12:14], etherType)
	copy(frame[14:], l3)

	return frame
}

// ipv4Packet builds an IPv4 packet (with a valid header checksum) carrying
// payload.
func ipv4Packet(src, dst [4]byte, proto uint8, payload []byte) []byte { //nolint:unparam // general-purpose builder; proto varies in later phases
	const hdrLen = 20

	pkt := make([]byte, hdrLen+len(payload))

	pkt[0] = 0x45 // version 4, IHL 5
	binary.BigEndian.PutUint16(pkt[2:4], uint16(hdrLen+len(payload)))
	pkt[8] = 64 // TTL
	pkt[9] = proto
	copy(pkt[12:16], src[:])
	copy(pkt[16:20], dst[:])
	binary.BigEndian.PutUint16(pkt[10:12], ipv4HeaderChecksum(pkt[:hdrLen]))
	copy(pkt[hdrLen:], payload)

	return pkt
}

// udpDatagram builds a UDP datagram with a zero checksum (valid for IPv4).
func udpDatagram(srcPort, dstPort uint16, payload []byte) []byte {
	const hdrLen = 8

	d := make([]byte, hdrLen+len(payload))

	binary.BigEndian.PutUint16(d[0:2], srcPort)
	binary.BigEndian.PutUint16(d[2:4], dstPort)
	binary.BigEndian.PutUint16(d[4:6], uint16(hdrLen+len(payload)))
	copy(d[hdrLen:], payload)

	return d
}

// innerIPv4UDP builds a UE inner packet: an IPv4/UDP datagram to dst:dport. On
// uplink, dst is the SDF remote address.
func innerIPv4UDP(dst [4]byte, dport uint16) []byte { //nolint:unparam // general-purpose builder; dport varies in later phases
	return ipv4Packet([4]byte{10, 0, 0, 9}, dst, 17, udpDatagram(0, dport, nil))
}

// testUEv6 is a sample inner UE IPv6 address (2001:db8::1).
var testUEv6 = [16]byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x01}

// ipv6Packet builds an IPv6 packet carrying payload. IPv6 has no header
// checksum.
func ipv6Packet(src, dst [16]byte, nextHdr uint8, payload []byte) []byte {
	const hdrLen = 40

	pkt := make([]byte, hdrLen+len(payload))

	pkt[0] = 0x60 // version 6
	binary.BigEndian.PutUint16(pkt[4:6], uint16(len(payload)))
	pkt[6] = nextHdr
	pkt[7] = 64 // hop limit
	copy(pkt[8:24], src[:])
	copy(pkt[24:40], dst[:])
	copy(pkt[hdrLen:], payload)

	return pkt
}

// innerIPv6UDP builds a UE inner packet: an IPv6/UDP datagram to dst:dport.
func innerIPv6UDP(dst [16]byte, dport uint16) []byte {
	src := [16]byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x09}

	return ipv6Packet(src, dst, 17, udpDatagram(0, dport, nil))
}

// gtpV4Outer wraps a GTP-U payload (the GTP header onward) in the
// Ethernet/IPv4/UDP(2152) outer headers of an N3 uplink frame.
func gtpV4Outer(gtpPayload []byte) []byte {
	return ethFrame(0x0800, ipv4Packet(testGNBIP, testUPFN3IP, 17, udpDatagram(GTPUDPPort, GTPUDPPort, gtpPayload)))
}

// gtpHeader builds a GTP-U G-PDU header: an 8-byte base header with the E flag
// set plus the 8-byte optional header word, followed by inner.
func gtpHeader(teid uint32, inner []byte) []byte {
	const gtpHdrLen = 16

	gtp := make([]byte, gtpHdrLen)
	gtp[0] = 0x34 // version=1, PT=1, E=1
	gtp[1] = 0xFF // GTPU_G_PDU
	binary.BigEndian.PutUint16(gtp[2:4], uint16(gtpHdrLen-8+len(inner)))
	binary.BigEndian.PutUint32(gtp[4:8], teid)

	return append(gtp, inner...)
}

// uplinkGPDU wraps inner in a well-formed GTP-U G-PDU inside an
// Ethernet/IPv4/UDP frame addressed to the GTP-U port.
func uplinkGPDU(teid uint32, inner []byte) []byte {
	return gtpV4Outer(gtpHeader(teid, inner))
}

// gtpV6Outer wraps a GTP-U payload in Ethernet/IPv6/UDP(2152) outer headers (an
// N3 uplink frame with IPv6 transport). The outer UDP checksum is left zero;
// the parse path does not validate it on receive.
func gtpV6Outer(gtpPayload []byte) []byte {
	return ethFrame(0x86DD, ipv6Packet(testGNBv6, testUPFN3v6, 17, udpDatagram(GTPUDPPort, GTPUDPPort, gtpPayload)))
}

// uplinkGPDUv6 wraps inner in a well-formed GTP-U G-PDU inside an
// Ethernet/IPv6/UDP frame (GTP-U over IPv6 transport).
func uplinkGPDUv6(teid uint32, inner []byte) []byte {
	return gtpV6Outer(gtpHeader(teid, inner))
}

// validUDPv6Checksum verifies a UDP-over-IPv6 checksum (RFC 8200 pseudo-header:
// src + dst + upper-layer length + next-header 17).
func validUDPv6Checksum(src, dst [16]byte, udpSegment []byte) bool {
	pseudo := make([]byte, 40+len(udpSegment))

	copy(pseudo[0:16], src[:])
	copy(pseudo[16:32], dst[:])
	binary.BigEndian.PutUint32(pseudo[32:36], uint32(len(udpSegment)))
	pseudo[39] = 17 // next header = UDP
	copy(pseudo[40:], udpSegment)

	return onesComplement16(pseudo) == 0
}

// malformedUplinkGTPv4 builds a GTP-U frame that sets the E flag but omits the
// optional header word the flag implies.
func malformedUplinkGTPv4(teid uint32) []byte {
	gtp := make([]byte, 8)
	gtp[0] = 0x34 // version=1, PT=1, E=1
	gtp[1] = 0xFF // GTPU_G_PDU
	binary.BigEndian.PutUint32(gtp[4:8], teid)

	return gtpV4Outer(gtp)
}

// gtpV4Frame is the parsed view of an Ethernet/IPv4/UDP/GTP-U G-PDU frame.
type gtpV4Frame struct {
	etherType       uint16
	outerSrc        [4]byte
	outerDst        [4]byte
	outerProto      uint8
	outerChecksumOK bool
	udpDstPort      uint16
	gtpFlags        uint8
	gtpMsgType      uint8
	teid            uint32
	qfi             uint8
	inner           []byte
}

// parseGTPv4Frame decodes a GTP-U-over-IPv4 G-PDU frame produced by the
// encapsulation path. Layout: eth(14) | IPv4(20) | UDP(8) | GTP(8) |
// gtp_hdr_ext(4) | pdu_session_container(4) | inner.
func parseGTPv4Frame(t *testing.T, frame []byte) gtpV4Frame {
	t.Helper()

	const headersLen = ethHdrLen + gtpV4EncapLen
	if len(frame) < headersLen {
		t.Fatalf("frame too short for a GTP-U/IPv4 G-PDU: %d bytes", len(frame))
	}

	ip := frame[ethHdrLen : ethHdrLen+20]
	udp := frame[ethHdrLen+20 : ethHdrLen+28]
	gtp := frame[ethHdrLen+28 : ethHdrLen+36]
	psc := frame[ethHdrLen+40 : ethHdrLen+44]

	f := gtpV4Frame{
		etherType:       binary.BigEndian.Uint16(frame[12:14]),
		outerProto:      ip[9],
		outerChecksumOK: validIPv4Checksum(ip),
		udpDstPort:      binary.BigEndian.Uint16(udp[2:4]),
		gtpFlags:        gtp[0],
		gtpMsgType:      gtp[1],
		teid:            binary.BigEndian.Uint32(gtp[4:8]),
		qfi:             psc[2] & 0x3f,
		inner:           frame[headersLen:],
	}
	copy(f.outerSrc[:], ip[12:16])
	copy(f.outerDst[:], ip[16:20])

	return f
}

// gtpV6Frame is the parsed view of an Ethernet/IPv6/UDP/GTP-U G-PDU frame.
type gtpV6Frame struct {
	outerSrc      [16]byte
	outerDst      [16]byte
	outerNextHdr  uint8
	udpDstPort    uint16
	udpChecksumOK bool
	gtpFlags      uint8
	gtpMsgType    uint8
	teid          uint32
	qfi           uint8
	inner         []byte
}

// parseGTPv6Frame decodes a GTP-U-over-IPv6 G-PDU frame. Layout: eth(14) |
// IPv6(40) | UDP(8) | GTP(8) | gtp_hdr_ext(4) | pdu_session_container(4) |
// inner.
func parseGTPv6Frame(t *testing.T, frame []byte) gtpV6Frame {
	t.Helper()

	const headersLen = ethHdrLen + gtpV6EncapLen
	if len(frame) < headersLen {
		t.Fatalf("frame too short for a GTP-U/IPv6 G-PDU: %d bytes", len(frame))
	}

	ip6 := frame[ethHdrLen : ethHdrLen+40]
	udp := frame[ethHdrLen+40 : ethHdrLen+48]
	gtp := frame[ethHdrLen+48 : ethHdrLen+56]
	psc := frame[ethHdrLen+60 : ethHdrLen+64]

	f := gtpV6Frame{
		outerNextHdr: ip6[6],
		udpDstPort:   binary.BigEndian.Uint16(udp[2:4]),
		gtpFlags:     gtp[0],
		gtpMsgType:   gtp[1],
		teid:         binary.BigEndian.Uint32(gtp[4:8]),
		qfi:          psc[2] & 0x3f,
		inner:        frame[headersLen:],
	}
	copy(f.outerSrc[:], ip6[8:24])
	copy(f.outerDst[:], ip6[24:40])
	f.udpChecksumOK = validUDPv6Checksum(f.outerSrc, f.outerDst, frame[ethHdrLen+40:])

	return f
}
