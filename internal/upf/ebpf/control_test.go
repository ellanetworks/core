// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package ebpf

import (
	"bytes"
	"encoding/binary"
	"testing"
	"time"

	"github.com/cilium/ebpf/ringbuf"
)

func assertEchoResponse(t *testing.T, gtp []byte, wantSeq uint16) {
	t.Helper()

	const (
		gtpEchoResponse = 2
		gtpIERecovery   = 0x0e
	)

	if len(gtp) != 14 {
		t.Fatalf("GTP-U message length = %d, want 14 (12-byte header + Recovery IE)", len(gtp))
	}

	if gtp[0] != 0x32 {
		t.Errorf("GTP flags = %#x, want 0x32 (version 1, PT 1, S 1)", gtp[0])
	}

	if gtp[1] != gtpEchoResponse {
		t.Errorf("GTP message type = %d, want %d (echo response)", gtp[1], gtpEchoResponse)
	}

	if got := binary.BigEndian.Uint16(gtp[2:4]); got != 6 {
		t.Errorf("GTP message length = %d, want 6", got)
	}

	if got := binary.BigEndian.Uint32(gtp[4:8]); got != 0 {
		t.Errorf("GTP TEID = %d, want 0", got)
	}

	if got := binary.BigEndian.Uint16(gtp[8:10]); got != wantSeq {
		t.Errorf("Echo Response sequence number = %#x, want %#x (the request's)", got, wantSeq)
	}

	if gtp[12] != gtpIERecovery {
		t.Errorf("IE type = %#x, want %#x (Recovery, mandatory per TS 29.281 Table 7.2.2-1)", gtp[12], gtpIERecovery)
	}

	if gtp[13] != 0 {
		t.Errorf("Recovery restart counter = %d, want 0 (TS 29.281 §7.2.2)", gtp[13])
	}
}

func TestGTPControlMessages(t *testing.T) {
	requireProgTestRun(t)

	obj := loadN3N6Program(t)

	const (
		gtpEchoRequest     = 1
		gtpEchoResponse    = 2
		gtpErrorIndication = 26
		gtpEndMarker       = 254
	)

	t.Run("echo request gets response", func(t *testing.T) {
		in := gtpControlFrame(gtpEchoRequest)

		action, out := runXDPOut(t, obj.UpfEntryFunc, in)

		if action != ActionTx {
			t.Fatalf("got XDP action %d, want ActionTx (%d)", action, ActionTx)
		}

		if want := len(in) + 6; len(out) != want {
			t.Fatalf("frame length = %d, want %d", len(out), want)
		}

		assertEchoResponse(t, out[ethHdrLen+20+8:], 0)

		if !bytes.Equal(out[26:30], testUPFN3IP[:]) || !bytes.Equal(out[30:34], testGNBIP[:]) {
			t.Errorf("outer IPs not swapped: src=%v dst=%v", out[26:30], out[30:34])
		}

		if src, dst := binary.BigEndian.Uint16(out[34:36]), binary.BigEndian.Uint16(out[36:38]); src != GTPUDPPort || dst != 3000 {
			t.Errorf("UDP ports not swapped: src=%d dst=%d, want %d/%d", src, dst, GTPUDPPort, 3000)
		}

		if !bytes.Equal(out[0:6], []byte{0x02, 0, 0, 0, 0, 0x01}) || !bytes.Equal(out[6:12], []byte{0x02, 0, 0, 0, 0, 0x02}) {
			t.Errorf("MAC addresses not swapped: dst=%x src=%x", out[0:6], out[6:12])
		}
	})

	passThrough := []struct {
		name    string
		msgType uint8
	}{
		{"echo response passes", gtpEchoResponse},
		{"error indication passes", gtpErrorIndication},
		{"end marker passes", gtpEndMarker},
	}

	for _, tc := range passThrough {
		t.Run(tc.name, func(t *testing.T) {
			if action := runXDP(t, obj.UpfEntryFunc, gtpControlFrame(tc.msgType)); action != ActionPass {
				t.Fatalf("got XDP action %d, want ActionPass (%d)", action, ActionPass)
			}
		})
	}
}

