// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package ebpf

import (
	"bytes"
	"encoding/binary"
	"net/netip"
	"testing"
)

func loadProgramLocalSwitch(t *testing.T) *BpfObjects {
	t.Helper()

	obj := NewBpfObjects(false, false, true, 0, 1, 0, 0)
	if err := obj.Load(); err != nil {
		t.Fatalf("load N3/N6 objects: %v", err)
	}

	t.Cleanup(func() { _ = obj.Close() })

	return obj
}

func TestLocalSwitchIPv4(t *testing.T) {
	requireProgTestRun(t)

	obj := loadProgramLocalSwitch(t)

	var (
		ueAIP = [4]byte{10, 0, 0, 9}
		ueBIP = [4]byte{10, 0, 0, 10}
		teidA = uint32(0x4C5331)
		teidB = uint32(0x4C5332)
	)

	ulPdr := PdrInfo{
		OuterHeaderRemoval: 0,
		IMSI:               "001010000000001",
		Far:                FarInfo{Action: 0x02},
		Qer:                QerInfo{GateStatusUL: 0, MaxBitrateUL: 0},
		UEIPv4:             netip.AddrFrom4(ueAIP),
		UEIPv6Prefix:       canonicalUEv6Prefix,
	}
	if err := obj.PutPdrUplink(teidA, ulPdr); err != nil {
		t.Fatalf("install uplink PDR: %v", err)
	}

	dlPdr := ipv4OuterDownlinkPDR(teidB, testUPFN3IP, testGNBIP, 5)
	if err := obj.PutPdrDownlink(netip.AddrFrom4(ueBIP), dlPdr); err != nil {
		t.Fatalf("install downlink PDR: %v", err)
	}

	inner := ipv4Packet(ueAIP, ueBIP, 17, udpDatagram(4000, 53, nil))

	action, out := runXDPOut(t, obj.UpfEntryFunc, uplinkGPDU(teidA, inner))

	if action == ActionDrop || action == ActionAborted {
		t.Fatalf("local-switched packet got XDP action %d, want a forwarding action", action)
	}

	f := parseGTPv4Frame(t, out)

	if f.gtpMsgType != 0xFF {
		t.Errorf("GTP message type = %#02x, want 0xFF (G-PDU)", f.gtpMsgType)
	}

	if f.teid != teidB {
		t.Errorf("outer TEID = %#x, want %#x (destination UE's tunnel)", f.teid, teidB)
	}

	if !bytes.Equal(f.outerSrc[:], testUPFN3IP[:]) {
		t.Errorf("outer src = %v, want %v (UPF N3)", f.outerSrc, testUPFN3IP)
	}

	if !bytes.Equal(f.outerDst[:], testGNBIP[:]) {
		t.Errorf("outer dst = %v, want %v (gNB)", f.outerDst, testGNBIP)
	}

	if f.qfi != 5 {
		t.Errorf("QFI = %d, want 5", f.qfi)
	}

	if !bytes.Equal(f.inner, inner) {
		t.Errorf("inner packet altered by local switch:\n got %x\nwant %x", f.inner, inner)
	}
}

func TestLocalSwitchDisabled(t *testing.T) {
	requireProgTestRun(t)

	obj := loadN3N6Program(t)

	var (
		ueAIP = [4]byte{10, 0, 0, 9}
		ueBIP = [4]byte{10, 0, 0, 10}
		teidA = uint32(0x4C5344)
		teidB = uint32(0x4C5345)
	)

	ulPdr := PdrInfo{
		OuterHeaderRemoval: 0,
		IMSI:               "001010000000001",
		Far:                FarInfo{Action: 0x02},
		Qer:                QerInfo{GateStatusUL: 0, MaxBitrateUL: 0},
		UEIPv4:             netip.AddrFrom4(ueAIP),
		UEIPv6Prefix:       canonicalUEv6Prefix,
	}
	if err := obj.PutPdrUplink(teidA, ulPdr); err != nil {
		t.Fatalf("install uplink PDR: %v", err)
	}

	dlPdr := ipv4OuterDownlinkPDR(teidB, testUPFN3IP, testGNBIP, 5)
	if err := obj.PutPdrDownlink(netip.AddrFrom4(ueBIP), dlPdr); err != nil {
		t.Fatalf("install downlink PDR: %v", err)
	}

	inner := ipv4Packet(ueAIP, ueBIP, 17, udpDatagram(4000, 53, nil))

	action, out := runXDPOut(t, obj.UpfEntryFunc, uplinkGPDU(teidA, inner))

	if action == ActionDrop || action == ActionAborted {
		t.Fatalf("packet got XDP action %d, want a forwarding action (N6 path)", action)
	}

	if len(out) >= ethHdrLen+20+8 && binary.BigEndian.Uint16(out[ethHdrLen+2:ethHdrLen+4]) == 0x0800 {
		udpOff := ethHdrLen + 20
		if len(out) >= udpOff+8 && binary.BigEndian.Uint16(out[udpOff+2:udpOff+4]) == GTPUDPPort {
			t.Fatalf("packet was GTP-encapsulated (local switch fired despite being disabled): outer dst port = %d", GTPUDPPort)
		}
	}
}

