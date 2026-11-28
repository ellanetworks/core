// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package upf

import (
	"bytes"
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/ellanetworks/core/internal/upf/engine"
)

func isSubslice(outer, inner []byte) bool {
	off := cap(outer) - cap(inner)
	if len(inner) == 0 || off < 0 || off+len(inner) > len(outer) {
		return false
	}

	return &outer[off] == &inner[0]
}

func fuzzGTPUHeader(msgType byte, flags byte, teid uint32, seq uint16, next byte, body []byte) []byte {
	hdr := 8
	if flags&gtpuFlagsOptional != 0 {
		hdr = 12
	}

	pdu := make([]byte, hdr+len(body))
	pdu[0] = 0x30 | flags
	pdu[1] = msgType
	binary.BigEndian.PutUint16(pdu[2:4], uint16(len(pdu)-8))
	binary.BigEndian.PutUint32(pdu[4:8], teid)

	if hdr == 12 {
		binary.BigEndian.PutUint16(pdu[8:10], seq)
		pdu[11] = next
	}

	copy(pdu[hdr:], body)

	return pdu
}

func fuzzExtChain(types []byte, units int, inner []byte) []byte {
	var exts []byte

	for i := range types {
		ext := make([]byte, units*4)
		ext[0] = byte(units)

		if i+1 < len(types) {
			ext[len(ext)-1] = types[i+1]
		}

		exts = append(exts, ext...)
	}

	return fuzzGTPUHeader(gtpuMsgGPDU, gtpuFlagExtension, 0xabcd, 0, types[0], append(exts, inner...))
}

func fuzzIPv4WithOptions(ihl int, options []byte, fragField uint16, proto byte, l4 []byte) []byte {
	ip := make([]byte, ihl*4+len(l4))
	ip[0] = 0x40 | byte(ihl)
	binary.BigEndian.PutUint16(ip[2:4], uint16(len(ip)))
	binary.BigEndian.PutUint16(ip[6:8], fragField)
	ip[8], ip[9] = 64, proto
	copy(ip[12:16], []byte{10, 45, 0, 1})
	copy(ip[16:20], []byte{10, 6, 0, 3})
	copy(ip[20:ihl*4], options)
	copy(ip[ihl*4:], l4)

	return ip
}