// TS 29.281 §5.1
func TestGTPEchoRequestWithSequenceNumber(t *testing.T) {
	requireProgTestRun(t)

	obj := loadN3N6Program(t)

	const gtpEchoRequest = 1

	in := gtpControlFrameSeq(gtpEchoRequest, 0x1234)

	action, out := runXDPOut(t, obj.UpfEntryFunc, in)

	if action != ActionTx {
		t.Fatalf("Echo Request with a sequence number (S=1, no extension header) got XDP action %d, want ActionTx (%d) — the UPF must answer it (TS 29.281 §7.2.1)", action, ActionTx)
	}

	assertEchoResponse(t, out[ethHdrLen+20+8:], 0x1234)
}

// RFC 8200
func TestGTPEchoResponseIPv6Checksum(t *testing.T) {
	requireProgTestRun(t)

	obj := loadN3N6Program(t)

	const gtpEchoRequest = 1

	action, out := runXDPOut(t, obj.UpfEntryFunc, gtpControlFrameV6(gtpEchoRequest))

	if action != ActionTx {
		t.Fatalf("IPv6 echo request got XDP action %d, want ActionTx (%d)", action, ActionTx)
	}

	assertEchoResponse(t, out[ethHdrLen+40+8:], 0)

	if got := binary.BigEndian.Uint16(out[ethHdrLen+4 : ethHdrLen+6]); got != 8+14 {
		t.Errorf("IPv6 payload length = %d, want %d (UDP header + Echo Response)", got, 8+14)
	}

	if !validUDPv6Checksum(testUPFN3v6, testGNBv6, out[ethHdrLen+40:]) {
		t.Error("Echo Response UDP-over-IPv6 checksum does not validate (mandatory over IPv6)")
	}
}

func TestRouterSolicitationIntercept(t *testing.T) {
	requireProgTestRun(t)

	const teid = 0x52530001

	obj := loadN3N6Program(t)
	putForwardingUplinkPDR(t, obj, teid, 0)

	rd, err := ringbuf.NewReader(obj.RsEventMap)
	if err != nil {
		t.Fatalf("open rs_event ring buffer: %v", err)
	}

	defer func() { _ = rd.Close() }()

	action := runXDP(t, obj.UpfEntryFunc, uplinkGPDU(teid, innerIPv6ICMPv6RS(testUEv6)))
	if action != ActionDrop {
		t.Fatalf("Router Solicitation not intercepted: got XDP action %d, want ActionDrop (%d)", action, ActionDrop)
	}

	rd.SetDeadline(time.Now().Add(time.Second))

	rec, err := rd.Read()
	if err != nil {
		t.Fatalf("no RS event emitted to userspace (RA responder would never fire): %v", err)
	}

	var ev RSEvent
	if err := binary.Read(bytes.NewReader(rec.RawSample), binary.NativeEndian, &ev); err != nil {
		t.Fatalf("decode RS event: %v", err)
	}

	if ev.TEID != teid {
		t.Errorf("RS event TEID = %#x, want %#x", ev.TEID, uint32(teid))
	}

	if ev.UEIPv6 != testUEv6 {
		t.Errorf("RS event UE IPv6 = %x, want %x", ev.UEIPv6, testUEv6)
	}
}

func readRSEvent(t *testing.T, rd *ringbuf.Reader) *RSEvent {
	t.Helper()

	rd.SetDeadline(time.Now().Add(time.Second))

	rec, err := rd.Read()
	if err != nil {
		return nil
	}

	if want := (binary.Size(RSEvent{}) + 3) &^ 3; len(rec.RawSample) != want {
		t.Fatalf("RS event is %d bytes, want %d (struct rs_event layout differs from RSEvent)", len(rec.RawSample), want)
	}

	var ev RSEvent
	if err := binary.Read(bytes.NewReader(rec.RawSample), binary.NativeEndian, &ev); err != nil {
		t.Fatalf("decode RS event: %v", err)
	}

	return &ev
}