func TestLocalSwitchIPv6(t *testing.T) {
	requireProgTestRun(t)

	obj := loadProgramLocalSwitch(t)

	var (
		ueAv6 = [16]byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x09}
		ueBv6 = [16]byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0x0a}
		teidA = uint32(0x4C5636)
		teidB = uint32(0x4C5637)
	)

	ueAPrefix := netip.AddrFrom16(ueAv6)

	ueAPrefixBytes := ueAPrefix.As16()
	for i := 8; i < 16; i++ {
		ueAPrefixBytes[i] = 0
	}

	ueBPrefix := netip.AddrFrom16(ueBv6)

	ueBPrefixBytes := ueBPrefix.As16()
	for i := 8; i < 16; i++ {
		ueBPrefixBytes[i] = 0
	}

	ulPdr := PdrInfo{
		OuterHeaderRemoval: 0,
		IMSI:               "001010000000001",
		Far:                FarInfo{Action: 0x02},
		Qer:                QerInfo{GateStatusUL: 0, MaxBitrateUL: 0},
		UEIPv4:             canonicalUEv4,
		UEIPv6Prefix:       netip.AddrFrom16(ueAPrefixBytes),
	}
	if err := obj.PutPdrUplink(teidA, ulPdr); err != nil {
		t.Fatalf("install uplink PDR: %v", err)
	}

	dlPdr := ipv4OuterDownlinkPDR(teidB, testUPFN3IP, testGNBIP, 5)
	if err := obj.PutPdrDownlink(netip.AddrFrom16(ueBPrefixBytes), dlPdr); err != nil {
		t.Fatalf("install downlink PDR: %v", err)
	}

	inner := ipv6Packet(ueAv6, ueBv6, 17, udpDatagram(4000, 53, nil))

	action, out := runXDPOut(t, obj.UpfEntryFunc, uplinkGPDU(teidA, inner))

	if action == ActionDrop || action == ActionAborted {
		t.Fatalf("local-switched packet got XDP action %d, want a forwarding action", action)
	}

	f := parseGTPv4Frame(t, out)

	if f.gtpMsgType != 0xFF {
		t.Errorf("GTP message type = %#02x, want 0xFF (G-PDU)", f.gtpMsgType)
	}

	if f.teid != teidB {
		t.Errorf("outer TEID = %#x, want %#x (destination UE's tunnel)", f.teid, teidB)
	}

	if !bytes.Equal(f.outerSrc[:], testUPFN3IP[:]) {
		t.Errorf("outer src = %v, want %v (UPF N3)", f.outerSrc, testUPFN3IP)
	}

	if !bytes.Equal(f.outerDst[:], testGNBIP[:]) {
		t.Errorf("outer dst = %v, want %v (gNB)", f.outerDst, testGNBIP)
	}

	if f.qfi != 5 {
		t.Errorf("QFI = %d, want 5", f.qfi)
	}

	if !bytes.Equal(f.inner, inner) {
		t.Errorf("inner packet altered by local switch:\n got %x\nwant %x", f.inner, inner)
	}
}

func TestLocalSwitchQERGateClosedDL(t *testing.T) {
	requireProgTestRun(t)

	obj := loadProgramLocalSwitch(t)

	var (
		ueAIP = [4]byte{10, 0, 0, 9}
		ueBIP = [4]byte{10, 0, 0, 10}
		teidA = uint32(0x4C5147)
		teidB = uint32(0x4C5148)
	)

	ulPdr := PdrInfo{
		OuterHeaderRemoval: 0,
		IMSI:               "001010000000001",
		Far:                FarInfo{Action: 0x02},
		Qer:                QerInfo{GateStatusUL: 0, MaxBitrateUL: 0},
		UEIPv4:             netip.AddrFrom4(ueAIP),
		UEIPv6Prefix:       canonicalUEv6Prefix,
	}
	if err := obj.PutPdrUplink(teidA, ulPdr); err != nil {
		t.Fatalf("install uplink PDR: %v", err)
	}

	dlPdr := ipv4OuterDownlinkPDR(teidB, testUPFN3IP, testGNBIP, 5)

	dlPdr.Qer.GateStatusDL = 1
	if err := obj.PutPdrDownlink(netip.AddrFrom4(ueBIP), dlPdr); err != nil {
		t.Fatalf("install downlink PDR: %v", err)
	}

	inner := ipv4Packet(ueAIP, ueBIP, 17, udpDatagram(4000, 53, nil))

	action := runXDP(t, obj.UpfEntryFunc, uplinkGPDU(teidA, inner))

	if action != ActionDrop {
		t.Fatalf("local switch with closed DL gate: got XDP action %d, want ActionDrop (%d)", action, ActionDrop)
	}

	if got := DropCount(obj, Uplink, "qer_gate_closed"); got != 1 {
		t.Errorf("qer_gate_closed = %d, want 1", got)
	}
}

func TestLocalSwitchSDFDeny(t *testing.T) {
	requireProgTestRun(t)

	const filterIndex = 1

	obj := loadProgramLocalSwitch(t)

	var (
		ueAIP = [4]byte{10, 0, 0, 9}
		ueBIP = [4]byte{10, 0, 0, 10}
		teidA = uint32(0x4C5344)
		teidB = uint32(0x4C5345)
	)

	ulPdr := PdrInfo{
		OuterHeaderRemoval: 0,
		IMSI:               "001010000000001",
		Far:                FarInfo{Action: 0x02},
		Qer:                QerInfo{GateStatusUL: 0, MaxBitrateUL: 0},
		UEIPv4:             netip.AddrFrom4(ueAIP),
		UEIPv6Prefix:       canonicalUEv6Prefix,
	}
	if err := obj.PutPdrUplink(teidA, ulPdr); err != nil {
		t.Fatalf("install uplink PDR: %v", err)
	}

	dlPdr := ipv4OuterDownlinkPDR(teidB, testUPFN3IP, testGNBIP, 5)

	dlPdr.FilterMapIndex = filterIndex
	if err := obj.PutPdrDownlink(netip.AddrFrom4(ueBIP), dlPdr); err != nil {
		t.Fatalf("install downlink PDR: %v", err)
	}

	putSDFFilter(t, obj, filterIndex, []SdfRule{
		sdfRuleIPv4(ueAIP, 32, 0, 0, 17, SdfActionDeny),
	})

	inner := ipv4Packet(ueAIP, ueBIP, 17, udpDatagram(4000, 53, nil))

	action := runXDP(t, obj.UpfEntryFunc, uplinkGPDU(teidA, inner))

	if action != ActionDrop {
		t.Fatalf("local switch with SDF deny: got XDP action %d, want ActionDrop (%d)", action, ActionDrop)
	}

	if got := DropCount(obj, Uplink, "sdf_filter"); got != 1 {
		t.Errorf("sdf_filter = %d, want 1", got)
	}
}

