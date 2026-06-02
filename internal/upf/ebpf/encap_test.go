// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package ebpf

import (
	"bytes"
	"net/netip"
	"testing"
)

// TestGTPEncapsulationDownlinkIPv4 checks that a downlink IPv4 packet for a UE
// is encapsulated into a GTP-U/UDP/IPv4 G-PDU: the outer header carries the
// FAR's local/remote addresses and TEID, the PDU session container carries the
// QER's QFI, the outer IPv4 checksum is valid, and the inner packet is preserved
// byte for byte. The final action depends on the host routing table, so the
// assertion is on the output packet.
func TestGTPEncapsulationDownlinkIPv4(t *testing.T) {
	requireProgTestRun(t)

	// n3_ifindex 1 (loopback) is the encapsulation egress and MTU-check device. A
	// non-GTP IPv4 packet is routed to handle_n6_packet (downlink encap) by
	// handle_ip4 regardless of the entrypoint's N3/N6 tag.
	obj := loadProgram(t, 1, 0)

	var (
		ueIP   = [4]byte{10, 45, 0, 2}
		local  = [4]byte{192, 168, 100, 1}
		remote = [4]byte{192, 168, 100, 9}
	)

	const (
		teid = 0x55667788
		qfi  = 9
	)

	putDownlinkPDR(t, obj, ueIP, teid, local, remote, qfi)

	inner := ipv4Packet([4]byte{8, 8, 8, 8}, ueIP, 17, udpDatagram(4000, 4001, []byte{0xde, 0xad, 0xbe, 0xef}))

	action, out := runXDPOut(t, obj.UpfN3N6EntrypointFunc, ethFrame(0x0800, inner))

	if action == XDP_ABORTED {
		t.Fatal("downlink packet got XDP_ABORTED; encapsulation failed")
	}

	if len(out) != ethHdrLen+gtpV4EncapLen+len(inner) {
		t.Fatalf("encapsulated frame length = %d, want %d", len(out), ethHdrLen+gtpV4EncapLen+len(inner))
	}

	f := parseGTPv4Frame(t, out)

	if !f.outerChecksumOK {
		t.Error("outer IPv4 header checksum is invalid")
	}

	if f.outerProto != 17 {
		t.Errorf("outer IP protocol = %d, want 17 (UDP)", f.outerProto)
	}

	if f.outerSrc != local {
		t.Errorf("outer src IP = %v, want %v (FAR localip)", f.outerSrc, local)
	}

	if f.outerDst != remote {
		t.Errorf("outer dst IP = %v, want %v (FAR remoteip)", f.outerDst, remote)
	}

	if f.udpDstPort != GTPUDPPort {
		t.Errorf("outer UDP dst port = %d, want %d", f.udpDstPort, GTPUDPPort)
	}

	if f.gtpFlags&0x04 == 0 {
		t.Errorf("GTP E flag not set (flags = %#02x)", f.gtpFlags)
	}

	if f.gtpMsgType != 0xFF {
		t.Errorf("GTP message type = %#02x, want 0xFF (G-PDU)", f.gtpMsgType)
	}

	if f.teid != teid {
		t.Errorf("GTP TEID = %#x, want %#x", f.teid, uint32(teid))
	}

	if f.qfi != qfi {
		t.Errorf("PDU session container QFI = %d, want %d", f.qfi, qfi)
	}

	if !bytes.Equal(f.inner, inner) {
		t.Errorf("inner packet altered by encapsulation:\n got %x\nwant %x", f.inner, inner)
	}
}

// TestGTPEncapsulationDownlinkInnerIPv6 checks that a downlink IPv6 packet for a
// UE (matched by its /64 prefix) is encapsulated into a GTP-U/UDP/IPv4 G-PDU
// carrying the inner IPv6 packet unchanged.
func TestGTPEncapsulationDownlinkInnerIPv6(t *testing.T) {
	requireProgTestRun(t)

	obj := loadProgram(t, 1, 0)

	uePrefix := netip.MustParseAddr("2001:db8:1::")
	ue := netip.MustParseAddr("2001:db8:1::2").As16()
	server := netip.MustParseAddr("2001:4860:4860::8888").As16()

	local := [4]byte{192, 168, 100, 1}
	remote := [4]byte{192, 168, 100, 9}

	const (
		teid = 0x0A0B0C0D
		qfi  = 5
	)

	putDownlinkPDRv6UE(t, obj, uePrefix, teid, local, remote, qfi)

	inner := ipv6Packet(server, ue, 17, udpDatagram(4000, 4001, []byte{0xde, 0xad}))

	action, out := runXDPOut(t, obj.UpfN3N6EntrypointFunc, ethFrame(0x86DD, inner))

	if action == XDP_ABORTED {
		t.Fatal("downlink IPv6 packet got XDP_ABORTED; encapsulation failed")
	}

	if len(out) != ethHdrLen+gtpV4EncapLen+len(inner) {
		t.Fatalf("encapsulated frame length = %d, want %d", len(out), ethHdrLen+gtpV4EncapLen+len(inner))
	}

	f := parseGTPv4Frame(t, out)

	if !f.outerChecksumOK {
		t.Error("outer IPv4 header checksum is invalid")
	}

	if f.outerSrc != local || f.outerDst != remote {
		t.Errorf("outer IPs = %v -> %v, want %v -> %v", f.outerSrc, f.outerDst, local, remote)
	}

	if f.teid != teid {
		t.Errorf("GTP TEID = %#x, want %#x", f.teid, uint32(teid))
	}

	if f.qfi != qfi {
		t.Errorf("PDU session container QFI = %d, want %d", f.qfi, qfi)
	}

	if !bytes.Equal(f.inner, inner) {
		t.Errorf("inner IPv6 packet altered by encapsulation:\n got %x\nwant %x", f.inner, inner)
	}
}

