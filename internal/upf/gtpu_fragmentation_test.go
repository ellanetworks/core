// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package upf

import (
	"bytes"
	"encoding/binary"
	"errors"
	"net/netip"
	"testing"

	"github.com/ellanetworks/core/internal/upf/engine"
	dto "github.com/prometheus/client_model/go"
)

var (
	testMACXDP = [6]byte{0x02, 0, 0, 0, 0, 0x01}
	testMACGo  = [6]byte{0x02, 0, 0, 0, 0, 0x02}
)

func testGPDU(n int) []byte {
	pdu := make([]byte, n)
	pdu[0], pdu[1] = 0x30, gtpuMsgGPDU
	binary.BigEndian.PutUint16(pdu[2:4], uint16(n-8))
	binary.BigEndian.PutUint32(pdu[4:8], 0xabcd)

	for i := 8; i < n; i++ {
		pdu[i] = byte(i)
	}

	return pdu
}

func TestReinjectionFrameIPv4(t *testing.T) {
	d := engine.GTPUDatagram{
		Local:       netip.MustParseAddrPort("10.3.0.2:2152"),
		Peer:        netip.MustParseAddrPort("10.3.0.3:40000"),
		Reassembled: true,
		Payload:     testGPDU(2000),
	}

	frame, ok := reinjectionFrame(testMACXDP, testMACGo, d)
	if !ok {
		t.Fatal("no frame built")
	}

	if !bytes.Equal(frame[0:6], testMACXDP[:]) || !bytes.Equal(frame[6:12], testMACGo[:]) {
		t.Errorf("MACs = % x, want dst %x src %x", frame[0:12], testMACXDP, testMACGo)
	}

	if et := binary.BigEndian.Uint16(frame[12:14]); et != etherTypeIPv4 {
		t.Errorf("ethertype = %#x, want IPv4", et)
	}

	ip := frame[ethHeaderLen : ethHeaderLen+ipv4HeaderLen]
	if onesComplementSum(0, ip) != 0xffff {
		t.Error("IPv4 header checksum invalid")
	}

	if total := binary.BigEndian.Uint16(ip[2:4]); int(total) != len(frame)-ethHeaderLen {
		t.Errorf("IPv4 total length = %d, want %d", total, len(frame)-ethHeaderLen)
	}

	if !bytes.Equal(ip[12:16], []byte{10, 3, 0, 3}) || !bytes.Equal(ip[16:20], []byte{10, 3, 0, 2}) {
		t.Errorf("addresses = %v -> %v, want the radio -> the UPF", ip[12:16], ip[16:20])
	}

	udp := frame[ethHeaderLen+ipv4HeaderLen:]
	if sp, dp := binary.BigEndian.Uint16(udp[0:2]), binary.BigEndian.Uint16(udp[2:4]); sp != 40000 || dp != gtpuPort {
		t.Errorf("ports = %d -> %d, want 40000 -> %d", sp, dp, gtpuPort)
	}

	if !bytes.Equal(udp[udpHeaderLen:], d.Payload) {
		t.Error("UDP payload differs from the datagram")
	}
}

func TestReinjectionFrameIPv6ChecksumsUDP(t *testing.T) {
	d := engine.GTPUDatagram{
		Local:       netip.MustParseAddrPort("[2001:db8:3::2]:2152"),
		Peer:        netip.MustParseAddrPort("[2001:db8:3::3]:2152"),
		Reassembled: true,
		Payload:     testGPDU(1801),
	}

	frame, ok := reinjectionFrame(testMACXDP, testMACGo, d)
	if !ok {
		t.Fatal("no frame built")
	}

	ip := frame[ethHeaderLen : ethHeaderLen+ipv6HeaderLen]
	udp := frame[ethHeaderLen+ipv6HeaderLen:]

	if plen := binary.BigEndian.Uint16(ip[4:6]); int(plen) != len(udp) {
		t.Errorf("payload length = %d, want %d", plen, len(udp))
	}

	sum := onesComplementSum(0, ip[8:40])
	sum = onesComplementSum(sum, []byte{0, 0, byte(len(udp) >> 8), byte(len(udp)), 0, 0, 0, ipProtoUDP})
	sum = onesComplementSum(sum, udp)

	if sum != 0xffff {
		t.Errorf("UDP checksum does not verify: sum %#x", sum)
	}
}

func TestReinjectionFrameRefusesOversizedDatagram(t *testing.T) {
	d := engine.GTPUDatagram{
		Local:   netip.MustParseAddrPort("10.3.0.2:2152"),
		Peer:    netip.MustParseAddrPort("10.3.0.3:2152"),
		Payload: testGPDU(maxReinjectedIP),
	}

	if _, ok := reinjectionFrame(testMACXDP, testMACGo, d); ok {
		t.Error("a datagram larger than the veth MTU was framed")
	}
}