func TestLocalSwitchQERRateLimit(t *testing.T) {
	requireProgTestRun(t)

	const (
		seid    = 0x4C514C
		qerID   = 9
		rateBps = 100_000
	)

	obj := loadProgramLocalSwitch(t)

	var (
		ueAIP = [4]byte{10, 0, 0, 9}
		ueBIP = [4]byte{10, 0, 0, 10}
		teidA = uint32(0x4C5152)
		teidB = uint32(0x4C5153)
	)

	ulPdr := PdrInfo{
		SEID:               seid,
		OuterHeaderRemoval: 0,
		IMSI:               "001010000000001",
		Far:                FarInfo{Action: 0x02},
		Qer:                QerInfo{GateStatusUL: 0, MaxBitrateUL: 0},
		UEIPv4:             netip.AddrFrom4(ueAIP),
		UEIPv6Prefix:       canonicalUEv6Prefix,
	}
	if err := obj.PutPdrUplink(teidA, ulPdr); err != nil {
		t.Fatalf("install uplink PDR: %v", err)
	}

	dlPdr := ipv4OuterDownlinkPDR(teidB, testUPFN3IP, testGNBIP, 5)
	dlPdr.SEID = seid + 1
	dlPdr.QerID = qerID

	dlPdr.Qer.MaxBitrateDL = rateBps
	if err := obj.PutPdrDownlink(netip.AddrFrom4(ueBIP), dlPdr); err != nil {
		t.Fatalf("install downlink PDR: %v", err)
	}

	inner := innerIPv4UDPSized(ueBIP, 1250)

	if action := runXDP(t, obj.UpfEntryFunc, uplinkGPDU(teidA, inner)); action == ActionDrop {
		t.Fatalf("first packet was dropped, %d as qer_rate_limit", DropCount(obj, Uplink, "qer_rate_limit"))
	}

	action := runXDP(t, obj.UpfEntryFunc, uplinkGPDU(teidA, inner))

	if got := DropCount(obj, Uplink, "qer_rate_limit"); action != ActionDrop || got != 1 {
		t.Fatalf("second packet got action %d with %d rate-limit drops, want ActionDrop (%d) with 1", action, got, ActionDrop)
	}
}

func TestLocalSwitchUplinkQERRateLimit(t *testing.T) {
	requireProgTestRun(t)

	const (
		seid    = 0x4C5551
		qerID   = 3
		rateBps = 100_000
	)

	obj := loadProgramLocalSwitch(t)

	var (
		ueAIP = [4]byte{10, 0, 0, 9}
		ueBIP = [4]byte{10, 0, 0, 10}
		teidA = uint32(0x4C5552)
		teidB = uint32(0x4C5553)
	)

	ulPdr := PdrInfo{
		SEID:               seid,
		QerID:              qerID,
		OuterHeaderRemoval: 0,
		IMSI:               "001010000000001",
		Far:                FarInfo{Action: 0x02},
		Qer:                QerInfo{GateStatusUL: 0, MaxBitrateUL: rateBps},
		UEIPv4:             netip.AddrFrom4(ueAIP),
		UEIPv6Prefix:       canonicalUEv6Prefix,
	}
	if err := obj.PutPdrUplink(teidA, ulPdr); err != nil {
		t.Fatalf("install uplink PDR: %v", err)
	}

	dlPdr := ipv4OuterDownlinkPDR(teidB, testUPFN3IP, testGNBIP, 5)
	if err := obj.PutPdrDownlink(netip.AddrFrom4(ueBIP), dlPdr); err != nil {
		t.Fatalf("install downlink PDR: %v", err)
	}

	inner := innerIPv4UDPSized(ueBIP, 1250)

	if action := runXDP(t, obj.UpfEntryFunc, uplinkGPDU(teidA, inner)); action == ActionDrop {
		t.Fatalf("first packet was dropped, %d as qer_rate_limit", DropCount(obj, Uplink, "qer_rate_limit"))
	}

	action := runXDP(t, obj.UpfEntryFunc, uplinkGPDU(teidA, inner))

	if got := DropCount(obj, Uplink, "qer_rate_limit"); action != ActionDrop || got != 1 {
		t.Fatalf("second packet got action %d with %d rate-limit drops, want ActionDrop (%d) with 1", action, got, ActionDrop)
	}
}

func TestLocalSwitchUplinkSDFDeny(t *testing.T) {
	requireProgTestRun(t)

	const filterIndex = 2

	obj := loadProgramLocalSwitch(t)

	var (
		ueAIP = [4]byte{10, 0, 0, 9}
		ueBIP = [4]byte{10, 0, 0, 10}
		teidA = uint32(0x4C5553)
		teidB = uint32(0x4C5554)
	)

	ulPdr := PdrInfo{
		OuterHeaderRemoval: 0,
		IMSI:               "001010000000001",
		Far:                FarInfo{Action: 0x02},
		Qer:                QerInfo{GateStatusUL: 0, MaxBitrateUL: 0},
		FilterMapIndex:     filterIndex,
		UEIPv4:             netip.AddrFrom4(ueAIP),
		UEIPv6Prefix:       canonicalUEv6Prefix,
	}
	if err := obj.PutPdrUplink(teidA, ulPdr); err != nil {
		t.Fatalf("install uplink PDR: %v", err)
	}

	dlPdr := ipv4OuterDownlinkPDR(teidB, testUPFN3IP, testGNBIP, 5)
	if err := obj.PutPdrDownlink(netip.AddrFrom4(ueBIP), dlPdr); err != nil {
		t.Fatalf("install downlink PDR: %v", err)
	}

	putSDFFilter(t, obj, filterIndex, []SdfRule{
		sdfRuleIPv4(ueBIP, 32, 0, 0, 17, SdfActionDeny),
	})

	inner := ipv4Packet(ueAIP, ueBIP, 17, udpDatagram(4000, 53, nil))

	action := runXDP(t, obj.UpfEntryFunc, uplinkGPDU(teidA, inner))

	if action != ActionDrop {
		t.Fatalf("local switch with uplink SDF deny: got XDP action %d, want ActionDrop (%d)", action, ActionDrop)
	}

	if got := DropCount(obj, Uplink, "sdf_filter"); got != 1 {
		t.Errorf("sdf_filter = %d, want 1", got)
	}
}