func assertRSIntercepted(t *testing.T, obj *BpfObjects, teid uint32, inner []byte, ueSrc [16]byte) *RSEvent {
	t.Helper()

	rd, err := ringbuf.NewReader(obj.RsEventMap)
	if err != nil {
		t.Fatalf("open rs_event ring buffer: %v", err)
	}

	defer func() { _ = rd.Close() }()

	if action := runXDP(t, obj.UpfEntryFunc, uplinkGPDU(teid, inner)); action != ActionDrop {
		t.Fatalf("Router Solicitation not intercepted: got XDP action %d, want ActionDrop (%d)", action, ActionDrop)
	}

	ev := readRSEvent(t, rd)
	if ev == nil {
		t.Fatal("no RS event emitted to userspace (RA responder would never fire)")
	}

	if ev.TEID != teid {
		t.Errorf("RS event TEID = %#x, want %#x", ev.TEID, teid)
	}

	if ev.UEIPv6 != ueSrc {
		t.Errorf("RS event UE IPv6 = %x, want %x", ev.UEIPv6, ueSrc)
	}

	if !bytes.Equal(ev.DstIPv6[:], inner[24:40]) {
		t.Errorf("RS event destination = %x, want %x", ev.DstIPv6, inner[24:40])
	}

	return ev
}

func assertRSMessage(t *testing.T, ev *RSEvent, msg []byte) {
	t.Helper()

	if int(ev.MsgLen) != len(msg) {
		t.Fatalf("RS event message length = %d, want %d", ev.MsgLen, len(msg))
	}

	if !bytes.Equal(ev.Msg[:ev.MsgLen], msg) {
		t.Errorf("RS event message = % x, want % x", ev.Msg[:ev.MsgLen], msg)
	}
}

func TestRouterSolicitationInterceptWithN6VLAN(t *testing.T) {
	requireProgTestRun(t)

	const teid = 0x52530002

	obj := loadProgramConfig(t, false, false, 0, 1, 0, 100)
	putForwardingUplinkPDR(t, obj, teid, 0)

	assertRSIntercepted(t, obj, teid, innerIPv6ICMPv6RS(testUEv6), testUEv6)
}

func TestRouterSolicitationInterceptBehindExtensionHeader(t *testing.T) {
	requireProgTestRun(t)

	const (
		teid        = 0x52530003
		protoICMPv6 = 58
	)

	obj := loadN3N6Program(t)
	putForwardingUplinkPDR(t, obj, teid, 0)

	msg := []byte{133, 0, 0, 0, 0, 0, 0, 0}
	inner := routerSolicitation(testUEv6, hopByHopHeader(protoICMPv6), msg)

	assertRSMessage(t, assertRSIntercepted(t, obj, teid, inner, testUEv6), msg)
}

func TestICMPv6EchoNotMisreadAsRouterSolicitation(t *testing.T) {
	requireProgTestRun(t)

	const (
		teid            = 0x52530004
		protoICMPv6     = 58
		icmpv6EchoReq   = 128
		rsTypeOffsetDst = 12
	)

	obj := loadProgramConfig(t, false, false, 0, 1, 0, 100)
	putForwardingUplinkPDR(t, obj, teid, 0)

	rd, err := ringbuf.NewReader(obj.RsEventMap)
	if err != nil {
		t.Fatalf("open rs_event ring buffer: %v", err)
	}

	defer func() { _ = rd.Close() }()

	dst := [16]byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x99}
	dst[rsTypeOffsetDst] = 133

	echo := []byte{icmpv6EchoReq, 0, 0, 0, 0, 0, 0, 0}
	inner := ipv6Packet(testUEv6, dst, protoICMPv6, echo)

	runXDP(t, obj.UpfEntryFunc, uplinkGPDU(teid, inner))

	if ev := readRSEvent(t, rd); ev != nil {
		t.Errorf("ICMPv6 Echo Request reported as a Router Solicitation with UE IPv6 %x", ev.UEIPv6)
	}
}

func optionedTrigger(src, dst [4]byte, options, udp []byte) []byte {
	ip := ipv4PacketWithOptions(src, dst, 17, options, udp)
	hdrLen := 20 + len(options)

	ip[8] = 3
	binary.BigEndian.PutUint16(ip[10:12], 0)
	binary.BigEndian.PutUint16(ip[10:12], ipv4HeaderChecksum(ip[:hdrLen]))

	return ethFrame(0x0800, ip)
}

