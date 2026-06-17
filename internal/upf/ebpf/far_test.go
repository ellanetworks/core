// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package ebpf

import (
	"net/netip"
	"testing"
)

// TestFARDropUplink checks that an uplink PDR whose FAR action is DROP (no
// FORW bit) drops the packet, regardless of an otherwise-forwardable session.
func TestFARDropUplink(t *testing.T) {
	requireProgTestRun(t)

	const teid = 0x44524F50 // "DROP"

	obj := loadN3N6Program(t)

	pdr := PdrInfo{
		IMSI: "001010000000001",
		Far:  FarInfo{Action: 0x01 /* FAR_DROP */},
		Qer:  QerInfo{GateStatusUL: 0 /* open */, MaxBitrateUL: 0 /* unlimited */},
	}
	if err := obj.PutPdrUplink(teid, pdr); err != nil {
		t.Fatalf("install drop uplink PDR: %v", err)
	}

	action := runXDP(t, obj.UpfN3N6EntrypointFunc, uplinkGPDU(teid, innerIPv4UDP([4]byte{8, 8, 8, 8}, 53)))
	if action != XDP_DROP {
		t.Fatalf("uplink packet with FAR DROP got XDP action %d, want XDP_DROP (%d)", action, XDP_DROP)
	}
}

// TestFARDropDownlink checks that a downlink PDR whose FAR action is DROP drops
// the packet instead of encapsulating it.
func TestFARDropDownlink(t *testing.T) {
	requireProgTestRun(t)

	obj := loadProgram(t, 1, 0)

	dropUE := [4]byte{10, 45, 0, 2}

	pdr := ipv4OuterDownlinkPDR(0x1234, [4]byte{192, 168, 100, 1}, [4]byte{192, 168, 100, 9}, 5)
	pdr.Far.Action = 0x01 // FAR_DROP

	if err := obj.PutPdrDownlink(netip.AddrFrom4(dropUE), pdr); err != nil {
		t.Fatalf("install drop downlink PDR: %v", err)
	}

	inner := ipv4Packet([4]byte{8, 8, 8, 8}, dropUE, 17, udpDatagram(4000, 4001, nil))

	action := runXDP(t, obj.UpfN3N6EntrypointFunc, ethFrame(0x0800, inner))
	if action != XDP_DROP {
		t.Fatalf("downlink packet with FAR DROP got XDP action %d, want XDP_DROP (%d)", action, XDP_DROP)
	}
}

// TestFARDropDownlinkIPv6 checks the FAR DROP action on the IPv6 downlink path.
func TestFARDropDownlinkIPv6(t *testing.T) {
	requireProgTestRun(t)

	obj := loadProgram(t, 1, 0)

	pdr := ipv4OuterDownlinkPDR(0x1234, testUPFN3IP, testGNBIP, 5)
	pdr.Far.Action = 0x01 // FAR_DROP

	if err := obj.PutPdrDownlink(netip.MustParseAddr("2001:db8::"), pdr); err != nil {
		t.Fatalf("install drop downlink IPv6 PDR: %v", err)
	}

	serverV6 := [16]byte{0x20, 0x01, 0x48, 0x60, 0x48, 0x60, 0, 0, 0, 0, 0, 0, 0, 0, 0x88, 0x88}
	inner := ipv6Packet(serverV6, testUEv6, 17, udpDatagram(4000, 53, nil))

	action := runXDP(t, obj.UpfN3N6EntrypointFunc, ethFrame(0x86DD, inner))
	if action != XDP_DROP {
		t.Fatalf("downlink IPv6 packet with FAR DROP got XDP action %d, want XDP_DROP (%d)", action, XDP_DROP)
	}
}