func TestLocalSwitchStatistics(t *testing.T) {
	requireProgTestRun(t)

	obj := loadProgramLocalSwitch(t)

	var (
		ueAIP = [4]byte{10, 0, 0, 9}
		ueBIP = [4]byte{10, 0, 0, 10}
		teidA = uint32(0x4C5354)
		teidB = uint32(0x4C5355)
	)

	ulPdr := PdrInfo{
		OuterHeaderRemoval: 0,
		IMSI:               "001010000000001",
		Far:                FarInfo{Action: 0x02},
		Qer:                QerInfo{GateStatusUL: 0, MaxBitrateUL: 0},
		UEIPv4:             netip.AddrFrom4(ueAIP),
		UEIPv6Prefix:       canonicalUEv6Prefix,
	}
	if err := obj.PutPdrUplink(teidA, ulPdr); err != nil {
		t.Fatalf("install uplink PDR: %v", err)
	}

	dlPdr := ipv4OuterDownlinkPDR(teidB, testUPFN3IP, testGNBIP, 5)
	if err := obj.PutPdrDownlink(netip.AddrFrom4(ueBIP), dlPdr); err != nil {
		t.Fatalf("install downlink PDR: %v", err)
	}

	inner := ipv4Packet(ueAIP, ueBIP, 17, udpDatagram(4000, 53, nil))

	action := runXDP(t, obj.UpfEntryFunc, uplinkGPDU(teidA, inner))
	if action == ActionDrop || action == ActionAborted {
		t.Fatalf("local-switched packet got XDP action %d, want a forwarding action", action)
	}

	ulStats, ok := readStats(obj, Uplink)
	if !ok {
		t.Fatalf("could not read uplink stats")
	}

	if ulStats.PacketCounters.Tx == 0 {
		t.Errorf("uplink tx counter = 0, want > 0 (local switch forwarded via N3)")
	}

	dlStats, ok := readStats(obj, Downlink)
	if !ok {
		t.Fatalf("could not read downlink stats")
	}

	if dlStats.PacketCounters.Tx == 0 {
		t.Errorf("downlink tx counter = 0, want > 0 (local switch billed to downlink)")
	}

	want := uint64(ethHdrLen + len(inner))

	if dlStats.ByteCounter.Bytes != want {
		t.Errorf("downlink byte counter = %d, want %d: the subscriber's frame, not the encapsulated one", dlStats.ByteCounter.Bytes, want)
	}

	if ulStats.ByteCounter.Bytes != want {
		t.Errorf("uplink byte counter = %d, want %d", ulStats.ByteCounter.Bytes, want)
	}
}

func TestLocalSwitchFlowAccounting(t *testing.T) {
	requireProgTestRun(t)

	obj := loadProgramLocalSwitch(t)

	obj.FlowAccounting = true
	if err := obj.LoadWithMapReplacements(); err != nil {
		t.Fatalf("reload with flow accounting: %v", err)
	}

	var (
		ueAIP = [4]byte{10, 0, 0, 9}
		ueBIP = [4]byte{10, 0, 0, 10}
		teidA = uint32(0x4C464C)
		teidB = uint32(0x4C464D)
	)

	ulPdr := PdrInfo{
		OuterHeaderRemoval: 0,
		IMSI:               "001010000000001",
		Far:                FarInfo{Action: 0x02},
		Qer:                QerInfo{GateStatusUL: 0, MaxBitrateUL: 0},
		UEIPv4:             netip.AddrFrom4(ueAIP),
		UEIPv6Prefix:       canonicalUEv6Prefix,
	}
	if err := obj.PutPdrUplink(teidA, ulPdr); err != nil {
		t.Fatalf("install uplink PDR: %v", err)
	}

	dlPdr := ipv4OuterDownlinkPDR(teidB, testUPFN3IP, testGNBIP, 5)
	if err := obj.PutPdrDownlink(netip.AddrFrom4(ueBIP), dlPdr); err != nil {
		t.Fatalf("install downlink PDR: %v", err)
	}

	inner := ipv4Packet(ueAIP, ueBIP, 17, udpDatagram(4000, 53, nil))

	runXDP(t, obj.UpfEntryFunc, uplinkGPDU(teidA, inner))

	var (
		key N3N6EntrypointFlow
		val N3N6EntrypointFlowStats
	)

	count := 0

	it := obj.FlowStats.Iterate()
	for it.Next(&key, &val) {
		count++
	}

	if err := it.Err(); err != nil {
		t.Fatalf("iterate flow_stats: %v", err)
	}

	if count != 2 {
		t.Errorf("flow_stats entries = %d, want 2 (uplink + downlink)", count)
	}
}

