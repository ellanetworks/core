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

// GTP-U control-message handling and ICMPv6 Router Solicitation interception.
// PMTU / frag-needed generation needs a small-MTU device and is deferred to the
// netns harness.

// assertEchoResponse checks gtp is a conformant GTP-U Echo Response: the S flag
// set (TS 29.281 §5.1), a zero TEID, the request's sequence number echoed, and
// the mandatory Recovery IE with a zero restart counter (§7.2.2, Table 7.2.2-1).
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

	// Length counts everything after the 8-byte mandatory header: the optional
	// word (4) and the Recovery IE (2).
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

// TestGTPControlMessages checks GTP-U control-message dispatch: an echo request
// is answered (XDP_TX, addresses/ports swapped, type set to echo response);
// other control messages are passed to the kernel.
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

		if action != XDP_TX {
			t.Fatalf("got XDP action %d, want XDP_TX (%d)", action, XDP_TX)
		}

		// The request carries an 8-byte header; the response is the 12-byte
		// header plus the 2-octet Recovery IE, so the frame grows by 6.
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
			if action := runXDP(t, obj.UpfEntryFunc, gtpControlFrame(tc.msgType)); action != XDP_PASS {
				t.Fatalf("got XDP action %d, want XDP_PASS (%d)", action, XDP_PASS)
			}
		})
	}
}

// TestGTPEchoRequestWithSequenceNumber checks that a GTP-U Echo Request carrying
// a sequence number (S flag set) but no extension header is answered. This is a
// conformant message (TS 29.281 §5.1, §7.2.1) and the form a real NG-RAN node
// uses for N3 path supervision.
//
// It fails today: parse_gtp assumes a fixed 4-byte extension header is present
// whenever any of E/S/PN is set (it skips sizeof(gtp_hdr_ext) + 4 = 8 bytes), so
// it cannot parse the 12-byte header and drops the message instead of answering.
func TestGTPEchoRequestWithSequenceNumber(t *testing.T) {
	requireProgTestRun(t)

	obj := loadN3N6Program(t)

	const gtpEchoRequest = 1

	in := gtpControlFrameSeq(gtpEchoRequest, 0x1234)

	action, out := runXDPOut(t, obj.UpfEntryFunc, in)

	if action != XDP_TX {
		t.Fatalf("Echo Request with a sequence number (S=1, no extension header) got XDP action %d, want XDP_TX (%d) — the UPF must answer it (TS 29.281 §7.2.1)", action, XDP_TX)
	}

	// The Echo Response repeats the request's sequence number (TS 29.281 §7.2.2).
	assertEchoResponse(t, out[ethHdrLen+20+8:], 0x1234)
}

// TestGTPEchoResponseIPv6Checksum checks that the Echo Response to an IPv6 echo
// request carries a valid UDP checksum. The checksum is mandatory over IPv6
// (RFC 8200); changing the GTP message type must not leave it stale, or the
// receiver drops the reply.
func TestGTPEchoResponseIPv6Checksum(t *testing.T) {
	requireProgTestRun(t)

	obj := loadN3N6Program(t)

	const gtpEchoRequest = 1

	action, out := runXDPOut(t, obj.UpfEntryFunc, gtpControlFrameV6(gtpEchoRequest))

	if action != XDP_TX {
		t.Fatalf("IPv6 echo request got XDP action %d, want XDP_TX (%d)", action, XDP_TX)
	}

	assertEchoResponse(t, out[ethHdrLen+40+8:], 0)

	// The Recovery IE grew the datagram, so the IPv6 payload length must have
	// been refreshed alongside it.
	if got := binary.BigEndian.Uint16(out[ethHdrLen+4 : ethHdrLen+6]); got != 8+14 {
		t.Errorf("IPv6 payload length = %d, want %d (UDP header + Echo Response)", got, 8+14)
	}

	// The reflected response is UPF -> gNB; its UDP checksum must validate.
	if !validUDPv6Checksum(testUPFN3v6, testGNBv6, out[ethHdrLen+40:]) {
		t.Error("Echo Response UDP-over-IPv6 checksum does not validate (mandatory over IPv6)")
	}
}

// TestRouterSolicitationIntercept checks that an inner ICMPv6 Router
// Solicitation, after decapsulation, is intercepted: the packet is dropped AND
// its TEID and UE source address are emitted to userspace on rs_event_map. The
// event is the contract that drives the RA responder, so asserting it (not just
// the drop) is what proves SLAAC would actually fire.
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
	if action != XDP_DROP {
		t.Fatalf("Router Solicitation not intercepted: got XDP action %d, want XDP_DROP (%d)", action, XDP_DROP)
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