func assertReplyIPv4Header(t *testing.T, out []byte, wantUDPPayload int, wantSrcPort, wantDstPort uint16) {
	t.Helper()

	const ipHdrLen, udpHdrLen = 20, 8

	if want := ethHdrLen + ipHdrLen + udpHdrLen + wantUDPPayload; len(out) != want {
		t.Fatalf("reply frame length = %d, want %d (no IPv4 options, tail trimmed)", len(out), want)
	}

	ip := out[ethHdrLen : ethHdrLen+ipHdrLen]

	if ip[0] != 0x45 {
		t.Errorf("IPv4 version/IHL = %#x, want 0x45 (no options, RFC 791 §3.1)", ip[0])
	}

	if got, want := binary.BigEndian.Uint16(ip[2:4]), uint16(ipHdrLen+udpHdrLen+wantUDPPayload); got != want {
		t.Errorf("IPv4 Total Length = %d, want %d", got, want)
	}

	if got := binary.BigEndian.Uint16(ip[6:8]); got != 0x4000 {
		t.Errorf("IPv4 flags/fragment offset = %#x, want 0x4000 (DF, unfragmented)", got)
	}

	if ip[8] != 64 {
		t.Errorf("IPv4 TTL = %d, want 64 (fresh, not the trigger's)", ip[8])
	}

	if ip[9] != 17 {
		t.Errorf("IPv4 protocol = %d, want 17 (UDP)", ip[9])
	}

	if !validIPv4Checksum(ip) {
		t.Error("IPv4 header checksum does not validate over the 20-octet header")
	}

	if !bytes.Equal(ip[12:16], testUPFN3IP[:]) || !bytes.Equal(ip[16:20], testGNBIP[:]) {
		t.Errorf("outer IPs = %v -> %v, want %v -> %v", ip[12:16], ip[16:20], testUPFN3IP, testGNBIP)
	}

	udp := out[ethHdrLen+ipHdrLen : ethHdrLen+ipHdrLen+udpHdrLen]

	if src, dst := binary.BigEndian.Uint16(udp[0:2]), binary.BigEndian.Uint16(udp[2:4]); src != wantSrcPort || dst != wantDstPort {
		t.Errorf("UDP ports = %d -> %d, want %d -> %d", src, dst, wantSrcPort, wantDstPort)
	}

	if got, want := binary.BigEndian.Uint16(udp[4:6]), uint16(udpHdrLen+wantUDPPayload); got != want {
		t.Errorf("UDP length = %d, want %d", got, want)
	}

	if got := binary.BigEndian.Uint16(udp[6:8]); got != 0 {
		t.Errorf("UDP checksum = %#x, want 0 (not computed over IPv4)", got)
	}
}

// TS 29.281 §7.2.1, §4.4.2.2, §4.4.3.2
func TestGTPEchoRequestWithOuterIPv4Options(t *testing.T) {
	requireProgTestRun(t)

	obj := loadN3N6Program(t)

	const gtpEchoRequest = 1

	for _, tc := range []struct {
		name    string
		options []byte
	}{
		{"record route", []byte{7, 8, 4, 0, 10, 0, 0, 1}},
		{"maximum header", bytes.Repeat([]byte{1}, 40)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gtp := make([]byte, 12)
			gtp[0] = 0x32
			gtp[1] = gtpEchoRequest
			binary.BigEndian.PutUint16(gtp[2:4], 4)
			binary.BigEndian.PutUint16(gtp[8:10], 0x4321)

			in := optionedTrigger(testGNBIP, testUPFN3IP, tc.options, udpDatagram(3000, GTPUDPPort, gtp))

			action, out := runXDPOut(t, obj.UpfEntryFunc, in)

			if action != ActionTx {
				t.Fatalf("Echo Request with outer IPv4 options got XDP action %d, want ActionTx (%d) (TS 29.281 §7.2.1)", action, ActionTx)
			}

			assertReplyIPv4Header(t, out, 14, GTPUDPPort, 3000)
			assertEchoResponse(t, out[ethHdrLen+20+8:], 0x4321)
		})
	}
}