func TestLocalSwitchURR(t *testing.T) {
	requireProgTestRun(t)

	const (
		ulSeid = 0x4C5552
		dlSeid = 0x4C4452
		urrID  = 7
	)

	obj := loadProgramLocalSwitch(t)

	if err := obj.NewUrr(ulSeid, urrID); err != nil {
		t.Fatalf("NewUrr (uplink): %v", err)
	}

	if err := obj.NewUrr(dlSeid, urrID); err != nil {
		t.Fatalf("NewUrr (downlink): %v", err)
	}

	var (
		ueAIP = [4]byte{10, 0, 0, 9}
		ueBIP = [4]byte{10, 0, 0, 10}
		teidA = uint32(0x4C5552)
		teidB = uint32(0x4C4452)
	)

	ulPdr := PdrInfo{
		SEID:               ulSeid,
		UrrID:              urrID,
		OuterHeaderRemoval: 0,
		IMSI:               "001010000000001",
		Far:                FarInfo{Action: 0x02},
		Qer:                QerInfo{GateStatusUL: 0, MaxBitrateUL: 0},
		UEIPv4:             netip.AddrFrom4(ueAIP),
		UEIPv6Prefix:       canonicalUEv6Prefix,
	}
	if err := obj.PutPdrUplink(teidA, ulPdr); err != nil {
		t.Fatalf("install uplink PDR: %v", err)
	}

	dlPdr := ipv4OuterDownlinkPDR(teidB, testUPFN3IP, testGNBIP, 5)
	dlPdr.SEID = dlSeid

	dlPdr.UrrID = urrID
	if err := obj.PutPdrDownlink(netip.AddrFrom4(ueBIP), dlPdr); err != nil {
		t.Fatalf("install downlink PDR: %v", err)
	}

	inner := ipv4Packet(ueAIP, ueBIP, 17, udpDatagram(4000, 53, nil))

	action := runXDP(t, obj.UpfEntryFunc, uplinkGPDU(teidA, inner))
	if action == ActionDrop || action == ActionAborted {
		t.Fatalf("local-switched packet got XDP action %d, want a forwarding action", action)
	}

	ulBytes := sumURR(t, obj, ulSeid, urrID)
	if ulBytes == 0 {
		t.Errorf("uplink URR bytes = 0, want > 0")
	}

	dlBytes := sumURR(t, obj, dlSeid, urrID)
	if dlBytes == 0 {
		t.Errorf("downlink URR bytes = 0, want > 0")
	}

	if ulBytes != dlBytes {
		t.Errorf("uplink URR (%d) != downlink URR (%d): both legs should bill the same packet", ulBytes, dlBytes)
	}
}

func TestLocalSwitchNoDownlinkPDR(t *testing.T) {
	requireProgTestRun(t)

	obj := loadProgramLocalSwitch(t)

	var (
		ueAIP = [4]byte{10, 0, 0, 9}
		teidA = uint32(0x4C4E44)
	)

	ulPdr := PdrInfo{
		OuterHeaderRemoval: 0,
		IMSI:               "001010000000001",
		Far:                FarInfo{Action: 0x02},
		Qer:                QerInfo{GateStatusUL: 0, MaxBitrateUL: 0},
		UEIPv4:             netip.AddrFrom4(ueAIP),
		UEIPv6Prefix:       canonicalUEv6Prefix,
	}
	if err := obj.PutPdrUplink(teidA, ulPdr); err != nil {
		t.Fatalf("install uplink PDR: %v", err)
	}

	inner := ipv4Packet(ueAIP, [4]byte{8, 8, 8, 8}, 17, udpDatagram(4000, 53, nil))

	action, out := runXDPOut(t, obj.UpfEntryFunc, uplinkGPDU(teidA, inner))

	if action == ActionDrop || action == ActionAborted {
		t.Fatalf("packet to internet got XDP action %d, want a forwarding action (N6 path)", action)
	}

	if len(out) >= ethHdrLen+20+8 {
		udpOff := ethHdrLen + 20
		if len(out) >= udpOff+8 && binary.BigEndian.Uint16(out[udpOff+2:udpOff+4]) == GTPUDPPort {
			t.Fatalf("packet was GTP-encapsulated (local switch fired for non-UE destination)")
		}
	}
}

func TestLocalSwitchFramedRoute(t *testing.T) {
	requireProgTestRun(t)

	obj := loadProgramLocalSwitch(t)

	var (
		ueAIP      = [4]byte{10, 0, 0, 9}
		framedUEIP = [4]byte{10, 0, 0, 10}
		teidA      = uint32(0x4C4652)
		teidB      = uint32(0x4C4653)
	)

	ulPdr := PdrInfo{
		OuterHeaderRemoval: 0,
		IMSI:               "001010000000001",
		Far:                FarInfo{Action: 0x02},
		Qer:                QerInfo{GateStatusUL: 0, MaxBitrateUL: 0},
		UEIPv4:             netip.AddrFrom4(ueAIP),
		UEIPv6Prefix:       canonicalUEv6Prefix,
	}
	if err := obj.PutPdrUplink(teidA, ulPdr); err != nil {
		t.Fatalf("install uplink PDR: %v", err)
	}

	dlPdr := ipv4OuterDownlinkPDR(teidB, testUPFN3IP, testGNBIP, 5)
	if err := obj.PutPdrDownlink(netip.AddrFrom4(framedUEIP), dlPdr); err != nil {
		t.Fatalf("install downlink PDR: %v", err)
	}

	if err := obj.PutFramedDownlink(netip.MustParsePrefix("192.168.50.0/24"), netip.AddrFrom4(framedUEIP)); err != nil {
		t.Fatalf("install framed route: %v", err)
	}

	framedDst := [4]byte{192, 168, 50, 9}
	inner := ipv4Packet(ueAIP, framedDst, 17, udpDatagram(4000, 53, nil))

	action, out := runXDPOut(t, obj.UpfEntryFunc, uplinkGPDU(teidA, inner))

	if action == ActionDrop || action == ActionAborted {
		t.Fatalf("framed-route local switch got XDP action %d, want a forwarding action", action)
	}

	f := parseGTPv4Frame(t, out)

	if f.teid != teidB {
		t.Errorf("outer TEID = %#x, want %#x (framed route did not reach the UE's tunnel)", f.teid, teidB)
	}
}

