// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package ebpf

import (
	"net/netip"
	"testing"
)

// TestFlowReportUplink checks that, with flow accounting enabled, a forwarded
// uplink packet records a flow_stats entry with the expected key (IMSI,
// addresses, protocol, action, egress interface) and counts. flow_stats is a
// regular LRU hash, so it reads back after BPF_PROG_TEST_RUN.
func TestFlowReportUplink(t *testing.T) {
	requireProgTestRun(t)

	const teid = 0x464C4F57

	obj := loadProgramFlow(t, 0, 1)
	putForwardingUplinkPDR(t, obj, teid, 0)

	srcUE := [4]byte{10, 0, 0, 9} // innerIPv4UDP source
	dst := [4]byte{8, 8, 8, 8}

	runXDP(t, obj.UpfN3N6EntrypointFunc, uplinkGPDU(teid, innerIPv4UDP(dst, 53)))

	var (
		key N3N6EntrypointFlow
		val N3N6EntrypointFlowStats
	)

	it := obj.FlowStats.Iterate()
	if !it.Next(&key, &val) {
		t.Fatalf("no flow_stats entry recorded (iterate err=%v)", it.Err())
	}

	const wantIMSI = 1010000000001 // "001010000000001"

	if key.Imsi != wantIMSI {
		t.Errorf("flow IMSI = %d, want %d", key.Imsi, uint64(wantIMSI))
	}

	if key.Proto != 17 {
		t.Errorf("flow protocol = %d, want 17 (UDP)", key.Proto)
	}

	if key.Action != 0 {
		t.Errorf("flow action = %d, want 0 (ALLOW)", key.Action)
	}

	if key.Dscp != 0 {
		t.Errorf("flow DSCP = %d, want 0", key.Dscp)
	}

	if key.EgressIfindex != 1 {
		t.Errorf("flow egress ifindex = %d, want 1 (N6)", key.EgressIfindex)
	}

	if want := IPToIn6Addr(netip.AddrFrom4(srcUE)); key.Saddr.In6U.U6Addr8 != want {
		t.Errorf("flow saddr = %v, want %v", key.Saddr.In6U.U6Addr8, want)
	}

	if want := IPToIn6Addr(netip.AddrFrom4(dst)); key.Daddr.In6U.U6Addr8 != want {
		t.Errorf("flow daddr = %v, want %v", key.Daddr.In6U.U6Addr8, want)
	}

	if val.Packets != 1 {
		t.Errorf("flow packets = %d, want 1", val.Packets)
	}

	if val.Bytes == 0 {
		t.Error("flow bytes = 0, want > 0")
	}
}

// TestURRByteAccounting checks that a URR accumulates the forwarded byte count
// across packets. urr_map is a PERCPU_HASH, which reads back after a test-run.
func TestURRByteAccounting(t *testing.T) {
	requireProgTestRun(t)

	const (
		teid  = 0x55525202
		urrID = 9
	)

	obj := loadN3N6Program(t)
	if err := obj.NewUrr(urrID); err != nil {
		t.Fatalf("NewUrr: %v", err)
	}

	pdr := PdrInfo{
		IMSI:  "001010000000001",
		UrrID: urrID,
		Far:   FarInfo{Action: 0x02 /* FAR_FORW */},
		Qer:   QerInfo{GateStatusUL: 0, MaxBitrateUL: 0},
	}
	if err := obj.PutPdrUplink(teid, pdr); err != nil {
		t.Fatalf("install uplink PDR: %v", err)
	}

	inner := innerIPv4UDP([4]byte{8, 8, 8, 8}, 53)
	perPacket := uint64(ethHdrLen + len(inner)) // URR counts the decapsulated frame

	runXDP(t, obj.UpfN3N6EntrypointFunc, uplinkGPDU(teid, inner))

	if got := sumURR(t, obj, urrID); got != perPacket {
		t.Fatalf("URR after 1 packet = %d, want %d", got, perPacket)
	}

	runXDP(t, obj.UpfN3N6EntrypointFunc, uplinkGPDU(teid, inner))

	if got := sumURR(t, obj, urrID); got != 2*perPacket {
		t.Fatalf("URR after 2 packets = %d, want %d", got, 2*perPacket)
	}
}