// TestGTPEncapsulationDownlinkIPv6Transport checks that a downlink IPv4 packet
// for a UE is encapsulated into a GTP-U over IPv6 transport: outer IPv6
// src/dst from the FAR, TEID, QFI, the mandatory outer UDP checksum (RFC 6936)
// valid, and the inner packet preserved.
func TestGTPEncapsulationDownlinkIPv6Transport(t *testing.T) {
	requireProgTestRun(t)

	obj := loadProgram(t, 1, 0)

	ueIP := [4]byte{10, 45, 0, 2}
	local := netip.MustParseAddr("2001:db8:33::1").As16()
	remote := netip.MustParseAddr("2001:db8:33::9").As16()

	const (
		teid = 0x77778888
		qfi  = 7
	)

	putDownlinkPDRv6Outer(t, obj, ueIP, teid, local, remote, qfi)

	inner := ipv4Packet([4]byte{8, 8, 8, 8}, ueIP, 17, udpDatagram(4000, 4001, []byte{0x01, 0x02, 0x03, 0x04}))

	action, out := runXDPOut(t, obj.UpfN3N6EntrypointFunc, ethFrame(0x0800, inner))

	if action == XDP_ABORTED {
		t.Fatal("downlink packet got XDP_ABORTED; IPv6-transport encapsulation failed")
	}

	if len(out) != ethHdrLen+gtpV6EncapLen+len(inner) {
		t.Fatalf("encapsulated frame length = %d, want %d", len(out), ethHdrLen+gtpV6EncapLen+len(inner))
	}

	f := parseGTPv6Frame(t, out)

	if f.outerNextHdr != 17 {
		t.Errorf("outer IPv6 next header = %d, want 17 (UDP)", f.outerNextHdr)
	}

	if f.outerSrc != local || f.outerDst != remote {
		t.Errorf("outer IPs = %x -> %x, want %x -> %x", f.outerSrc, f.outerDst, local, remote)
	}

	if !f.udpChecksumOK {
		t.Error("outer UDP-over-IPv6 checksum is invalid")
	}

	if f.udpDstPort != GTPUDPPort {
		t.Errorf("outer UDP dst port = %d, want %d", f.udpDstPort, GTPUDPPort)
	}

	if f.gtpMsgType != 0xFF {
		t.Errorf("GTP message type = %#02x, want 0xFF (G-PDU)", f.gtpMsgType)
	}

	if f.teid != teid {
		t.Errorf("GTP TEID = %#x, want %#x", f.teid, uint32(teid))
	}

	if f.qfi != qfi {
		t.Errorf("PDU session container QFI = %d, want %d", f.qfi, qfi)
	}

	if !bytes.Equal(f.inner, inner) {
		t.Errorf("inner packet altered by encapsulation:\n got %x\nwant %x", f.inner, inner)
	}
}

// TestTransportLevelMarking checks that a FAR's transport-level marking is
// written to the outer IPv4 TOS byte of the encapsulated downlink packet.
func TestTransportLevelMarking(t *testing.T) {
	requireProgTestRun(t)

	const (
		teid    = 0x544F5301
		qfi     = 5
		wantTOS = 0xB8 // DSCP EF
	)

	obj := loadProgram(t, 1, 0)

	pdr := ipv4OuterDownlinkPDR(teid, testUPFN3IP, testGNBIP, qfi)
	pdr.Far.TransportLevelMarking = uint16(wantTOS) << 8

	if err := obj.PutPdrDownlink(netip.AddrFrom4(ueIP), pdr); err != nil {
		t.Fatalf("install downlink PDR: %v", err)
	}

	inner := ipv4Packet(serverIP, ueIP, 17, udpDatagram(4000, 53, nil))

	action, out := runXDPOut(t, obj.UpfN3N6EntrypointFunc, ethFrame(0x0800, inner))
	if action == XDP_ABORTED {
		t.Fatal("downlink packet got XDP_ABORTED")
	}

	if tos := out[ethHdrLen+1]; tos != wantTOS {
		t.Errorf("outer IPv4 TOS = %#02x, want %#02x (FAR transport-level marking)", tos, wantTOS)
	}

	if f := parseGTPv4Frame(t, out); !f.outerChecksumOK {
		t.Error("outer IPv4 header checksum invalid after marking")
	}
}

