// Copyright 2026 Ella Networks
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