func fuzzGTPUSeeds() [][]byte {
	udp := l4Segment(ipProtoUDP, udpChecksumOffset, 64)
	tcp := l4Segment(ipProtoTCP, tcpChecksumOffset, 60)
	v4udp := innerIPv4(ipProtoUDP, udp, 0)
	v4tcp := innerIPv4(ipProtoTCP, tcp, 0)
	v6udp := innerIPv6(ipProtoUDP, udp)
	v6tcp := innerIPv6(ipProtoTCP, tcp)

	recordRoute := []byte{7, 11, 4, 0, 0, 0, 0, 0, 0, 0, 0, 1}

	seeds := [][]byte{
		testGPDU(64),
		gpduWithoutOptional(v4udp),
		gpduWithoutOptional(v6tcp),
		gpduWithPSC(v4tcp),
		gpduWithPSC(v6udp),
		gpduWithPSC(innerIPv4(ipProtoUDP, udp, 0x2000)),
		gpduWithPSC(innerIPv4(ipProtoUDP, udp, 0x0010)),
		gpduWithPSC(innerIPv4(ipProtoUDP, udp, 0x4000)),
		fuzzExtChain([]byte{0x85}, 1, v4udp),
		fuzzExtChain([]byte{0x85}, 2, v6udp),
		fuzzExtChain([]byte{0x40, 0xc0, 0x82, 0x85}, 1, v4tcp),
		fuzzExtChain([]byte{0x20, 0x40, 0x81, 0x83, 0x84, 0x85, 0x86, 0x04, 0x03, 0xc0, 0x82, 0x40, 0x20, 0x81, 0x84, 0x85}, 1, v4udp),
		fuzzExtChain([]byte{0x84}, 255, v6tcp),
		fuzzGTPUHeader(gtpuMsgGPDU, gtpuFlagExtension, 0xabcd, 0, 0x85, []byte{0, 0, 9, 0}),
		fuzzGTPUHeader(gtpuMsgGPDU, gtpuFlagExtension, 0xabcd, 0, 0x85, []byte{2, 0x10, 9, 0}),
		fuzzGTPUHeader(gtpuMsgGPDU, gtpuFlagExtension, 0xabcd, 0, 0x85, nil),
		fuzzGTPUHeader(gtpuMsgGPDU, 0x02, 0xabcd, 7, 0, v4udp),
		fuzzGTPUHeader(gtpuMsgGPDU, 0x01, 0xabcd, 0, 0, v6udp),
		fuzzGTPUHeader(gtpuMsgGPDU, 0x02, 0xabcd, 7, 0, nil)[:10],
		fuzzGTPUHeader(1, 0x02, 0, 1, 0, nil),
		fuzzGTPUHeader(2, 0x02, 0, 1, 0, []byte{14, 0}),
		fuzzGTPUHeader(gtpuMsgErrorInd, 0x02, 0, 1, 0, []byte{16, 0, 0, 0xab, 0xcd, 133, 0, 4, 10, 3, 0, 2}),
		fuzzGTPUHeader(gtpuMsgErrorInd, 0x02, 0, 1, 0, []byte{16, 0, 0, 0xab, 0xcd, 133, 0, 16, 0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2}),
		errorIndicationPDU(0xabcd, netip.MustParseAddr("10.3.0.2")),
		fuzzGTPUHeader(254, 0, 0xabcd, 0, 0, nil),
		fuzzGTPUHeader(254, gtpuFlagExtension, 0xabcd, 0, 0x85, []byte{1, 0, 9, 0}),
		gpduWithPSC(innerIPv4(ipProtoUDP, make([]byte, 8), 0)),
		gpduWithPSC(innerIPv4(ipProtoUDP, make([]byte, 7), 0)),
	}

	for ihl := 5; ihl <= 15; ihl++ {
		opts := make([]byte, (ihl-5)*4)
		for i := range opts {
			opts[i] = 1
		}

		if len(opts) > 0 {
			opts[len(opts)-1] = 0
		}

		if len(opts) >= len(recordRoute) {
			copy(opts, recordRoute)
		}

		seeds = append(seeds, gpduWithPSC(fuzzIPv4WithOptions(ihl, opts, 0, ipProtoUDP, udp)))
		seeds = append(seeds, gpduWithoutOptional(fuzzIPv4WithOptions(ihl, opts, 0x2000, ipProtoTCP, tcp)))
	}

	return seeds
}

func fuzzEncapsulatedIPv6(tclass byte, payload []byte) []byte {
	pkt := make([]byte, ipv6HeaderLen+udpHeaderLen+len(payload))
	pkt[0], pkt[1] = 0x60|tclass>>4, tclass<<4
	binary.BigEndian.PutUint16(pkt[4:6], uint16(udpHeaderLen+len(payload)))
	pkt[6], pkt[7] = ipProtoUDP, 64
	local, peer := netip.MustParseAddr("2001:db8:3::2").As16(), netip.MustParseAddr("2001:db8:3::3").As16()
	copy(pkt[8:24], local[:])
	copy(pkt[24:40], peer[:])
	binary.BigEndian.PutUint16(pkt[40:42], gtpuPort)
	binary.BigEndian.PutUint16(pkt[42:44], gtpuPort)
	binary.BigEndian.PutUint16(pkt[44:46], uint16(udpHeaderLen+len(payload)))
	copy(pkt[48:], payload)

	return pkt
}

func fuzzEncapsulatedIPv4Options(ihl int, payload []byte) []byte {
	opts := make([]byte, (ihl-5)*4)
	for i := range opts {
		opts[i] = 1
	}

	udp := make([]byte, udpHeaderLen+len(payload))
	binary.BigEndian.PutUint16(udp[0:2], gtpuPort)
	binary.BigEndian.PutUint16(udp[2:4], gtpuPort)
	binary.BigEndian.PutUint16(udp[4:6], uint16(len(udp)))
	copy(udp[udpHeaderLen:], payload)

	pkt := fuzzIPv4WithOptions(ihl, opts, 0, ipProtoUDP, udp)
	pkt[1] = 0xb8

	return pkt
}

