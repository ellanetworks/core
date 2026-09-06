// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package ebpf

import (
	"net/netip"
	"testing"
)

func TestFlowReportUplink(t *testing.T) {
	requireProgTestRun(t)

	const teid = 0x464C4F57

	obj := loadProgramFlow(t, 0, 1)
	putForwardingUplinkPDR(t, obj, teid, 0)

	srcUE := [4]byte{10, 0, 0, 9}
	dst := [4]byte{8, 8, 8, 8}

	runXDP(t, obj.UpfEntryFunc, uplinkGPDU(teid, innerIPv4UDP(dst, 53)))

	var (
		key N3N6EntrypointFlow
		val N3N6EntrypointFlowStats
	)

	it := obj.FlowStats.Iterate()
	if !it.Next(&key, &val) {
		t.Fatalf("no flow_stats entry recorded (iterate err=%v)", it.Err())
	}

	const wantIMSI = "001010000000001"

	if got := DecodeIMSITag(key.Imsi); got != wantIMSI {
		t.Errorf("flow IMSI = %q, want %q", got, wantIMSI)
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

	if key.Direction != FlowDirectionUplink {
		t.Errorf("flow direction = %d, want %d (uplink)", key.Direction, FlowDirectionUplink)
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

func TestURRByteAccounting(t *testing.T) {
	requireProgTestRun(t)

	const (
		teid  = 0x55525202
		urrID = 9
		seid  = 0x55525202
	)

	obj := loadN3N6Program(t)
	if err := obj.NewUrr(seid, urrID); err != nil {
		t.Fatalf("NewUrr: %v", err)
	}

	pdr := PdrInfo{
		SEID:         seid,
		IMSI:         "001010000000001",
		UrrID:        urrID,
		Far:          FarInfo{Action: 0x02},
		Qer:          QerInfo{GateStatusUL: 0, MaxBitrateUL: 0},
		UEIPv4:       netip.AddrFrom4([4]byte{10, 0, 0, 9}),
		UEIPv6Prefix: netip.MustParseAddr("2001:db8::"),
	}
	if err := obj.PutPdrUplink(teid, pdr); err != nil {
		t.Fatalf("install uplink PDR: %v", err)
	}

	inner := innerIPv4UDP([4]byte{8, 8, 8, 8}, 53)
	perPacket := uint64(ethHdrLen + len(inner))

	runXDP(t, obj.UpfEntryFunc, uplinkGPDU(teid, inner))

	if got := sumURR(t, obj, seid, urrID); got != perPacket {
		t.Fatalf("URR after 1 packet = %d, want %d", got, perPacket)
	}

	runXDP(t, obj.UpfEntryFunc, uplinkGPDU(teid, inner))

	if got := sumURR(t, obj, seid, urrID); got != 2*perPacket {
		t.Fatalf("URR after 2 packets = %d, want %d", got, 2*perPacket)
	}
}

func TestFlowReportDownlink(t *testing.T) {
	requireProgTestRun(t)

	const (
		teid = 0x464C4F58
		qfi  = 5
	)

	obj := loadProgramFlow(t, 1, 0)
	putDownlinkPDR(t, obj, ueIP, teid, testUPFN3IP, testGNBIP, qfi)

	inner := ipv4Packet(serverIP, ueIP, 17, udpDatagram(4000, 53, nil))
	runXDP(t, obj.UpfEntryFunc, ethFrame(0x0800, inner))

	var (
		key N3N6EntrypointFlow
		val N3N6EntrypointFlowStats
	)

	it := obj.FlowStats.Iterate()
	if !it.Next(&key, &val) {
		t.Fatalf("no flow_stats entry recorded (iterate err=%v)", it.Err())
	}

	const wantIMSI = "001010000000001"

	if got := DecodeIMSITag(key.Imsi); got != wantIMSI {
		t.Errorf("flow IMSI = %q, want %q", got, wantIMSI)
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

	if key.Direction != FlowDirectionDownlink {
		t.Errorf("flow direction = %d, want %d (downlink)", key.Direction, FlowDirectionDownlink)
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

	if val.Bytes == 0 {
		t.Error("flow bytes = 0, want > 0")
	}
}

func TestURRByteAccountingDownlink(t *testing.T) {
	requireProgTestRun(t)

	const (
		teid  = 0x55525203
		urrID = 11
		qfi   = 5
		seid  = 0x55525203
	)

	obj := loadProgram(t, 1, 0)
	if err := obj.NewUrr(seid, urrID); err != nil {
		t.Fatalf("NewUrr: %v", err)
	}

	pdr := ipv4OuterDownlinkPDR(teid, testUPFN3IP, testGNBIP, qfi)
	pdr.UrrID = urrID
	pdr.SEID = seid

	if err := obj.PutPdrDownlink(netip.AddrFrom4(ueIP), pdr); err != nil {
		t.Fatalf("install downlink PDR: %v", err)
	}

	inner := ipv4Packet(serverIP, ueIP, 17, udpDatagram(4000, 53, nil))
	frame := ethFrame(0x0800, inner)
	perPacket := uint64(len(frame))

	runXDP(t, obj.UpfEntryFunc, frame)

	if got := sumURR(t, obj, seid, urrID); got != perPacket {
		t.Fatalf("URR after 1 packet = %d, want %d", got, perPacket)
	}

	runXDP(t, obj.UpfEntryFunc, frame)

	if got := sumURR(t, obj, seid, urrID); got != 2*perPacket {
		t.Fatalf("URR after 2 packets = %d, want %d", got, 2*perPacket)
	}
}

func TestURRAddRestore(t *testing.T) {
	requireProgTestRun(t)

	const (
		seid  = 0x55525204
		urrID = 13
	)

	obj := loadN3N6Program(t)
	if err := obj.NewUrr(seid, urrID); err != nil {
		t.Fatalf("NewUrr: %v", err)
	}

	if err := obj.AddUrr(seid, urrID, 500); err != nil {
		t.Fatalf("AddUrr: %v", err)
	}

	if got := sumURR(t, obj, seid, urrID); got != 500 {
		t.Fatalf("URR after AddUrr(500) = %d, want 500", got)
	}

	drained, err := obj.GetAndResetUrr(seid, urrID)
	if err != nil {
		t.Fatalf("GetAndResetUrr: %v", err)
	}

	if drained != 500 {
		t.Fatalf("drained = %d, want 500", drained)
	}

	if got := sumURR(t, obj, seid, urrID); got != 0 {
		t.Fatalf("URR after reset = %d, want 0", got)
	}

	if err := obj.AddUrr(seid, urrID, drained); err != nil {
		t.Fatalf("AddUrr restore: %v", err)
	}

	if got := sumURR(t, obj, seid, urrID); got != 500 {
		t.Fatalf("URR after restore = %d, want 500", got)
	}
}

func sumURR(t *testing.T, obj *BpfObjects, seid uint64, urrID uint32) uint64 {
	t.Helper()

	var perCPU []uint64
	if err := obj.UrrMap.Lookup(N3N6EntrypointUrrKey{Seid: seid, UrrId: urrID}, &perCPU); err != nil {
		t.Fatalf("urr_map lookup: %v", err)
	}

	var sum uint64
	for _, v := range perCPU {
		sum += v
	}

	return sum
}

func TestFlowDSCP(t *testing.T) {
	requireProgTestRun(t)

	tests := []struct {
		name     string
		teid     uint32
		inner    []byte
		wantDSCP uint8
	}{
		{
			name:     "ipv4 ef",
			teid:     0x44534301,
			inner:    withTOS(innerIPv4UDP([4]byte{8, 8, 8, 8}, 53), 0xB8),
			wantDSCP: 46,
		},
		{
			name:     "ipv4 cs3",
			teid:     0x44534302,
			inner:    withTOS(innerIPv4UDP([4]byte{8, 8, 8, 8}, 53), 0x60),
			wantDSCP: 24,
		},
		{
			name:     "ipv6 ef",
			teid:     0x44534303,
			inner:    withTrafficClass(innerIPv6UDP(testUEv6, 53), 0xB8),
			wantDSCP: 46,
		},
		{
			name:     "ipv6 cs3",
			teid:     0x44534304,
			inner:    withTrafficClass(innerIPv6UDP(testUEv6, 53), 0x60),
			wantDSCP: 24,
		},
		{
			name:     "ipv6 best effort",
			teid:     0x44534305,
			inner:    innerIPv6UDP(testUEv6, 53),
			wantDSCP: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			obj := loadProgramFlow(t, 0, 1)
			putForwardingUplinkPDR(t, obj, tc.teid, 0)

			runXDP(t, obj.UpfEntryFunc, uplinkGPDU(tc.teid, tc.inner))

			var (
				key N3N6EntrypointFlow
				val N3N6EntrypointFlowStats
			)

			it := obj.FlowStats.Iterate()
			if !it.Next(&key, &val) {
				t.Fatalf("no flow_stats entry recorded (iterate err=%v)", it.Err())
			}

			if key.Dscp != tc.wantDSCP {
				t.Errorf("flow DSCP = %d, want %d", key.Dscp, tc.wantDSCP)
			}
		})
	}
}