func TestLocalSwitchIPv6Transport(t *testing.T) {
	requireProgTestRun(t)

	obj := loadProgramLocalSwitch(t)

	var (
		ueAIP = [4]byte{10, 0, 0, 9}
		ueBIP = [4]byte{10, 0, 0, 10}
		teidA = uint32(0x4C5636)
		teidB = uint32(0x4C5637)
	)

	ulPdr := PdrInfo{
		OuterHeaderRemoval: 1,
		IMSI:               "001010000000001",
		Far:                FarInfo{Action: 0x02},
		Qer:                QerInfo{GateStatusUL: 0, MaxBitrateUL: 0},
		UEIPv4:             netip.AddrFrom4(ueAIP),
		UEIPv6Prefix:       canonicalUEv6Prefix,
	}
	if err := obj.PutPdrUplink(teidA, ulPdr); err != nil {
		t.Fatalf("install uplink PDR: %v", err)
	}

	dlPdr := ipv4OuterDownlinkPDR(teidB, testUPFN3IP, testGNBIP, 5)
	if err := obj.PutPdrDownlink(netip.AddrFrom4(ueBIP), dlPdr); err != nil {
		t.Fatalf("install downlink PDR: %v", err)
	}

	inner := ipv4Packet(ueAIP, ueBIP, 17, udpDatagram(4000, 53, nil))

	action, out := runXDPOut(t, obj.UpfEntryFunc, uplinkGPDUv6(teidA, inner))

	if action == ActionDrop || action == ActionAborted {
		t.Fatalf("local switch over IPv6 transport got XDP action %d, want a forwarding action", action)
	}

	f := parseGTPv4Frame(t, out)

	if f.teid != teidB {
		t.Errorf("outer TEID = %#x, want %#x", f.teid, teidB)
	}

	if !bytes.Equal(f.inner, inner) {
		t.Errorf("inner packet altered by local switch:\n got %x\nwant %x", f.inner, inner)
	}
}

func TestLocalSwitchSourceSpoofStillEnforced(t *testing.T) {
	requireProgTestRun(t)

	obj := loadProgramLocalSwitch(t)

	var (
		ueAIP = [4]byte{10, 0, 0, 9}
		ueBIP = [4]byte{10, 0, 0, 10}
		teidA = uint32(0x4C5350)
		teidB = uint32(0x4C5351)
	)

	ulPdr := PdrInfo{
		OuterHeaderRemoval: 0,
		IMSI:               "001010000000001",
		Far:                FarInfo{Action: 0x02},
		Qer:                QerInfo{GateStatusUL: 0, MaxBitrateUL: 0},
		UEIPv4:             netip.AddrFrom4(ueAIP),
		UEIPv6Prefix:       canonicalUEv6Prefix,
	}
	if err := obj.PutPdrUplink(teidA, ulPdr); err != nil {
		t.Fatalf("install uplink PDR: %v", err)
	}

	dlPdr := ipv4OuterDownlinkPDR(teidB, testUPFN3IP, testGNBIP, 5)
	if err := obj.PutPdrDownlink(netip.AddrFrom4(ueBIP), dlPdr); err != nil {
		t.Fatalf("install downlink PDR: %v", err)
	}

	spoofed := [4]byte{10, 0, 0, 99}
	inner := ipv4Packet(spoofed, ueBIP, 17, udpDatagram(4000, 53, nil))

	action := runXDP(t, obj.UpfEntryFunc, uplinkGPDU(teidA, inner))

	if action != ActionDrop {
		t.Fatalf("spoofed source got XDP action %d, want ActionDrop (%d)", action, ActionDrop)
	}

	if got := DropCount(obj, Uplink, "source_spoof_ipv4"); got != 1 {
		t.Errorf("source_spoof_ipv4 = %d, want 1", got)
	}
}

func TestLocalSwitchMTUExceeded(t *testing.T) {
	requireProgTestRun(t)

	obj := loadProgramLocalSwitch(t)

	var (
		ueAIP = [4]byte{10, 0, 0, 9}
		ueBIP = [4]byte{10, 0, 0, 10}
		teidA = uint32(0x4C4D54)
		teidB = uint32(0x4C4D55)
	)

	ulPdr := PdrInfo{
		OuterHeaderRemoval: 0,
		IMSI:               "001010000000001",
		Far:                FarInfo{Action: 0x02},
		Qer:                QerInfo{GateStatusUL: 0, MaxBitrateUL: 0},
		UEIPv4:             netip.AddrFrom4(ueAIP),
		UEIPv6Prefix:       canonicalUEv6Prefix,
	}
	if err := obj.PutPdrUplink(teidA, ulPdr); err != nil {
		t.Fatalf("install uplink PDR: %v", err)
	}

	dlPdr := ipv4OuterDownlinkPDR(teidB, testUPFN3IP, testGNBIP, 5)
	if err := obj.PutPdrDownlink(netip.AddrFrom4(ueBIP), dlPdr); err != nil {
		t.Fatalf("install downlink PDR: %v", err)
	}

	inner := innerIPv4UDPSized(ueBIP, 1400)

	action := runXDP(t, obj.UpfEntryFunc, uplinkGPDU(teidA, inner))

	if action == ActionAborted {
		t.Fatalf("local switch with normal packet got ActionAborted: %d", action)
	}
}