func encapsulatedIPv4(tos byte, local, peer [4]byte, dport uint16, payload []byte) []byte {
	pkt := make([]byte, ipv4HeaderLen+udpHeaderLen+len(payload))
	pkt[0] = 0x45
	pkt[1] = tos
	binary.BigEndian.PutUint16(pkt[2:4], uint16(len(pkt)))
	pkt[9] = ipProtoUDP
	copy(pkt[12:16], local[:])
	copy(pkt[16:20], peer[:])
	binary.BigEndian.PutUint16(pkt[20:22], gtpuPort)
	binary.BigEndian.PutUint16(pkt[22:24], dport)
	binary.BigEndian.PutUint16(pkt[24:26], uint16(udpHeaderLen+len(payload)))
	copy(pkt[28:], payload)

	return pkt
}

func TestParseEncapsulatedPacketIPv4(t *testing.T) {
	payload := testGPDU(1500)
	pkt := encapsulatedIPv4(0xb8, [4]byte{10, 3, 0, 2}, [4]byte{10, 3, 0, 3}, gtpuPort, payload)

	local, peer, tclass, got, ok := parseEncapsulatedPacket(pkt)
	if !ok {
		t.Fatal("rejected a well-formed packet")
	}

	if local != netip.MustParseAddr("10.3.0.2") || peer != netip.MustParseAddr("10.3.0.3") {
		t.Errorf("addresses = %s -> %s", local, peer)
	}

	if tclass != 0xb8 {
		t.Errorf("TOS = %#x, want 0xb8", tclass)
	}

	if !bytes.Equal(got, payload) {
		t.Error("payload differs")
	}
}

func TestParseEncapsulatedPacketIPv6(t *testing.T) {
	payload := testGPDU(1300)
	pkt := make([]byte, ipv6HeaderLen+udpHeaderLen+len(payload))
	pkt[0], pkt[1] = 0x6b, 0x80
	binary.BigEndian.PutUint16(pkt[4:6], uint16(udpHeaderLen+len(payload)))
	pkt[6] = ipProtoUDP
	local, peer := netip.MustParseAddr("2001:db8:3::2").As16(), netip.MustParseAddr("2001:db8:3::3").As16()
	copy(pkt[8:24], local[:])
	copy(pkt[24:40], peer[:])
	binary.BigEndian.PutUint16(pkt[42:44], gtpuPort)
	copy(pkt[48:], payload)

	l, p, tclass, got, ok := parseEncapsulatedPacket(pkt)
	if !ok {
		t.Fatal("rejected a well-formed packet")
	}

	if l.As16() != local || p.As16() != peer {
		t.Errorf("addresses = %s -> %s", l, p)
	}

	if tclass != 0xb8 {
		t.Errorf("traffic class = %#x, want 0xb8", tclass)
	}

	if !bytes.Equal(got, payload) {
		t.Error("payload differs")
	}
}

func TestParseEncapsulatedPacketRejectsMalformed(t *testing.T) {
	good := encapsulatedIPv4(0, [4]byte{10, 3, 0, 2}, [4]byte{10, 3, 0, 3}, gtpuPort, testGPDU(64))

	truncated := bytes.Clone(good)
	binary.BigEndian.PutUint16(truncated[2:4], uint16(len(good)+1))

	notGTPU := encapsulatedIPv4(0, [4]byte{10, 3, 0, 2}, [4]byte{10, 3, 0, 3}, 53, testGPDU(64))

	for name, pkt := range map[string][]byte{
		"short":       good[:10],
		"truncated":   truncated,
		"not GTP-U":   notGTPU,
		"bad version": append([]byte{0x15}, good[1:]...),
	} {
		if _, _, _, _, ok := parseEncapsulatedPacket(pkt); ok {
			t.Errorf("%s: accepted", name)
		}
	}
}

func newTestFragmentationResponder(t *testing.T, allowed ...int) (*FragmentationResponder, *[][]byte) {
	t.Helper()

	f := NewFragmentationResponder(nil, nil, nil, allowed...)
	f.injectFD = 3
	f.dstMAC = testMACXDP
	f.srcMAC = testMACGo

	var sent [][]byte

	f.send = func(_ int, frame []byte) error {
		sent = append(sent, bytes.Clone(frame))
		return nil
	}

	return f, &sent
}

func reassemblyCount(result string) float64 {
	var m dto.Metric
	if err := gtpuFragmentation.WithLabelValues(directionUplink, result).Write(&m); err != nil {
		return -1
	}

	return m.GetCounter().GetValue()
}

