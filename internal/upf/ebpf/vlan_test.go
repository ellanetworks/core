// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package ebpf

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// TestVLANDownlinkInsertion checks that when an N3 VLAN is configured, the
// downlink encapsulation tags the egress frame: the Ethernet protocol becomes
// 802.1Q, the VLAN tag carries the configured ID and an IPv4 inner protocol, and
// the GTP-U/IPv4 packet plus inner payload follow.
func TestVLANDownlinkInsertion(t *testing.T) {
	requireProgTestRun(t)

	const (
		teid    = 0x564C414E
		qfi     = 2
		vlanID  = 100
		vlanLen = 4
	)

	obj := loadProgramVLAN(t, 1, 0, vlanID, 0)

	ueIP := [4]byte{10, 45, 0, 2}
	local := [4]byte{192, 168, 100, 1}
	remote := [4]byte{192, 168, 100, 9}

	putDownlinkPDR(t, obj, ueIP, teid, local, remote, qfi)

	inner := ipv4Packet([4]byte{8, 8, 8, 8}, ueIP, 17, udpDatagram(4000, 4001, nil))

	action, out := runXDPOut(t, obj.UpfN3N6EntrypointFunc, ethFrame(0x0800, inner))

	if action == XDP_ABORTED {
		t.Fatal("downlink packet got XDP_ABORTED; VLAN encapsulation failed")
	}

	if len(out) != ethHdrLen+vlanLen+gtpV4EncapLen+len(inner) {
		t.Fatalf("VLAN-tagged frame length = %d, want %d", len(out), ethHdrLen+vlanLen+gtpV4EncapLen+len(inner))
	}

	if et := binary.BigEndian.Uint16(out[12:14]); et != 0x8100 {
		t.Errorf("Ethernet protocol = %#04x, want 0x8100 (802.1Q)", et)
	}

	if tci := binary.BigEndian.Uint16(out[14:16]); tci&0x0fff != vlanID {
		t.Errorf("VLAN ID = %d, want %d", tci&0x0fff, vlanID)
	}

	if ep := binary.BigEndian.Uint16(out[16:18]); ep != 0x0800 {
		t.Errorf("VLAN encapsulated protocol = %#04x, want 0x0800 (IPv4)", ep)
	}

	if !bytes.Equal(out[ethHdrLen+vlanLen+gtpV4EncapLen:], inner) {
		t.Errorf("inner packet altered by encapsulation:\n got %x\nwant %x", out[ethHdrLen+vlanLen+gtpV4EncapLen:], inner)
	}
}