func TestLocalSwitchNoEncap(t *testing.T) {
	requireProgTestRun(t)

	obj := loadProgramLocalSwitch(t)

	var (
		ueAIP = [4]byte{10, 0, 0, 9}
		ueBIP = [4]byte{10, 0, 0, 10}
		teidA = uint32(0x4C4E45)
	)

	ulPdr := PdrInfo{
		OuterHeaderRemoval: 0,
		IMSI:               "001010000000001",
		Far:                FarInfo{Action: 0x02},
		Qer:                QerInfo{GateStatusUL: 0, MaxBitrateUL: 0},
		UEIPv4:             netip.AddrFrom4(ueAIP),
		UEIPv6Prefix:       canonicalUEv6Prefix,
	}
	if err := obj.PutPdrUplink(teidA, ulPdr); err != nil {
		t.Fatalf("install uplink PDR: %v", err)
	}

	dlPdr := PdrInfo{
		IMSI: "001010000000001",
		Far:  FarInfo{Action: 0x02},
		Qer:  QerInfo{GateStatusDL: 0, MaxBitrateDL: 0},
	}
	if err := obj.PutPdrDownlink(netip.AddrFrom4(ueBIP), dlPdr); err != nil {
		t.Fatalf("install downlink PDR: %v", err)
	}

	inner := ipv4Packet(ueAIP, ueBIP, 17, udpDatagram(4000, 53, nil))

	action := runXDP(t, obj.UpfEntryFunc, uplinkGPDU(teidA, inner))

	if action != ActionDrop {
		t.Fatalf("local switch with no OHC: got XDP action %d, want ActionDrop (%d)", action, ActionDrop)
	}

	if got := DropCount(obj, Uplink, "far_no_encap"); got != 1 {
		t.Errorf("far_no_encap = %d, want 1", got)
	}
}

func TestLocalSwitchFARDrop(t *testing.T) {
	requireProgTestRun(t)

	obj := loadProgramLocalSwitch(t)

	var (
		ueAIP = [4]byte{10, 0, 0, 9}
		ueBIP = [4]byte{10, 0, 0, 10}
		teidA = uint32(0x4C4644)
		teidB = uint32(0x4C4645)
	)

	ulPdr := PdrInfo{
		OuterHeaderRemoval: 0,
		IMSI:               "001010000000001",
		Far:                FarInfo{Action: 0x02},
		Qer:                QerInfo{GateStatusUL: 0, MaxBitrateUL: 0},
		UEIPv4:             netip.AddrFrom4(ueAIP),
		UEIPv6Prefix:       canonicalUEv6Prefix,
	}
	if err := obj.PutPdrUplink(teidA, ulPdr); err != nil {
		t.Fatalf("install uplink PDR: %v", err)
	}

	dlPdr := ipv4OuterDownlinkPDR(teidB, testUPFN3IP, testGNBIP, 5)

	dlPdr.Far.Action = 0x01
	if err := obj.PutPdrDownlink(netip.AddrFrom4(ueBIP), dlPdr); err != nil {
		t.Fatalf("install downlink PDR: %v", err)
	}

	inner := ipv4Packet(ueAIP, ueBIP, 17, udpDatagram(4000, 53, nil))

	action := runXDP(t, obj.UpfEntryFunc, uplinkGPDU(teidA, inner))

	if action != ActionDrop {
		t.Fatalf("local switch with FAR DROP: got XDP action %d, want ActionDrop (%d)", action, ActionDrop)
	}

	if got := DropCount(obj, Uplink, "far_no_forward"); got != 1 {
		t.Errorf("far_no_forward = %d, want 1", got)
	}
}

func TestLocalSwitchUplinkFARUnsupportedN9(t *testing.T) {
	requireProgTestRun(t)

	obj := loadProgramLocalSwitch(t)

	var (
		ueAIP = [4]byte{10, 0, 0, 9}
		teidA = uint32(0x4C4E39)
	)

	ulPdr := PdrInfo{
		OuterHeaderRemoval: 0,
		IMSI:               "001010000000001",
		Far: FarInfo{
			Action:              0x02,
			OuterHeaderCreation: 0x01,
			TeID:                0x99999999,
		},
		Qer:          QerInfo{GateStatusUL: 0, MaxBitrateUL: 0},
		UEIPv4:       netip.AddrFrom4(ueAIP),
		UEIPv6Prefix: canonicalUEv6Prefix,
	}
	if err := obj.PutPdrUplink(teidA, ulPdr); err != nil {
		t.Fatalf("install uplink PDR: %v", err)
	}

	inner := ipv4Packet(ueAIP, [4]byte{8, 8, 8, 8}, 17, udpDatagram(4000, 53, nil))

	action := runXDP(t, obj.UpfEntryFunc, uplinkGPDU(teidA, inner))

	if action != ActionDrop {
		t.Fatalf("uplink FAR with OHC (N9): got XDP action %d, want ActionDrop (%d)", action, ActionDrop)
	}

	if got := DropCount(obj, Uplink, "far_unsupported"); got != 1 {
		t.Errorf("far_unsupported = %d, want 1", got)
	}
}

