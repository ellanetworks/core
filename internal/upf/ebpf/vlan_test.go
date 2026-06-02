// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package ebpf

import (
	"bytes"
	"encoding/binary"
	"net/netip"
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

// TestVLANDownlinkInsertionInnerIPv6 checks N3 VLAN insertion when the inner
// packet is IPv6.
func TestVLANDownlinkInsertionInnerIPv6(t *testing.T) {
	requireProgTestRun(t)

	const (
		teid    = 0x564C4136
		qfi     = 2
		vlanID  = 100
		vlanLen = 4
	)

	obj := loadProgramVLAN(t, 1, 0, vlanID, 0)

	server := [16]byte{0x20, 0x01, 0x48, 0x60, 0x48, 0x60, 0, 0, 0, 0, 0, 0, 0, 0, 0x88, 0x88}

	pdr := ipv4OuterDownlinkPDR(teid, testUPFN3IP, testGNBIP, qfi)
	if err := obj.PutPdrDownlink(netip.MustParseAddr("2001:db8::"), pdr); err != nil {
		t.Fatalf("install downlink IPv6 PDR: %v", err)
	}

	inner := ipv6Packet(server, testUEv6, 17, udpDatagram(4000, 53, nil))

	action, out := runXDPOut(t, obj.UpfN3N6EntrypointFunc, ethFrame(0x86DD, inner))
	if action == XDP_ABORTED {
		t.Fatal("downlink IPv6 packet got XDP_ABORTED; VLAN encapsulation failed")
	}

	if len(out) != ethHdrLen+vlanLen+gtpV4EncapLen+len(inner) {
		t.Fatalf("VLAN-tagged frame length = %d, want %d", len(out), ethHdrLen+vlanLen+gtpV4EncapLen+len(inner))
	}

	if et := binary.BigEndian.Uint16(out[12:14]); et != 0x8100 {
		t.Errorf("Ethernet protocol = %#04x, want 0x8100 (802.1Q)", et)
	}

	if !bytes.Equal(out[ethHdrLen+vlanLen+gtpV4EncapLen:], inner) {
		t.Errorf("inner IPv6 packet altered by encapsulation:\n got %x\nwant %x", out[ethHdrLen+vlanLen+gtpV4EncapLen:], inner)
	}
}

// TestVLANUplinkStrip checks that a VLAN-tagged uplink GTP-U frame is parsed and
// decapsulated to its inner packet (the ingress VLAN tag is stripped).
func TestVLANUplinkStrip(t *testing.T) {
	requireProgTestRun(t)

	const (
		teid   = 0x564C4153
		vlanID = 100
	)

	obj := loadN3N6Program(t)
	putForwardingUplinkPDR(t, obj, teid, 0)

	inner := innerIPv4UDP([4]byte{8, 8, 8, 8}, 53)
	gtp := ipv4Packet(testGNBIP, testUPFN3IP, 17, udpDatagram(GTPUDPPort, GTPUDPPort, gtpHeader(teid, inner)))

	action, out := runXDPOut(t, obj.UpfN3N6EntrypointFunc, vlanFrame(vlanID, 0x0800, gtp))
	if action == XDP_DROP || action == XDP_ABORTED {
		t.Fatalf("VLAN-tagged uplink got XDP action %d, want a forwarding action", action)
	}

	if len(out) != ethHdrLen+len(inner) || !bytes.Equal(out[ethHdrLen:], inner) {
		t.Fatalf("VLAN-tagged uplink not decapsulated to its inner packet:\n got %x\nwant %x", out, inner)
	}
}

// TestVLANUplinkN6Insertion checks that when an N6 VLAN is configured, the
// decapsulated uplink packet egresses with the N6 VLAN tag inserted.
func TestVLANUplinkN6Insertion(t *testing.T) {
	requireProgTestRun(t)

	const (
		teid    = 0x564C4136 + 1
		n6VLAN  = 200
		vlanLen = 4
	)

	obj := loadProgramVLAN(t, 0, 1, 0, n6VLAN)
	putForwardingUplinkPDR(t, obj, teid, 0)

	inner := innerIPv4UDP([4]byte{8, 8, 8, 8}, 53)

	action, out := runXDPOut(t, obj.UpfN3N6EntrypointFunc, uplinkGPDU(teid, inner))
	if action == XDP_DROP || action == XDP_ABORTED {
		t.Fatalf("uplink got XDP action %d, want a forwarding action", action)
	}

	if len(out) != ethHdrLen+vlanLen+len(inner) {
		t.Fatalf("decapsulated frame length = %d, want %d (with N6 VLAN)", len(out), ethHdrLen+vlanLen+len(inner))
	}

	if et := binary.BigEndian.Uint16(out[12:14]); et != 0x8100 {
		t.Errorf("Ethernet protocol = %#04x, want 0x8100 (802.1Q)", et)
	}

	if tci := binary.BigEndian.Uint16(out[14:16]); tci&0x0fff != n6VLAN {
		t.Errorf("VLAN ID = %d, want %d", tci&0x0fff, n6VLAN)
	}

	if !bytes.Equal(out[ethHdrLen+vlanLen:], inner) {
		t.Errorf("inner packet altered by decapsulation:\n got %x\nwant %x", out[ethHdrLen+vlanLen:], inner)
	}
}
