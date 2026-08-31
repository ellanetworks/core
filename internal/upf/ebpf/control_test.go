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

	var ev RSEvent
	if err := binary.Read(bytes.NewReader(rec.RawSample), binary.NativeEndian, &ev); err != nil {
		t.Fatalf("decode RS event: %v", err)
	}

	return &ev
}

func assertRSIntercepted(t *testing.T, obj *BpfObjects, teid uint32, inner []byte, ueSrc [16]byte) {
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
		teid          = 0x52530003
		protoHopByHop = 0
		protoICMPv6   = 58
	)

	obj := loadN3N6Program(t)
	putForwardingUplinkPDR(t, obj, teid, 0)

	allRouters := [16]byte{0xff, 0x02, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x02}
	rs := []byte{133, 0, 0, 0, 0, 0, 0, 0}
	inner := ipv6Packet(testUEv6, allRouters, protoHopByHop, append(hopByHopHeader(protoICMPv6), rs...))

	assertRSIntercepted(t, obj, teid, inner, testUEv6)
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