func TestLocalSwitchURRNotChargedOnDrop(t *testing.T) {
	requireProgTestRun(t)

	const (
		ulSeid      = 0x4C5553
		dlSeid      = 0x4C4453
		urrID       = 7
		filterIndex = 1
	)

	var (
		ueAIP = [4]byte{10, 0, 0, 9}
		ueBIP = [4]byte{10, 0, 0, 10}
		teidA = uint32(0x4C5553)
		teidB = uint32(0x4C4453)
	)

	tests := []struct {
		name       string
		dropReason string
		setup      func(t *testing.T, obj *BpfObjects, dlPdr *PdrInfo)
	}{
		{
			name:       "dl gate closed",
			dropReason: "qer_gate_closed",
			setup: func(t *testing.T, obj *BpfObjects, dlPdr *PdrInfo) {
				dlPdr.Qer.GateStatusDL = 1
			},
		},
		{
			name:       "sdf deny",
			dropReason: "sdf_filter",
			setup: func(t *testing.T, obj *BpfObjects, dlPdr *PdrInfo) {
				dlPdr.FilterMapIndex = filterIndex
				putSDFFilter(t, obj, filterIndex, []SdfRule{
					sdfRuleIPv4(ueAIP, 32, 0, 0, 17, SdfActionDeny),
				})
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			obj := loadProgramLocalSwitch(t)

			if err := obj.NewUrr(ulSeid, urrID); err != nil {
				t.Fatalf("NewUrr (uplink): %v", err)
			}

			if err := obj.NewUrr(dlSeid, urrID); err != nil {
				t.Fatalf("NewUrr (downlink): %v", err)
			}

			ulPdr := PdrInfo{
				SEID:               ulSeid,
				UrrID:              urrID,
				OuterHeaderRemoval: 0,
				IMSI:               "001010000000001",
				Far:                FarInfo{Action: 0x02},
				Qer:                QerInfo{GateStatusUL: 0, MaxBitrateUL: 0},
				UEIPv4:             netip.AddrFrom4(ueAIP),
				UEIPv6Prefix:       canonicalUEv6Prefix,
			}
			if err := obj.PutPdrUplink(teidA, ulPdr); err != nil {
				t.Fatalf("install uplink PDR: %v", err)
			}

			dlPdr := ipv4OuterDownlinkPDR(teidB, testUPFN3IP, testGNBIP, 5)
			dlPdr.SEID = dlSeid
			dlPdr.UrrID = urrID

			tc.setup(t, obj, &dlPdr)

			if err := obj.PutPdrDownlink(netip.AddrFrom4(ueBIP), dlPdr); err != nil {
				t.Fatalf("install downlink PDR: %v", err)
			}

			inner := ipv4Packet(ueAIP, ueBIP, 17, udpDatagram(4000, 53, nil))

			action := runXDP(t, obj.UpfEntryFunc, uplinkGPDU(teidA, inner))
			if action != ActionDrop {
				t.Fatalf("got XDP action %d, want ActionDrop (%d)", action, ActionDrop)
			}

			if got := DropCount(obj, Uplink, tc.dropReason); got != 1 {
				t.Errorf("%s = %d, want 1", tc.dropReason, got)
			}

			if got := sumURR(t, obj, ulSeid, urrID); got != 0 {
				t.Errorf("uplink URR bytes = %d, want 0: the packet never left the UPF", got)
			}

			if got := sumURR(t, obj, dlSeid, urrID); got != 0 {
				t.Errorf("downlink URR bytes = %d, want 0: the packet never left the UPF", got)
			}
		})
	}
}

func TestLocalSwitchQERMetersFromL3(t *testing.T) {
	requireProgTestRun(t)

	const (
		qerID   = 11
		rateBps = 28_800
	)

	var (
		ueAIP = [4]byte{10, 0, 0, 9}
		ueBIP = [4]byte{10, 0, 0, 10}
		ueAv6 = [16]byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x09}
		ueBv6 = [16]byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0x0a}
		teidA = uint32(0x4C4C33)
		teidB = uint32(0x4C4C34)
	)

	prefixOf := func(addr [16]byte) netip.Addr {
		for i := 8; i < 16; i++ {
			addr[i] = 0
		}

		return netip.AddrFrom16(addr)
	}

	tests := []struct {
		name    string
		ulPdr   PdrInfo
		dlKey   netip.Addr
		inner   []byte
		l3Bytes int
	}{
		{
			name: "ipv4",
			ulPdr: PdrInfo{
				IMSI:         "001010000000001",
				Far:          FarInfo{Action: 0x02},
				UEIPv4:       netip.AddrFrom4(ueAIP),
				UEIPv6Prefix: canonicalUEv6Prefix,
			},
			dlKey:   netip.AddrFrom4(ueBIP),
			inner:   ipv4Packet(ueAIP, ueBIP, 17, udpDatagram(4000, 53, nil)),
			l3Bytes: 28,
		},
		{
			name: "ipv6",
			ulPdr: PdrInfo{
				IMSI:         "001010000000001",
				Far:          FarInfo{Action: 0x02},
				UEIPv4:       canonicalUEv4,
				UEIPv6Prefix: prefixOf(ueAv6),
			},
			dlKey:   prefixOf(ueBv6),
			inner:   ipv6Packet(ueAv6, ueBv6, 17, udpDatagram(4000, 53, nil)),
			l3Bytes: 48,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if len(tc.inner) != tc.l3Bytes {
				t.Fatalf("inner packet is %d bytes, want %d", len(tc.inner), tc.l3Bytes)
			}

			obj := loadProgramLocalSwitch(t)

			if err := obj.PutPdrUplink(teidA, tc.ulPdr); err != nil {
				t.Fatalf("install uplink PDR: %v", err)
			}

			dlPdr := ipv4OuterDownlinkPDR(teidB, testUPFN3IP, testGNBIP, 5)
			dlPdr.QerID = qerID
			dlPdr.Qer.MaxBitrateDL = rateBps

			if err := obj.PutPdrDownlink(tc.dlKey, dlPdr); err != nil {
				t.Fatalf("install downlink PDR: %v", err)
			}

			if action := runXDP(t, obj.UpfEntryFunc, uplinkGPDU(teidA, tc.inner)); action == ActionDrop {
				t.Fatalf("first packet was dropped, %d as qer_rate_limit", DropCount(obj, Uplink, "qer_rate_limit"))
			}

			action := runXDP(t, obj.UpfEntryFunc, uplinkGPDU(teidA, tc.inner))

			if got := DropCount(obj, Uplink, "qer_rate_limit"); action != ActionDrop || got != 1 {
				t.Fatalf("second packet got action %d with %d rate-limit drops, want ActionDrop (%d) with 1: %d L3 bytes at %d bps exceeds the window, but the L4 payload alone does not",
					action, got, ActionDrop, tc.l3Bytes, rateBps)
			}
		})
	}
}
