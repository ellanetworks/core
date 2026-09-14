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

	"github.com/cilium/ebpf/ringbuf"
)

const (
	gtpIETEIDDataI   = 16
	gtpIEPeerAddress = 133
	gtpIERecoveryTV  = 14
)

func errorIndicationIEs(teid uint32, peer netip.Addr) []byte {
	ies := []byte{gtpIETEIDDataI, 0, 0, 0, 0}
	binary.BigEndian.PutUint32(ies[1:5], teid)

	addr := peer.AsSlice()
	ies = append(ies, gtpIEPeerAddress, 0, byte(len(addr)))
	ies = append(ies, addr...)

	return ies
}

func errorIndicationGTP(ies []byte) []byte {
	gtp := make([]byte, 12)
	gtp[0] = 0x32
	gtp[1] = 26
	binary.BigEndian.PutUint16(gtp[2:4], uint16(4+len(ies)))

	return append(gtp, ies...)
}

func readErrorIndication(t *testing.T, rd *ringbuf.Reader) (ErrorIndication, bool) {
	t.Helper()

	rd.SetDeadline(time.Now().Add(time.Second))

	rec, err := rd.Read()
	if err != nil {
		return ErrorIndication{}, false
	}

	var ev ErrorIndication
	if err := binary.Read(bytes.NewReader(rec.RawSample), binary.NativeEndian, &ev); err != nil {
		t.Fatalf("decode error indication event: %v", err)
	}

	return ev, true
}

// TS 29.281 §7.3.1, TS 29.244 §5.10
func TestErrorIndicationReportsTheRemoteFTEID(t *testing.T) {
	requireProgTestRun(t)

	obj := loadN3N6Program(t)

	rd, err := ringbuf.NewReader(obj.ErrorIndMap)
	if err != nil {
		t.Fatalf("open error_ind ring buffer: %v", err)
	}

	defer func() { _ = rd.Close() }()

	const teid = 0x0BAD7E1D

	peer := netip.AddrFrom4(testGNBIP)
	frame := ethFrame(0x0800, ipv4Packet(testGNBIP, testUPFN3IP, 17,
		udpDatagram(GTPUDPPort, GTPUDPPort, errorIndicationGTP(errorIndicationIEs(teid, peer)))))

	if action := runXDP(t, obj.UpfEntryFunc, frame); action != ActionPass {
		t.Fatalf("Error Indication got XDP action %d, want ActionPass (%d)", action, ActionPass)
	}

	ev, ok := readErrorIndication(t, rd)
	if !ok {
		t.Fatal("no error indication event emitted; the SMF is never told the tunnel is dead")
	}

	if ev.TEID != teid {
		t.Errorf("reported TEID = %#x, want %#x (TS 29.281 §8.3)", ev.TEID, uint32(teid))
	}

	if got := In6AddrToIP(ev.PeerAddr); got != peer {
		t.Errorf("reported peer = %s, want %s (TS 29.281 §8.4)", got, peer)
	}
}

// TS 29.281 §8.4
func TestErrorIndicationReportsAnIPv6Peer(t *testing.T) {
	requireProgTestRun(t)

	obj := loadN3N6Program(t)

	rd, err := ringbuf.NewReader(obj.ErrorIndMap)
	if err != nil {
		t.Fatalf("open error_ind ring buffer: %v", err)
	}

	defer func() { _ = rd.Close() }()

	const teid = 0x11223344

	peer := netip.AddrFrom16(testGNBv6)
	frame := gtpV6Outer(errorIndicationGTP(errorIndicationIEs(teid, peer)))

	if action := runXDP(t, obj.UpfEntryFunc, frame); action != ActionPass {
		t.Fatalf("Error Indication over IPv6 transport got XDP action %d, want ActionPass (%d)", action, ActionPass)
	}

	ev, ok := readErrorIndication(t, rd)
	if !ok {
		t.Fatal("no error indication event emitted for an IPv6 peer")
	}

	if ev.TEID != teid {
		t.Errorf("reported TEID = %#x, want %#x", ev.TEID, uint32(teid))
	}

	if got := In6AddrToIP(ev.PeerAddr); got != peer {
		t.Errorf("reported peer = %s, want %s", got, peer)
	}
}

