// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package ebpf

import (
	"encoding/binary"
	"testing"
)

const (
	ethHdrLen = 14

	GTPUDPPort = 2152

	gtpV4EncapLen = 44

	gtpV6EncapLen = 64
)

var (
	testGNBIP   = [4]byte{10, 0, 0, 1}
	testUPFN3IP = [4]byte{10, 0, 0, 2}

	testGNBv6   = [16]byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0xaa, 0, 0, 0, 0, 0, 0, 0, 0x01}
	testUPFN3v6 = [16]byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0xaa, 0, 0, 0, 0, 0, 0, 0, 0x02}
)

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

func ipv4L4Checksum(src, dst [4]byte, proto uint8, l4 []byte) uint16 {
	pseudo := make([]byte, 12, 12+len(l4))
	copy(pseudo[0:4], src[:])
	copy(pseudo[4:8], dst[:])
	pseudo[9] = proto
	binary.BigEndian.PutUint16(pseudo[10:12], uint16(len(l4)))

	return onesComplement16(append(pseudo, l4...))
}

func validIPv4L4Checksum(src, dst [4]byte, proto uint8, l4 []byte) bool {
	return ipv4L4Checksum(src, dst, proto, l4) == 0
}

func tcpSegmentChecksummed(src, dst [4]byte, srcPort, dstPort uint16, payload []byte) []byte {
	seg := make([]byte, 20+len(payload))

	binary.BigEndian.PutUint16(seg[0:2], srcPort)
	binary.BigEndian.PutUint16(seg[2:4], dstPort)
	seg[12] = 0x50
	seg[13] = 0x10
	copy(seg[20:], payload)
	binary.BigEndian.PutUint16(seg[16:18], ipv4L4Checksum(src, dst, 6, seg))

	return seg
}

func udpDatagramChecksummed(src, dst [4]byte, srcPort, dstPort uint16, payload []byte) []byte {
	d := make([]byte, 8+len(payload))

	binary.BigEndian.PutUint16(d[0:2], srcPort)
	binary.BigEndian.PutUint16(d[2:4], dstPort)
	binary.BigEndian.PutUint16(d[4:6], uint16(8+len(payload)))
	copy(d[8:], payload)

	csum := ipv4L4Checksum(src, dst, 17, d)
	if csum == 0 {
		csum = 0xffff
	}

	binary.BigEndian.PutUint16(d[6:8], csum)

	return d
}

func icmpEchoRequest(id, seq uint16, payload []byte) []byte {
	m := make([]byte, 8+len(payload))

	m[0] = 8
	binary.BigEndian.PutUint16(m[4:6], id)
	binary.BigEndian.PutUint16(m[6:8], seq)
	copy(m[8:], payload)
	binary.BigEndian.PutUint16(m[2:4], onesComplement16(m))

	return m
}

func icmpEchoReply(id, seq uint16, payload []byte) []byte {
	m := icmpEchoRequest(id, seq, payload)
	m[0] = 0
	binary.BigEndian.PutUint16(m[2:4], 0)
	binary.BigEndian.PutUint16(m[2:4], onesComplement16(m))

	return m
}

func validICMPChecksum(msg []byte) bool { return onesComplement16(msg) == 0 }

func ethFrame(etherType uint16, l3 []byte) []byte {
	frame := make([]byte, ethHdrLen+len(l3))

	copy(frame[0:6], []byte{0x02, 0x00, 0x00, 0x00, 0x00, 0x02})
	copy(frame[6:12], []byte{0x02, 0x00, 0x00, 0x00, 0x00, 0x01})
	binary.BigEndian.PutUint16(frame[12:14], etherType)
	copy(frame[14:], l3)

	return frame
}

func vlanFrame(vlanID, innerEtherType uint16, l3 []byte) []byte {
	frame := make([]byte, ethHdrLen+4+len(l3))

	copy(frame[0:6], []byte{0x02, 0x00, 0x00, 0x00, 0x00, 0x02})
	copy(frame[6:12], []byte{0x02, 0x00, 0x00, 0x00, 0x00, 0x01})
	binary.BigEndian.PutUint16(frame[12:14], 0x8100)
	binary.BigEndian.PutUint16(frame[14:16], vlanID&0x0fff)
	binary.BigEndian.PutUint16(frame[16:18], innerEtherType)
	copy(frame[18:], l3)

	return frame
}