func FuzzParseEncapsulatedPacket(f *testing.F) {
	local, peer := [4]byte{10, 3, 0, 2}, [4]byte{10, 3, 0, 3}

	for _, gtpu := range fuzzGTPUSeeds() {
		f.Add(gtpu)
		f.Add(encapsulatedIPv4(0xb8, local, peer, gtpuPort, gtpu))
		f.Add(fuzzEncapsulatedIPv6(0xb8, gtpu))
	}

	good := encapsulatedIPv4(0, local, peer, gtpuPort, testGPDU(64))

	truncated := bytes.Clone(good)
	binary.BigEndian.PutUint16(truncated[2:4], uint16(len(good)+1))

	notUDP := bytes.Clone(good)
	notUDP[9] = ipProtoTCP

	for ihl := 5; ihl <= 15; ihl++ {
		f.Add(fuzzEncapsulatedIPv4Options(ihl, testGPDU(64)))
	}

	f.Add(good[:ipv4HeaderLen])
	f.Add(good[:ipv4HeaderLen+udpHeaderLen])
	f.Add(truncated)
	f.Add(notUDP)
	f.Add(encapsulatedIPv4(0, local, peer, 53, testGPDU(64)))
	f.Add(fuzzEncapsulatedIPv6(0, nil))
	f.Add(fuzzEncapsulatedIPv6(0, testGPDU(64))[:ipv6HeaderLen])
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, pkt []byte) {
		original := bytes.Clone(pkt)

		l, p, _, payload, ok := parseEncapsulatedPacket(pkt)
		if ok {
			if !l.IsValid() || !p.IsValid() {
				t.Fatalf("accepted packet with invalid addresses %v -> %v", l, p)
			}

			if len(payload) > 0 && !isSubslice(pkt, payload) {
				t.Fatal("payload is not within the input packet")
			}

			if !bytes.Equal(pkt, original) {
				t.Fatal("parseEncapsulatedPacket modified its input")
			}

			if inner := gtpuInnerPacket(payload); len(inner) > 0 && !isSubslice(payload, inner) {
				t.Fatal("inner packet is not within the GTP-U payload")
			}

			once := bytes.Clone(payload)
			finalizeInnerChecksum(once)

			changed := 0

			for i := range once {
				if once[i] != payload[i] {
					changed++
				}
			}

			if changed > 2 {
				t.Fatalf("finalizeInnerChecksum changed %d bytes", changed)
			}

			twice := bytes.Clone(once)
			finalizeInnerChecksum(twice)

			if !bytes.Equal(once, twice) {
				t.Fatal("finalizeInnerChecksum is not idempotent")
			}
		}

		if inner := gtpuInnerPacket(pkt); len(inner) > 0 && !isSubslice(pkt, inner) {
			t.Fatal("inner packet is not within the input")
		}

		finalizeInnerChecksum(bytes.Clone(pkt))
	})
}

