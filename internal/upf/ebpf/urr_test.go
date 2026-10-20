// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package ebpf

import (
	"encoding/binary"
	"net/netip"
	"slices"
	"testing"

	"golang.org/x/sys/unix"
)

func bootNanos(t *testing.T) uint64 {
	t.Helper()

	var ts unix.Timespec
	if err := unix.ClockGettime(unix.CLOCK_BOOTTIME, &ts); err != nil {
		t.Fatalf("boot clock: %v", err)
	}

	return uint64(ts.Nano())
}

func downlinkURRProgram(t *testing.T, seid uint64, urrID uint32) *BpfObjects {
	t.Helper()

	obj := loadProgram(t, 1, 0)
	if err := obj.NewUrr(seid, urrID); err != nil {
		t.Fatalf("NewUrr: %v", err)
	}

	pdr := ipv4OuterDownlinkPDR(uint32(seid), testUPFN3IP, testGNBIP, 5)
	pdr.UrrID = urrID
	pdr.SEID = seid

	if err := obj.PutPdrDownlink(netip.AddrFrom4(ueIP), pdr); err != nil {
		t.Fatalf("install downlink PDR: %v", err)
	}

	if err := obj.PutPdrDownlink(netip.MustParseAddr("2001:db8::"), pdr); err != nil {
		t.Fatalf("install IPv6 downlink PDR: %v", err)
	}

	return obj
}

func TestURRCountsPacketsAndTheirTimes(t *testing.T) {
	requireProgTestRun(t)

	const (
		seid  = 0x55525301
		urrID = 21
	)

	obj := downlinkURRProgram(t, seid, urrID)
	frame := ethFrame(0x0800, ipv4Packet(serverIP, ueIP, 17, udpDatagram(4000, 53, nil)))

	before := bootNanos(t)
	runXDP(t, obj.UpfEntryFunc, frame)
	runXDP(t, obj.UpfEntryFunc, frame)
	after := bootNanos(t)

	perCPU, err := obj.ReadUrr(seid, urrID)
	if err != nil {
		t.Fatalf("ReadUrr: %v", err)
	}

	var packets, first, last uint64

	for _, v := range perCPU {
		packets += v.DownlinkPackets + v.UplinkPackets

		if v.FirstPacketNs != 0 && (first == 0 || v.FirstPacketNs < first) {
			first = v.FirstPacketNs
		}

		last = max(last, v.LastPacketNs)
	}

	if packets != 2 {
		t.Fatalf("URR counted %d packets, want 2", packets)
	}

	if first < before || last > after || first > last {
		t.Fatalf("first/last packet %d/%d, want before <= first <= last <= after (%d, %d)", first, last, before, after)
	}
}

func TestTakeUrrFirstPacketResetsOnlyTheFirstPacketTime(t *testing.T) {
	requireProgTestRun(t)

	const (
		seid  = 0x55525302
		urrID = 22
	)

	obj := downlinkURRProgram(t, seid, urrID)
	inner := ipv4Packet(serverIP, ueIP, 17, udpDatagram(4000, 53, nil))
	frame := ethFrame(0x0800, inner)

	before := bootNanos(t)
	runXDP(t, obj.UpfEntryFunc, frame)
	after := bootNanos(t)

	first, err := obj.TakeUrrFirstPacket(seid, urrID)
	if err != nil {
		t.Fatalf("TakeUrrFirstPacket: %v", err)
	}

	if first < before || first > after {
		t.Fatalf("first packet %d, want between %d and %d", first, before, after)
	}

	if again, err := obj.TakeUrrFirstPacket(seid, urrID); err != nil || again != 0 {
		t.Fatalf("second take = %d (%v), want 0: the time is taken once", again, err)
	}

	perCPU, err := obj.ReadUrr(seid, urrID)
	if err != nil {
		t.Fatalf("ReadUrr: %v", err)
	}

	if slices.ContainsFunc(perCPU, func(v N3N6EntrypointUrrCounters) bool { return v.FirstPacketNs != 0 }) {
		t.Fatalf("per-CPU values %+v, want every first packet time reset", perCPU)
	}

	if got := readURR(t, obj, seid, urrID); got.Downlink != uint64(len(inner)) {
		t.Fatalf("URR = %d down after the take, want the %d counted before it", got.Downlink, len(inner))
	}

	runXDP(t, obj.UpfEntryFunc, frame)

	if next, err := obj.TakeUrrFirstPacket(seid, urrID); err != nil || next <= after {
		t.Fatalf("first packet after the take = %d (%v), want the new packet's time", next, err)
	}
}

func TestTakeUrrFirstPacketOfAMissingURR(t *testing.T) {
	requireProgTestRun(t)

	obj := loadProgram(t, 1, 0)

	if first, err := obj.TakeUrrFirstPacket(0x55525303, 1); err != nil || first != 0 {
		t.Fatalf("TakeUrrFirstPacket = %d (%v), want 0 and no error", first, err)
	}
}

func TestInconsistentIPLengthIsDroppedUnbilled(t *testing.T) {
	requireProgTestRun(t)

	setIPv4TotalLength := func(pkt []byte, n uint16) []byte {
		binary.BigEndian.PutUint16(pkt[2:4], n)
		binary.BigEndian.PutUint16(pkt[10:12], 0)
		binary.BigEndian.PutUint16(pkt[10:12], ipv4HeaderChecksum(pkt[:20]))

		return pkt
	}

	setIPv6PayloadLength := func(pkt []byte, n uint16) []byte {
		binary.BigEndian.PutUint16(pkt[4:6], n)

		return pkt
	}

	server6 := [16]byte{0x20, 0x01, 0x48, 0x60, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x88, 0x88}

	cases := []struct {
		name      string
		etherType uint16
		packet    func() []byte
	}{
		{"IPv4 total length past the frame", 0x0800, func() []byte {
			pkt := ipv4Packet(serverIP, ueIP, 17, udpDatagram(4000, 53, nil))
			return setIPv4TotalLength(pkt, uint16(len(pkt)+100))
		}},
		{"IPv4 total length inside the header", 0x0800, func() []byte {
			return setIPv4TotalLength(ipv4Packet(serverIP, ueIP, 17, udpDatagram(4000, 53, nil)), 12)
		}},
		{"IPv6 zero payload length", 0x86DD, func() []byte {
			return setIPv6PayloadLength(ipv6Packet(server6, testUEv6, 17, udpDatagram(4000, 53, nil)), 0)
		}},
		{"IPv6 payload length past the frame", 0x86DD, func() []byte {
			pkt := ipv6Packet(server6, testUEv6, 17, udpDatagram(4000, 53, nil))
			return setIPv6PayloadLength(pkt, uint16(len(pkt)))
		}},
	}

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			seid := uint64(0x55525310 + i)
			urrID := uint32(30 + i)

			obj := downlinkURRProgram(t, seid, urrID)
			before := DropCount(obj, Downlink, "malformed_header")

			action := runXDP(t, obj.UpfEntryFunc, ethFrame(tc.etherType, tc.packet()))
			if action != ActionDrop {
				t.Fatalf("XDP action %d, want a drop: a packet whose IP length disagrees with the frame cannot be billed", action)
			}

			if got := sumURR(t, obj, seid, urrID); got != 0 {
				t.Fatalf("URR billed %d bytes for a dropped packet", got)
			}

			if got := DropCount(obj, Downlink, "malformed_header") - before; got != 1 {
				t.Fatalf("malformed_header drops = %d, want 1", got)
			}
		})
	}
}