func validICMPv6Checksum(src, dst [16]byte, icmp6 []byte) bool {
	pseudo := make([]byte, 40+len(icmp6))

	copy(pseudo[0:16], src[:])
	copy(pseudo[16:32], dst[:])
	binary.BigEndian.PutUint32(pseudo[32:36], uint32(len(icmp6)))
	pseudo[39] = 58
	copy(pseudo[40:], icmp6)

	return onesComplement16(pseudo) == 0
}

func icmpv6Checksummed(src, dst [16]byte, icmp6 []byte) []byte {
	out := make([]byte, len(icmp6))
	copy(out, icmp6)
	out[2], out[3] = 0, 0

	pseudo := make([]byte, 40+len(out))

	copy(pseudo[0:16], src[:])
	copy(pseudo[16:32], dst[:])
	binary.BigEndian.PutUint32(pseudo[32:36], uint32(len(out)))
	pseudo[39] = 58
	copy(pseudo[40:], out)

	binary.BigEndian.PutUint16(out[2:4], onesComplement16(pseudo))

	return out
}

func ipv4Packet(src, dst [4]byte, proto uint8, payload []byte) []byte {
	const hdrLen = 20

	pkt := make([]byte, hdrLen+len(payload))

	pkt[0] = 0x45
	binary.BigEndian.PutUint16(pkt[2:4], uint16(hdrLen+len(payload)))
	pkt[8] = 64
	pkt[9] = proto
	copy(pkt[12:16], src[:])
	copy(pkt[16:20], dst[:])
	binary.BigEndian.PutUint16(pkt[10:12], ipv4HeaderChecksum(pkt[:hdrLen]))
	copy(pkt[hdrLen:], payload)

	return pkt
}

func withDF(pkt []byte) []byte {
	pkt[6] |= 0x40
	binary.BigEndian.PutUint16(pkt[10:12], 0)
	binary.BigEndian.PutUint16(pkt[10:12], ipv4HeaderChecksum(pkt[:20]))

	return pkt
}

func udpDatagram(srcPort, dstPort uint16, payload []byte) []byte {
	const hdrLen = 8

	d := make([]byte, hdrLen+len(payload))

	binary.BigEndian.PutUint16(d[0:2], srcPort)
	binary.BigEndian.PutUint16(d[2:4], dstPort)
	binary.BigEndian.PutUint16(d[4:6], uint16(hdrLen+len(payload)))
	copy(d[hdrLen:], payload)

	return d
}

func innerIPv4UDP(dst [4]byte, dport uint16) []byte { //nolint:unparam // general-purpose builder; dport varies in later phases
	return ipv4Packet([4]byte{10, 0, 0, 9}, dst, 17, udpDatagram(0, dport, nil))
}

func innerIPv4UDPSized(dst [4]byte, total int) []byte {
	const ipUDPHdrLen = 20 + 8

	return ipv4Packet([4]byte{10, 0, 0, 9}, dst, 17, udpDatagram(0, 53, make([]byte, total-ipUDPHdrLen)))
}

func tcpSegment(srcPort, dstPort uint16) []byte {
	seg := make([]byte, 20)

	binary.BigEndian.PutUint16(seg[0:2], srcPort)
	binary.BigEndian.PutUint16(seg[2:4], dstPort)
	seg[12] = 0x50

	return seg
}

func innerIPv4TCP(dst [4]byte, dport uint16) []byte {
	return ipv4Packet([4]byte{10, 0, 0, 9}, dst, 6, tcpSegment(0, dport))
}

var testUEv6 = [16]byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x01}

func ipv6Packet(src, dst [16]byte, nextHdr uint8, payload []byte) []byte {
	const hdrLen = 40

	pkt := make([]byte, hdrLen+len(payload))

	pkt[0] = 0x60
	binary.BigEndian.PutUint16(pkt[4:6], uint16(len(payload)))
	pkt[6] = nextHdr
	pkt[7] = 64
	copy(pkt[8:24], src[:])
	copy(pkt[24:40], dst[:])
	copy(pkt[hdrLen:], payload)

	return pkt
}

func innerIPv6UDP(dst [16]byte, dport uint16) []byte { //nolint:unparam // general-purpose builder; dport varies across callers
	src := [16]byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x09}

	return ipv6Packet(src, dst, 17, udpDatagram(0, dport, nil))
}

const ipprotoHopOpts = 0