// ipv4OuterDownlinkPDR builds a downlink PDR that forwards and encapsulates into
// a GTP-U/IPv4 tunnel toward remote with the given TEID and QFI.
func ipv4OuterDownlinkPDR(teid uint32, local, remote [4]byte, qfi uint8) PdrInfo {
	return PdrInfo{
		IMSI: "001010000000001", // non-numeric IMSI zeroes the FAR
		Far: FarInfo{
			Action:              0x02, // FAR_FORW
			OuterHeaderCreation: 0x01, // OHC_GTP_U_UDP_IPv4
			TeID:                teid,
			LocalIP:             IPToIn6Addr(netip.AddrFrom4(local)),
			RemoteIP:            IPToIn6Addr(netip.AddrFrom4(remote)),
		},
		Qer: QerInfo{GateStatusDL: 0 /* GATE_STATUS_OPEN */, Qfi: qfi, MaxBitrateDL: 0 /* unlimited */},
	}
}

// putDownlinkPDR installs a downlink PDR keyed by an IPv4 UE address.
func putDownlinkPDR(t *testing.T, obj *BpfObjects, ueIP [4]byte, teid uint32, local, remote [4]byte, qfi uint8) {
	t.Helper()

	if err := obj.PutPdrDownlink(netip.AddrFrom4(ueIP), ipv4OuterDownlinkPDR(teid, local, remote, qfi)); err != nil {
		t.Fatalf("install downlink PDR: %v", err)
	}
}

// putDownlinkPDRFiltered installs a downlink PDR (IPv4 UE) that applies the SDF
// filter at filterIndex.
func putDownlinkPDRFiltered(t *testing.T, obj *BpfObjects, ueIP [4]byte, teid uint32, local, remote [4]byte, qfi uint8, filterIndex uint32) {
	t.Helper()

	pdr := ipv4OuterDownlinkPDR(teid, local, remote, qfi)
	pdr.FilterMapIndex = filterIndex

	if err := obj.PutPdrDownlink(netip.AddrFrom4(ueIP), pdr); err != nil {
		t.Fatalf("install filtered downlink PDR: %v", err)
	}
}

// putDownlinkPDRv6UE installs a downlink PDR keyed by a UE IPv6 /64 prefix.
func putDownlinkPDRv6UE(t *testing.T, obj *BpfObjects, uePrefix netip.Addr, teid uint32, local, remote [4]byte, qfi uint8) {
	t.Helper()

	if err := obj.PutPdrDownlink(uePrefix, ipv4OuterDownlinkPDR(teid, local, remote, qfi)); err != nil {
		t.Fatalf("install downlink IPv6 PDR: %v", err)
	}
}

// ipv6OuterDownlinkPDR builds a downlink PDR that forwards and encapsulates into
// a GTP-U over IPv6 tunnel toward remote with the given TEID and QFI.
func ipv6OuterDownlinkPDR(teid uint32, local, remote [16]byte, qfi uint8) PdrInfo {
	return PdrInfo{
		IMSI: "001010000000001",
		Far: FarInfo{
			Action:              0x02, // FAR_FORW
			OuterHeaderCreation: 0x02, // OHC_GTP_U_UDP_IPv6
			TeID:                teid,
			LocalIP:             local,
			RemoteIP:            remote,
		},
		Qer: QerInfo{GateStatusDL: 0 /* GATE_STATUS_OPEN */, Qfi: qfi, MaxBitrateDL: 0 /* unlimited */},
	}
}

// putDownlinkPDRv6Outer installs a downlink PDR (IPv4 UE) that encapsulates into
// an IPv6 transport.
func putDownlinkPDRv6Outer(t *testing.T, obj *BpfObjects, ueIP [4]byte, teid uint32, local, remote [16]byte, qfi uint8) {
	t.Helper()

	if err := obj.PutPdrDownlink(netip.AddrFrom4(ueIP), ipv6OuterDownlinkPDR(teid, local, remote, qfi)); err != nil {
		t.Fatalf("install downlink IPv6-transport PDR: %v", err)
	}
}

// putForwardingUplinkPDRGTP installs an uplink PDR keyed by lookupTEID that
// re-encapsulates (GTP-to-GTP) toward remote with outerTEID instead of
// decapsulating.
func putForwardingUplinkPDRGTP(t *testing.T, obj *BpfObjects, lookupTEID uint32, local, remote [4]byte, outerTEID uint32) {
	t.Helper()

	pdr := PdrInfo{
		IMSI: "001010000000001",
		Far: FarInfo{
			Action:              0x02, // FAR_FORW
			OuterHeaderCreation: 0x01, // OHC_GTP_U_UDP_IPv4
			TeID:                outerTEID,
			LocalIP:             IPToIn6Addr(netip.AddrFrom4(local)),
			RemoteIP:            IPToIn6Addr(netip.AddrFrom4(remote)),
		},
		Qer: QerInfo{GateStatusUL: 0 /* GATE_STATUS_OPEN */, MaxBitrateUL: 0 /* unlimited */},
	}
	if err := obj.PutPdrUplink(lookupTEID, pdr); err != nil {
		t.Fatalf("install uplink GTP-forward PDR: %v", err)
	}
}