// TS 29.281 §8.1
func TestErrorIndicationSkipsAPrecedingRecoveryIE(t *testing.T) {
	requireProgTestRun(t)

	obj := loadN3N6Program(t)

	rd, err := ringbuf.NewReader(obj.ErrorIndMap)
	if err != nil {
		t.Fatalf("open error_ind ring buffer: %v", err)
	}

	defer func() { _ = rd.Close() }()

	const teid = 0x55667788

	peer := netip.AddrFrom4(testGNBIP)
	ies := append([]byte{gtpIERecoveryTV, 0}, errorIndicationIEs(teid, peer)...)
	frame := ethFrame(0x0800, ipv4Packet(testGNBIP, testUPFN3IP, 17,
		udpDatagram(GTPUDPPort, GTPUDPPort, errorIndicationGTP(ies))))

	if action := runXDP(t, obj.UpfEntryFunc, frame); action != ActionPass {
		t.Fatalf("got XDP action %d, want ActionPass (%d)", action, ActionPass)
	}

	ev, ok := readErrorIndication(t, rd)
	if !ok {
		t.Fatal("a Recovery IE ahead of the mandatory IEs suppressed the report")
	}

	if ev.TEID != teid {
		t.Errorf("reported TEID = %#x, want %#x", ev.TEID, uint32(teid))
	}
}

// TS 29.281 §5.1: the message length bounds the IEs, so bytes past it — frame
// padding, or a peer's trailing garbage — are not parsed as one.
func TestErrorIndicationIgnoresBytesPastTheMessageLength(t *testing.T) {
	requireProgTestRun(t)

	obj := loadN3N6Program(t)

	rd, err := ringbuf.NewReader(obj.ErrorIndMap)
	if err != nil {
		t.Fatalf("open error_ind ring buffer: %v", err)
	}

	defer func() { _ = rd.Close() }()

	const teid = 0x99aabbcc

	peer := netip.AddrFrom4(testGNBIP)
	gtp := errorIndicationGTP(errorIndicationIEs(teid, peer))

	const optionalWord, teidIE = 4, 5

	binary.BigEndian.PutUint16(gtp[2:4], optionalWord+teidIE)

	frame := ethFrame(0x0800, ipv4Packet(testGNBIP, testUPFN3IP, 17,
		udpDatagram(GTPUDPPort, GTPUDPPort, gtp)))

	if action := runXDP(t, obj.UpfEntryFunc, frame); action != ActionPass {
		t.Fatalf("got XDP action %d, want ActionPass (%d)", action, ActionPass)
	}

	if ev, ok := readErrorIndication(t, rd); ok {
		t.Errorf("a Peer Address IE past the declared message length was parsed, reporting %#x at %s",
			ev.TEID, In6AddrToIP(ev.PeerAddr))
	}
}

// TS 29.281 Table 7.3.1-1
func TestErrorIndicationWithoutTheMandatoryIEsReportsNothing(t *testing.T) {
	requireProgTestRun(t)

	obj := loadN3N6Program(t)

	rd, err := ringbuf.NewReader(obj.ErrorIndMap)
	if err != nil {
		t.Fatalf("open error_ind ring buffer: %v", err)
	}

	defer func() { _ = rd.Close() }()

	frame := ethFrame(0x0800, ipv4Packet(testGNBIP, testUPFN3IP, 17,
		udpDatagram(GTPUDPPort, GTPUDPPort, errorIndicationGTP(nil))))

	if action := runXDP(t, obj.UpfEntryFunc, frame); action != ActionPass {
		t.Fatalf("got XDP action %d, want ActionPass (%d)", action, ActionPass)
	}

	if ev, ok := readErrorIndication(t, rd); ok {
		t.Errorf("an Error Indication with no IEs reported F-TEID %#x, want no report", ev.TEID)
	}
}