func hopByHopHeader(nextHdr uint8) []byte {
	return []byte{
		nextHdr,
		0,
		1, 4,
		0, 0, 0, 0,
	}
}

func innerIPv6UDPHopByHop(dst [16]byte, dport uint16) []byte {
	return ipv6Packet(testUEv6Src, dst, ipprotoHopOpts, append(hopByHopHeader(17), udpDatagram(0, dport, nil)...))
}

var testUEv6Src = [16]byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x09}

const (
	ipprotoFragment = 44
	ipprotoAH       = 51
	ipprotoESP      = 50
	ipprotoDstOpts  = 60
)

func ipv6FragmentHeader(nextHdr uint8, offsetUnits uint16, more bool) []byte {
	h := make([]byte, 8)

	h[0] = nextHdr

	off := offsetUnits << 3
	if more {
		off |= 1
	}

	binary.BigEndian.PutUint16(h[2:4], off)
	binary.BigEndian.PutUint32(h[4:8], 0x0badf00d)

	return h
}

func authHeader(nextHdr uint8) []byte {
	h := make([]byte, 24)

	h[0] = nextHdr
	h[1] = 4

	return h
}

func destOptsHeader(nextHdr uint8) []byte {
	return []byte{nextHdr, 0, 1, 4, 0, 0, 0, 0}
}

func innerIPv4Fragment(dst [4]byte, offsetUnits uint16, more bool, port uint16) []byte {
	payload := make([]byte, 16)
	binary.BigEndian.PutUint16(payload[2:4], port)

	pkt := ipv4Packet([4]byte{10, 0, 0, 9}, dst, 17, payload)

	frag := offsetUnits
	if more {
		frag |= 0x2000
	}

	binary.BigEndian.PutUint16(pkt[6:8], frag)
	binary.BigEndian.PutUint16(pkt[10:12], 0)
	binary.BigEndian.PutUint16(pkt[10:12], ipv4HeaderChecksum(pkt[:20]))

	return pkt
}

func innerIPv4FragmentID(src, dst [4]byte, id, offsetUnits uint16, more bool, sport, dport uint16) []byte {
	var payload []byte

	if offsetUnits == 0 {
		payload = udpDatagram(sport, dport, []byte{0xde, 0xad, 0xbe, 0xef})
	} else {
		payload = make([]byte, 16)
	}

	pkt := ipv4Packet(src, dst, 17, payload)

	frag := offsetUnits
	if more {
		frag |= 0x2000
	}

	binary.BigEndian.PutUint16(pkt[4:6], id)
	binary.BigEndian.PutUint16(pkt[6:8], frag)
	binary.BigEndian.PutUint16(pkt[10:12], 0)
	binary.BigEndian.PutUint16(pkt[10:12], ipv4HeaderChecksum(pkt[:20]))

	return pkt
}

func innerIPv6NonFirstFragment(dst [16]byte, decoyPort uint16) []byte {
	payload := make([]byte, 16)
	binary.BigEndian.PutUint16(payload[2:4], decoyPort)

	frag := ipv6FragmentHeader(17, 1, false)

	return ipv6Packet(testUEv6Src, dst, ipprotoFragment, append(frag, payload...))
}

func innerIPv6FragmentChainingToExtHeader(dst [16]byte, decoyProto uint8) []byte {
	payload := []byte{decoyProto, 0, 1, 4, 0, 0, 0, 0}

	frag := ipv6FragmentHeader(ipprotoDstOpts, 1, false)

	return ipv6Packet(testUEv6Src, dst, ipprotoFragment, append(frag, payload...))
}

func innerIPv6ChainTooLong(dst [16]byte, dport uint16) []byte {
	const headers = 8

	chain := udpDatagram(0, dport, nil)
	next := uint8(17)

	for i := 0; i < headers; i++ {
		chain = append(destOptsHeader(next), chain...)
		next = ipprotoDstOpts
	}

	return ipv6Packet(testUEv6Src, dst, ipprotoDstOpts, chain)
}

func innerIPv6ICMPv6RS(ueSrc [16]byte) []byte {
	allRouters := [16]byte{0xff, 0x02, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x02}
	rs := []byte{133, 0, 0, 0, 0, 0, 0, 0}

	return ipv6Packet(ueSrc, allRouters, 58, rs)
}

