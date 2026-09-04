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

// FAR_BUFF | FAR_NOCP: the idle-UE action that triggers paging and capture.
const farBuffNocp = 0x04 | 0x08

// readRingRecord reads one record from a ring buffer within a deadline, or
// returns nil so a missing capture fails the assertion rather than hanging.
func readRingRecord(t *testing.T, r *ringbuf.Reader) []byte {
	t.Helper()

	r.SetDeadline(time.Now().Add(2 * time.Second))

	var rec ringbuf.Record

	if err := r.ReadInto(&rec); err != nil {
		return nil
	}

	return bytes.Clone(rec.RawSample)
}

// buffNocpPDR builds a downlink PDR whose FAR holds BUFF|NOCP.
func buffNocpPDR(seid uint64, pdrID uint32, qfi uint8) PdrInfo {
	pdr := ipv4OuterDownlinkPDR(0x1234, testUPFN3IP, testGNBIP, qfi)
	pdr.SEID = seid
	pdr.PdrID = pdrID
	pdr.Far.Action = farBuffNocp

	return pdr
}

// TestDlBufferCaptureIPv4 checks that a downlink packet to an idle UE is captured,
// and the nocp notification is still emitted.
func TestDlBufferCaptureIPv4(t *testing.T) {
	requireProgTestRun(t)

	obj := loadProgram(t, 1, 0)

	const (
		seid  = uint64(0x51D1)
		pdrID = uint32(7)
		qfi   = uint8(5)
	)

	ueAddr := [4]byte{10, 45, 0, 2}

	pdr := buffNocpPDR(seid, pdrID, qfi)

	if err := obj.PutPdrDownlink(netip.AddrFrom4(ueAddr), pdr); err != nil {
		t.Fatalf("install BUFF|NOCP downlink PDR: %v", err)
	}

	bufReader, err := ringbuf.NewReader(obj.DlBufferMap)
	if err != nil {
		t.Fatalf("open dl_buffer_map reader: %v", err)
	}

	defer func() { _ = bufReader.Close() }()

	nocpReader, err := ringbuf.NewReader(obj.NocpMap)
	if err != nil {
		t.Fatalf("open nocp_map reader: %v", err)
	}

	defer func() { _ = nocpReader.Close() }()

	inner := ipv4Packet([4]byte{8, 8, 8, 8}, ueAddr, 17, udpDatagram(4000, 4001, []byte("hello idle ue")))

	action := runXDP(t, obj.UpfEntryFunc, ethFrame(0x0800, inner))
	if action != ActionDrop {
		t.Fatalf("BUFF|NOCP downlink got XDP action %d, want ActionDrop (%d)", action, ActionDrop)
	}

	sample := readRingRecord(t, bufReader)
	if sample == nil {
		t.Fatal("no capture record in dl_buffer_map")
	}

	if len(sample) < 16 {
		t.Fatalf("capture record too short: %d bytes", len(sample))
	}

	if got := binary.NativeEndian.Uint64(sample[0:8]); got != seid {
		t.Errorf("record SEID = %#x, want %#x", got, seid)
	}

	if got := binary.NativeEndian.Uint16(sample[8:10]); got != uint16(pdrID) {
		t.Errorf("record PDR-ID = %d, want %d", got, pdrID)
	}

	if got := sample[12]; got != qfi {
		t.Errorf("record QFI = %d, want %d", got, qfi)
	}

	if got := sample[13]; got != 4 {
		t.Errorf("record family = %d, want 4", got)
	}

	pktLen := binary.NativeEndian.Uint16(sample[10:12])
	if int(pktLen) != len(sample)-16 {
		t.Fatalf("record length = %d, want %d", pktLen, len(sample)-16)
	}

	if !bytes.Equal(sample[16:], inner) {
		t.Error("captured payload differs from the injected L3 packet")
	}

	if nocp := readRingRecord(t, nocpReader); nocp == nil {
		t.Error("no nocp notification: capture must never cost the page")
	}

	if got := obj.GetDlBufferCounters().Captured; got != 1 {
		t.Errorf("captured counter = %d, want 1", got)
	}
}