// TestFlowReportDownlink is the downlink counterpart to TestFlowReportUplink: a
// forwarded downlink packet records a flow with the server as source, the UE as
// destination, and the N3 egress interface.
func TestFlowReportDownlink(t *testing.T) {
	requireProgTestRun(t)

	const (
		teid = 0x464C4F58
		qfi  = 5
	)

	obj := loadProgramFlow(t, 1, 0)
	putDownlinkPDR(t, obj, ueIP, teid, testUPFN3IP, testGNBIP, qfi)

	inner := ipv4Packet(serverIP, ueIP, 17, udpDatagram(4000, 53, nil))
	runXDP(t, obj.UpfN3N6EntrypointFunc, ethFrame(0x0800, inner))

	var (
		key N3N6EntrypointFlow
		val N3N6EntrypointFlowStats
	)

	it := obj.FlowStats.Iterate()
	if !it.Next(&key, &val) {
		t.Fatalf("no flow_stats entry recorded (iterate err=%v)", it.Err())
	}

	if key.Proto != 17 {
		t.Errorf("flow protocol = %d, want 17 (UDP)", key.Proto)
	}

	if key.Action != 0 {
		t.Errorf("flow action = %d, want 0 (ALLOW)", key.Action)
	}

	if key.EgressIfindex != 1 {
		t.Errorf("flow egress ifindex = %d, want 1 (N3)", key.EgressIfindex)
	}

	if want := IPToIn6Addr(netip.AddrFrom4(serverIP)); key.Saddr.In6U.U6Addr8 != want {
		t.Errorf("flow saddr = %v, want %v (server)", key.Saddr.In6U.U6Addr8, want)
	}

	if want := IPToIn6Addr(netip.AddrFrom4(ueIP)); key.Daddr.In6U.U6Addr8 != want {
		t.Errorf("flow daddr = %v, want %v (UE)", key.Daddr.In6U.U6Addr8, want)
	}

	if val.Packets != 1 {
		t.Errorf("flow packets = %d, want 1", val.Packets)
	}
}

// TestURRByteAccountingDownlink checks that a downlink URR accumulates the
// forwarded byte count.
func TestURRByteAccountingDownlink(t *testing.T) {
	requireProgTestRun(t)

	const (
		teid  = 0x55525203
		urrID = 11
		qfi   = 5
	)

	obj := loadProgram(t, 1, 0)
	if err := obj.NewUrr(urrID); err != nil {
		t.Fatalf("NewUrr: %v", err)
	}

	pdr := ipv4OuterDownlinkPDR(teid, testUPFN3IP, testGNBIP, qfi)
	pdr.UrrID = urrID

	if err := obj.PutPdrDownlink(netip.AddrFrom4(ueIP), pdr); err != nil {
		t.Fatalf("install downlink PDR: %v", err)
	}

	inner := ipv4Packet(serverIP, ueIP, 17, udpDatagram(4000, 53, nil))
	frame := ethFrame(0x0800, inner)
	perPacket := uint64(len(frame)) // URR counts the pre-encapsulation frame

	runXDP(t, obj.UpfN3N6EntrypointFunc, frame)

	if got := sumURR(t, obj, urrID); got != perPacket {
		t.Fatalf("URR after 1 packet = %d, want %d", got, perPacket)
	}

	runXDP(t, obj.UpfN3N6EntrypointFunc, frame)

	if got := sumURR(t, obj, urrID); got != 2*perPacket {
		t.Fatalf("URR after 2 packets = %d, want %d", got, 2*perPacket)
	}
}

func sumURR(t *testing.T, obj *BpfObjects, urrID uint32) uint64 {
	t.Helper()

	var perCPU []uint64
	if err := obj.UrrMap.Lookup(urrID, &perCPU); err != nil {
		t.Fatalf("urr_map lookup: %v", err)
	}

	var sum uint64
	for _, v := range perCPU {
		sum += v
	}

	return sum
}