func gtpControlFrame(msgType uint8) []byte {
	gtp := make([]byte, 8)
	gtp[0] = 0x30
	gtp[1] = msgType

	return ethFrame(0x0800, ipv4Packet(testGNBIP, testUPFN3IP, 17, udpDatagram(3000, GTPUDPPort, gtp)))
}

func gtpControlFrameV6(msgType uint8) []byte {
	gtp := make([]byte, 8)
	gtp[0] = 0x30
	gtp[1] = msgType

	return gtpV6Outer(gtp)
}

func gtpControlFrameSeq(msgType uint8, seq uint16) []byte {
	gtp := make([]byte, 12)
	gtp[0] = 0x32
	gtp[1] = msgType
	binary.BigEndian.PutUint16(gtp[2:4], 4)
	binary.BigEndian.PutUint16(gtp[8:10], seq)

	return ethFrame(0x0800, ipv4Packet(testGNBIP, testUPFN3IP, 17, udpDatagram(3000, GTPUDPPort, gtp)))
}

func gtpV4Outer(gtpPayload []byte) []byte {
	return ethFrame(0x0800, ipv4Packet(testGNBIP, testUPFN3IP, 17, udpDatagram(GTPUDPPort, GTPUDPPort, gtpPayload)))
}

func gtpHeader(teid uint32, inner []byte) []byte {
	const gtpHdrLen = 16

	gtp := make([]byte, gtpHdrLen)
	gtp[0] = 0x34
	gtp[1] = 0xFF
	binary.BigEndian.PutUint16(gtp[2:4], uint16(gtpHdrLen-8+len(inner)))
	binary.BigEndian.PutUint32(gtp[4:8], teid)
	gtp[11] = 0x85
	gtp[12] = 0x01
	gtp[13] = 0x10
	gtp[14] = 0x00
	gtp[15] = 0x00

	return append(gtp, inner...)
}

func uplinkGPDU(teid uint32, inner []byte) []byte {
	return gtpV4Outer(gtpHeader(teid, inner))
}

func gtpHeaderTwoExtHeaders(teid uint32, inner []byte) []byte {
	const gtpHdrLen = 20

	gtp := make([]byte, gtpHdrLen)
	gtp[0] = 0x34
	gtp[1] = 0xFF
	binary.BigEndian.PutUint16(gtp[2:4], uint16(gtpHdrLen-8+len(inner)))
	binary.BigEndian.PutUint32(gtp[4:8], teid)
	gtp[11] = 0xC0
	gtp[12] = 0x01
	gtp[15] = 0x85
	gtp[16] = 0x01
	gtp[17] = 0x10
	gtp[19] = 0x00

	return gtpV4Outer(append(gtp, inner...))
}

func gtpV6Outer(gtpPayload []byte) []byte {
	return ethFrame(0x86DD, ipv6Packet(testGNBv6, testUPFN3v6, 17, udpDatagram(GTPUDPPort, GTPUDPPort, gtpPayload)))
}

func uplinkGPDUv6(teid uint32, inner []byte) []byte {
	return gtpV6Outer(gtpHeader(teid, inner))
}

func validUDPv6Checksum(src, dst [16]byte, udpSegment []byte) bool {
	pseudo := make([]byte, 40+len(udpSegment))

	copy(pseudo[0:16], src[:])
	copy(pseudo[16:32], dst[:])
	binary.BigEndian.PutUint32(pseudo[32:36], uint32(len(udpSegment)))
	pseudo[39] = 17
	copy(pseudo[40:], udpSegment)

	return onesComplement16(pseudo) == 0
}

func malformedUplinkGTPv4(teid uint32) []byte {
	gtp := make([]byte, 8)
	gtp[0] = 0x34
	gtp[1] = 0xFF
	binary.BigEndian.PutUint32(gtp[4:8], teid)

	return gtpV4Outer(gtp)
}

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

func udpDatagramChecksummedV6(src, dst [16]byte, srcPort, dstPort uint16, payload []byte) []byte {
	d := udpDatagram(srcPort, dstPort, payload)

	pseudo := make([]byte, 40+len(d))
	copy(pseudo[0:16], src[:])
	copy(pseudo[16:32], dst[:])
	binary.BigEndian.PutUint32(pseudo[32:36], uint32(len(d)))
	pseudo[39] = 17
	copy(pseudo[40:], d)

	csum := onesComplement16(pseudo)
	if csum == 0 {
		csum = 0xffff
	}

	binary.BigEndian.PutUint16(d[6:8], csum)

	return d
}