func TestReceiveReinjectsOnlyReassembledGPDUsFromN3(t *testing.T) {
	f, sent := newTestFragmentationResponder(t, 7)

	base := engine.GTPUDatagram{
		Local:       netip.MustParseAddrPort("10.3.0.2:2152"),
		Peer:        netip.MustParseAddrPort("10.3.0.3:2152"),
		IfIndex:     7,
		Reassembled: true,
		Payload:     testGPDU(1600),
	}

	notReassembled := base
	notReassembled.Reassembled = false

	wrongIface := base
	wrongIface.IfIndex = 9

	echo := base
	echo.Payload = bytes.Clone(base.Payload)
	echo.Payload[1] = 1

	beforeReassembled, beforeIgnored := reassemblyCount(reasmReassembled), reassemblyCount(reasmIgnored)

	for _, d := range []engine.GTPUDatagram{notReassembled, wrongIface, echo, base} {
		f.receive(d)
	}

	if len(*sent) != 1 {
		t.Fatalf("re-injected %d frames, want 1", len(*sent))
	}

	if !bytes.Equal((*sent)[0][ethHeaderLen+ipv4HeaderLen+udpHeaderLen:], base.Payload) {
		t.Error("the re-injected frame does not carry the datagram")
	}

	if got := reassemblyCount(reasmReassembled) - beforeReassembled; got != 1 {
		t.Errorf("reassembled = %v, want 1", got)
	}

	if got := reassemblyCount(reasmIgnored) - beforeIgnored; got != 3 {
		t.Errorf("ignored = %v, want 3", got)
	}
}

func TestReceiveCountsInjectFailure(t *testing.T) {
	f, _ := newTestFragmentationResponder(t)
	f.send = func(int, []byte) error { return errors.New("boom") }

	before := reassemblyCount(reasmInjectFailed)

	f.receive(engine.GTPUDatagram{
		Local:       netip.MustParseAddrPort("10.3.0.2:2152"),
		Peer:        netip.MustParseAddrPort("10.3.0.3:2152"),
		Reassembled: true,
		Payload:     testGPDU(100),
	})

	if got := reassemblyCount(reasmInjectFailed) - before; got != 1 {
		t.Errorf("inject_failed = %v, want 1", got)
	}
}

type recordedGTPU struct {
	local, peer netip.Addr
	payload     []byte
}

type fakeGTPU struct{ sent []recordedGTPU }

func (g *fakeGTPU) ServeGTPU(engine.GTPUReceiver) error { return nil }

func (g *fakeGTPU) SendGTPU(local netip.Addr, peer netip.Addr, _ uint8, payload []byte) error {
	g.sent = append(g.sent, recordedGTPU{local, peer, bytes.Clone(payload)})
	return nil
}

func TestReceiveAnswersUnknownTEIDWithErrorIndication(t *testing.T) {
	gtpu := &fakeGTPU{}
	f := NewFragmentationResponder(nil, gtpu, func(teid uint32) bool { return teid == 0x1111 })
	f.injectFD = 3

	var injected int

	f.send = func(int, []byte) error {
		injected++
		return nil
	}

	d := engine.GTPUDatagram{
		Local:       netip.MustParseAddrPort("10.3.0.2:2152"),
		Peer:        netip.MustParseAddrPort("10.3.0.3:2152"),
		Reassembled: true,
		Payload:     testGPDU(1600),
	}

	f.receive(d)

	if injected != 0 {
		t.Errorf("re-injected a G-PDU for an unknown TEID")
	}

	if len(gtpu.sent) != 1 {
		t.Fatalf("sent %d GTP-U messages, want 1 Error Indication", len(gtpu.sent))
	}

	got := gtpu.sent[0]
	if got.local != d.Local.Addr() || got.peer != d.Peer.Addr() {
		t.Errorf("Error Indication %s -> %s, want %s -> %s", got.local, got.peer, d.Local.Addr(), d.Peer.Addr())
	}

	want := []byte{
		0x32, gtpuMsgErrorInd, 0, 16, 0, 0, 0, 0, 0, 0, 0, 0,
		gtpuIETEIDData, 0, 0, 0xab, 0xcd,
		gtpuIEPeerAddr, 0, 4, 10, 3, 0, 2,
	}
	if !bytes.Equal(got.payload, want) {
		t.Errorf("Error Indication = % x, want % x (TS 29.281 §7.3.1)", got.payload, want)
	}

	d.Payload = testGPDU(1600)
	binary.BigEndian.PutUint32(d.Payload[4:8], 0x1111)

	f.receive(d)

	if injected != 1 || len(gtpu.sent) != 1 {
		t.Errorf("a known TEID: injected %d, sent %d, want 1 and 1", injected, len(gtpu.sent))
	}
}

func TestErrorIndicationPDUIPv6PeerAddress(t *testing.T) {
	pdu := errorIndicationPDU(7, netip.MustParseAddr("2001:db8::2"))

	if len(pdu) != 36 || binary.BigEndian.Uint16(pdu[2:4]) != 28 || pdu[19] != 16 {
		t.Errorf("IPv6 Error Indication = % x, want 36 bytes, length 28, a 16-byte peer address", pdu)
	}
}