func FuzzFragmentationReceive(f *testing.F) {
	for _, gtpu := range fuzzGTPUSeeds() {
		f.Add(gtpu, false, uint8(1), false, uint16(2152))
		f.Add(gtpu, true, uint8(1), true, uint16(2152))
	}

	f.Add(testGPDU(64), false, uint8(0), true, uint16(2152))
	f.Add(testGPDU(64), true, uint8(2), false, uint16(2152))

	f.Add(testGPDU(maxReinjectedIP), false, uint8(0), true, uint16(2152))
	f.Add(testGPDU(maxReinjectedIP-ipv6HeaderLen-udpHeaderLen), true, uint8(0), true, uint16(2152))
	f.Add(fuzzGTPUHeader(gtpuMsgGPDU, 0, 0, 0, 0, nil), false, uint8(0), false, uint16(0))
	f.Add([]byte{0x30, gtpuMsgGPDU, 0, 0}, false, uint8(2), true, uint16(2152))

	f.Fuzz(func(t *testing.T, payload []byte, ipv6 bool, ifSel uint8, known bool, peerPort uint16) {
		gtpu := &fakeGTPU{}

		var lookedUp []uint32

		knownTEID := func(teid uint32) bool {
			lookedUp = append(lookedUp, teid)
			return known
		}

		fr := NewFragmentationResponder(nil, gtpu, knownTEID, 1)
		fr.injectFD = 3
		fr.dstMAC = testMACXDP
		fr.srcMAC = testMACGo

		var sent [][]byte

		fr.send = func(_ int, frame []byte) error {
			sent = append(sent, bytes.Clone(frame))
			return nil
		}

		localAddr, peerAddr := netip.MustParseAddr("10.3.0.2"), netip.MustParseAddr("10.3.0.3")
		if ipv6 {
			localAddr, peerAddr = netip.MustParseAddr("2001:db8:3::2"), netip.MustParseAddr("2001:db8:3::3")
		}

		d := engine.GTPUDatagram{
			Local:       netip.AddrPortFrom(localAddr, gtpuPort),
			Peer:        netip.AddrPortFrom(peerAddr, peerPort),
			IfIndex:     int(ifSel % 3),
			Reassembled: true,
			Payload:     bytes.Clone(payload),
		}

		fr.receive(d)

		if len(sent)+len(gtpu.sent) > 1 {
			t.Fatalf("one datagram produced %d frames and %d GTP-U sends", len(sent), len(gtpu.sent))
		}

		if len(sent) == 1 {
			if d.IfIndex != 1 || len(payload) < gtpuMinHeader || payload[1] != gtpuMsgGPDU {
				t.Fatal("re-injected a datagram that should have been ignored")
			}

			ipLen := ipv4HeaderLen
			if ipv6 {
				ipLen = ipv6HeaderLen
			}

			frame := sent[0]
			if len(frame) != ethHeaderLen+ipLen+udpHeaderLen+len(payload) {
				t.Fatalf("frame length %d for a %d-byte payload", len(frame), len(payload))
			}

			if !bytes.Equal(frame[ethHeaderLen+ipLen+udpHeaderLen:], payload) {
				t.Fatal("re-injected payload differs from the datagram")
			}

			if !ipv6 && onesComplementSum(0, frame[ethHeaderLen:ethHeaderLen+ipv4HeaderLen]) != 0xffff {
				t.Fatal("re-injected IPv4 header checksum is invalid")
			}

			if ipv6 {
				udp := frame[ethHeaderLen+ipv6HeaderLen:]
				sum := onesComplementSum(0, frame[ethHeaderLen+8:ethHeaderLen+40])
				sum = onesComplementSum(sum, []byte{0, 0, byte(len(udp) >> 8), byte(len(udp)), 0, 0, 0, ipProtoUDP})

				if onesComplementSum(sum, udp) != 0xffff {
					t.Fatal("re-injected IPv6 UDP checksum is invalid")
				}
			}
		}

		if len(gtpu.sent) == 1 {
			if known || len(lookedUp) != 1 || lookedUp[0] == 0 {
				t.Fatal("sent an Error Indication for a TEID that was not unknown")
			}

			ei := gtpu.sent[0].payload
			if len(ei) < 12 || ei[1] != gtpuMsgErrorInd || int(binary.BigEndian.Uint16(ei[2:4])) != len(ei)-8 {
				t.Fatalf("malformed Error Indication % x", ei)
			}

			if ei[12] != gtpuIETEIDData || binary.BigEndian.Uint32(ei[13:17]) != lookedUp[0] {
				t.Fatal("Error Indication does not carry the offending TEID")
			}

			if gtpu.sent[0].peer != peerAddr || gtpu.sent[0].local != localAddr {
				t.Fatal("Error Indication addressed to the wrong peer")
			}
		}

		mixed := engine.GTPUDatagram{
			Local:   netip.AddrPortFrom(localAddr, gtpuPort),
			Peer:    netip.AddrPortFrom(netip.MustParseAddr("10.3.0.3"), peerPort),
			Payload: payload,
		}

		_, ok := reinjectionFrame(testMACXDP, testMACGo, mixed)
		if ipv6 && ok {
			t.Fatal("reinjectionFrame framed a mixed-family datagram")
		}

		if !ipv6 && !ok && ipv4HeaderLen+udpHeaderLen+len(payload) <= maxReinjectedIP {
			t.Fatal("reinjectionFrame refused a datagram that fits")
		}
	})
}