// TestDlBufferCaptureIPv6 is TestDlBufferCaptureIPv4 on the IPv6 inner path.
func TestDlBufferCaptureIPv6(t *testing.T) {
	requireProgTestRun(t)

	obj := loadProgram(t, 1, 0)

	const (
		seid  = uint64(0x51D2)
		pdrID = uint32(9)
		qfi   = uint8(1)
	)

	uePrefix := netip.MustParseAddr("2001:db8::")

	pdr := buffNocpPDR(seid, pdrID, qfi)

	if err := obj.PutPdrDownlink(uePrefix, pdr); err != nil {
		t.Fatalf("install BUFF|NOCP downlink IPv6 PDR: %v", err)
	}

	bufReader, err := ringbuf.NewReader(obj.DlBufferMap)
	if err != nil {
		t.Fatalf("open dl_buffer_map reader: %v", err)
	}

	defer func() { _ = bufReader.Close() }()

	dst := [16]byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}
	src := [16]byte{0x20, 0x01, 0x48, 0x60, 0x48, 0x60, 0, 0, 0, 0, 0, 0, 0, 0, 0x88, 0x88}

	inner := ipv6Packet(src, dst, 17, udpDatagram(4000, 53, []byte("v6 idle")))

	action := runXDP(t, obj.UpfEntryFunc, ethFrame(0x86DD, inner))
	if action != ActionDrop {
		t.Fatalf("BUFF|NOCP IPv6 downlink got XDP action %d, want ActionDrop (%d)", action, ActionDrop)
	}

	sample := readRingRecord(t, bufReader)
	if sample == nil {
		t.Fatal("no capture record in dl_buffer_map")
	}

	if got := binary.NativeEndian.Uint64(sample[0:8]); got != seid {
		t.Errorf("record SEID = %#x, want %#x", got, seid)
	}

	if got := sample[13]; got != 6 {
		t.Errorf("record family = %d, want 6", got)
	}

	if !bytes.Equal(sample[16:], inner) {
		t.Error("captured payload differs from the injected L3 packet")
	}
}

// TestDlBufferCaptureWithoutResponder checks that capture works with readers opened only after the packet ran.
func TestDlBufferCaptureWithoutResponder(t *testing.T) {
	requireProgTestRun(t)

	obj := loadProgram(t, 1, 0)

	ueAddr := [4]byte{10, 45, 0, 3}

	pdr := buffNocpPDR(1, 1, 1)

	if err := obj.PutPdrDownlink(netip.AddrFrom4(ueAddr), pdr); err != nil {
		t.Fatalf("install BUFF|NOCP downlink PDR: %v", err)
	}

	inner := ipv4Packet([4]byte{8, 8, 8, 8}, ueAddr, 17, udpDatagram(4000, 4001, nil))

	if action := runXDP(t, obj.UpfEntryFunc, ethFrame(0x0800, inner)); action != ActionDrop {
		t.Fatalf("BUFF|NOCP downlink got XDP action %d, want ActionDrop (%d)", action, ActionDrop)
	}

	bufReader, err := ringbuf.NewReader(obj.DlBufferMap)
	if err != nil {
		t.Fatalf("open dl_buffer_map reader: %v", err)
	}

	defer func() { _ = bufReader.Close() }()

	nocpReader, err := ringbuf.NewReader(obj.NocpMap)
	if err != nil {
		t.Fatalf("open nocp_map reader: %v", err)
	}

	defer func() { _ = nocpReader.Close() }()

	if sample := readRingRecord(t, bufReader); sample == nil {
		t.Fatal("no capture record without any responder setup")
	}

	if nocp := readRingRecord(t, nocpReader); nocp == nil {
		t.Error("paging notification missing")
	}
}
