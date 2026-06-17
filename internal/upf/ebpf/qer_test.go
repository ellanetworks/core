// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package ebpf

import (
	"net/netip"
	"testing"
)

// QER gate enforcement. A non-open gate must drop the packet before it is
// forwarded. The QFI marking is asserted by the encapsulation tests, and rate
// limiting is timing/state dependent (deferred to the netns harness).

// TestQERGateUplinkClosed checks that a closed uplink QER gate drops the packet.
func TestQERGateUplinkClosed(t *testing.T) {
	requireProgTestRun(t)

	const teid = 0x51455201

	obj := loadN3N6Program(t)

	pdr := PdrInfo{
		IMSI: "001010000000001",
		Far:  FarInfo{Action: 0x02 /* FAR_FORW */},
		Qer:  QerInfo{GateStatusUL: 1 /* GATE_STATUS_CLOSED */, MaxBitrateUL: 0},
	}
	if err := obj.PutPdrUplink(teid, pdr); err != nil {
		t.Fatalf("install uplink PDR: %v", err)
	}

	action := runXDP(t, obj.UpfN3N6EntrypointFunc, uplinkGPDU(teid, innerIPv4UDP([4]byte{8, 8, 8, 8}, 53)))

	if action != XDP_DROP {
		t.Fatalf("closed uplink gate: got XDP action %d, want XDP_DROP (%d)", action, XDP_DROP)
	}
}

// TestQERGateDownlinkClosed checks that a closed downlink QER gate drops the
// packet.
func TestQERGateDownlinkClosed(t *testing.T) {
	requireProgTestRun(t)

	const (
		teid = 0x51455202
		qfi  = 4
	)

	obj := loadProgram(t, 1, 0)

	ueIP := [4]byte{10, 45, 0, 2}

	pdr := ipv4OuterDownlinkPDR(teid, [4]byte{192, 168, 100, 1}, [4]byte{192, 168, 100, 9}, qfi)

	pdr.Qer.GateStatusDL = 1 // GATE_STATUS_CLOSED
	if err := obj.PutPdrDownlink(netip.AddrFrom4(ueIP), pdr); err != nil {
		t.Fatalf("install downlink PDR: %v", err)
	}

	inner := ipv4Packet([4]byte{8, 8, 8, 8}, ueIP, 17, udpDatagram(4000, 4001, nil))

	action := runXDP(t, obj.UpfN3N6EntrypointFunc, ethFrame(0x0800, inner))

	if action != XDP_DROP {
		t.Fatalf("closed downlink gate: got XDP action %d, want XDP_DROP (%d)", action, XDP_DROP)
	}
}

// TestQERGateDownlinkClosedIPv6 checks that a closed downlink gate drops an IPv6
// packet on the separate IPv6 downlink code path.
func TestQERGateDownlinkClosedIPv6(t *testing.T) {
	requireProgTestRun(t)

	const (
		teid = 0x51455203
		qfi  = 4
	)

	obj := loadProgram(t, 1, 0)

	pdr := ipv4OuterDownlinkPDR(teid, testUPFN3IP, testGNBIP, qfi)
	pdr.Qer.GateStatusDL = 1 // GATE_STATUS_CLOSED

	if err := obj.PutPdrDownlink(netip.MustParseAddr("2001:db8::"), pdr); err != nil {
		t.Fatalf("install downlink IPv6 PDR: %v", err)
	}

	serverV6 := [16]byte{0x20, 0x01, 0x48, 0x60, 0x48, 0x60, 0, 0, 0, 0, 0, 0, 0, 0, 0x88, 0x88}
	inner := ipv6Packet(serverV6, testUEv6, 17, udpDatagram(4000, 53, nil))

	action := runXDP(t, obj.UpfN3N6EntrypointFunc, ethFrame(0x86DD, inner))

	if action != XDP_DROP {
		t.Fatalf("closed downlink gate (IPv6): got XDP action %d, want XDP_DROP (%d)", action, XDP_DROP)
	}
}
