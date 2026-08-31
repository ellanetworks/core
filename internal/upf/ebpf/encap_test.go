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

const gtpV4EncapLenS1U = 36

func putDownlinkPDRS1U(t *testing.T, obj *BpfObjects, ueIP [4]byte, teid uint32, local, remote [4]byte) {
	t.Helper()

	pdr := ipv4OuterDownlinkPDR(teid, local, remote, 0)
	pdr.Far.OuterHeaderCreation |= 0x10

	if err := obj.PutPdrDownlink(netip.AddrFrom4(ueIP), pdr); err != nil {
		t.Fatalf("install S1-U downlink PDR: %v", err)
	}
}

func TestGTPEncapsulationDownlinkS1U(t *testing.T) {
	requireProgTestRun(t)

	obj := loadProgram(t, 1, 0)

	var (
		ueIP   = [4]byte{10, 45, 0, 2}
		local  = [4]byte{192, 168, 100, 1}
		remote = [4]byte{192, 168, 100, 9}
	)

	const teid = 0x55667788

	putDownlinkPDRS1U(t, obj, ueIP, teid, local, remote)

	inner := ipv4Packet([4]byte{8, 8, 8, 8}, ueIP, 17, udpDatagram(4000, 4001, []byte{0xde, 0xad, 0xbe, 0xef}))

	action, out := runXDPOut(t, obj.UpfEntryFunc, ethFrame(0x0800, inner))

	if action == ActionAborted {
		t.Fatal("S1-U downlink packet got ActionAborted; encapsulation failed")
	}

	if len(out) != ethHdrLen+gtpV4EncapLenS1U+len(inner) {
		t.Fatalf("S1-U frame length = %d, want %d (no PDU session container)",
			len(out), ethHdrLen+gtpV4EncapLenS1U+len(inner))
	}

	gtp := out[ethHdrLen+28:]

	if gtp[0]&0x04 != 0 {
		t.Errorf("GTP E flag set on an S1-U G-PDU (flags = %#02x); want no extension header", gtp[0])
	}

	if gtp[1] != 0xFF {
		t.Errorf("GTP message type = %#02x, want 0xFF (G-PDU)", gtp[1])
	}

	if msgLen := binary.BigEndian.Uint16(gtp[2:4]); int(msgLen) != len(inner) {
		t.Errorf("GTP message length = %d, want %d (inner only, no extension)", msgLen, len(inner))
	}

	if got := binary.BigEndian.Uint32(gtp[4:8]); got != teid {
		t.Errorf("GTP TEID = %#x, want %#x", got, uint32(teid))
	}

	if !bytes.Equal(out[ethHdrLen+gtpV4EncapLenS1U:], inner) {
		t.Errorf("inner packet altered by S1-U encapsulation:\n got %x\nwant %x",
			out[ethHdrLen+gtpV4EncapLenS1U:], inner)
	}
}

func TestGTPEncapsulationDownlinkIPv4(t *testing.T) {
	requireProgTestRun(t)

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

	action, out := runXDPOut(t, obj.UpfEntryFunc, ethFrame(0x0800, inner))

	if action == ActionAborted {
		t.Fatal("downlink packet got ActionAborted; encapsulation failed")
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

	action, out := runXDPOut(t, obj.UpfEntryFunc, ethFrame(0x86DD, inner))

	if action == ActionAborted {
		t.Fatal("downlink IPv6 packet got ActionAborted; encapsulation failed")
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

// RFC 6936
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

	action, out := runXDPOut(t, obj.UpfEntryFunc, ethFrame(0x0800, inner))

	if action == ActionAborted {
		t.Fatal("downlink packet got ActionAborted; IPv6-transport encapsulation failed")
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

func TestDownlinkIPv6TransportRejectsOverDeclaredInnerLength(t *testing.T) {
	requireProgTestRun(t)

	obj := loadProgram(t, 1, 0)

	ueIP := [4]byte{10, 45, 0, 2}
	server := [4]byte{8, 8, 8, 8}
	local := netip.MustParseAddr("2001:db8:33::1").As16()
	remote := netip.MustParseAddr("2001:db8:33::9").As16()

	const (
		teid = 0x77778888
		qfi  = 7
	)

	putDownlinkPDRv6Outer(t, obj, ueIP, teid, local, remote, qfi)

	inner := ipv4Packet(server, ueIP, 17,
		udpDatagramChecksummed(server, ueIP, 4000, 4001, []byte{0x01, 0x02, 0x03, 0x04}))

	binary.BigEndian.PutUint16(inner[2:4], uint16(len(inner)+200))
	binary.BigEndian.PutUint16(inner[10:12], 0)
	binary.BigEndian.PutUint16(inner[10:12], ipv4HeaderChecksum(inner[:20]))

	before := DropCount(obj, Downlink, "internal_encap_failed")

	action, _ := runXDPOut(t, obj.UpfEntryFunc, ethFrame(0x0800, inner))
	if action != ActionAborted {
		t.Fatalf("action = %d, want ActionAborted (%d): the frame left with outer lengths it does not carry",
			action, ActionAborted)
	}

	if after := DropCount(obj, Downlink, "internal_encap_failed"); after != before+1 {
		t.Errorf("internal_encap_failed = %d, want %d", after, before+1)
	}
}

func TestTransportLevelMarking(t *testing.T) {
	requireProgTestRun(t)

	const (
		teid    = 0x544F5301
		qfi     = 5
		wantTOS = 0xB8
	)

	obj := loadProgram(t, 1, 0)

	pdr := ipv4OuterDownlinkPDR(teid, testUPFN3IP, testGNBIP, qfi)
	pdr.Far.TransportLevelMarking = uint16(wantTOS) << 8

	if err := obj.PutPdrDownlink(netip.AddrFrom4(ueIP), pdr); err != nil {
		t.Fatalf("install downlink PDR: %v", err)
	}

	inner := ipv4Packet(serverIP, ueIP, 17, udpDatagram(4000, 53, nil))

	action, out := runXDPOut(t, obj.UpfEntryFunc, ethFrame(0x0800, inner))
	if action == ActionAborted {
		t.Fatal("downlink packet got ActionAborted")
	}

	if tos := out[ethHdrLen+1]; tos != wantTOS {
		t.Errorf("outer IPv4 TOS = %#02x, want %#02x (FAR transport-level marking)", tos, wantTOS)
	}

	if f := parseGTPv4Frame(t, out); !f.outerChecksumOK {
		t.Error("outer IPv4 header checksum invalid after marking")
	}
}

func ipv4OuterDownlinkPDR(teid uint32, local, remote [4]byte, qfi uint8) PdrInfo {
	return PdrInfo{
		IMSI: "001010000000001",
		Far: FarInfo{
			Action:              0x02,
			OuterHeaderCreation: 0x01,
			TeID:                teid,
			LocalIP:             IPToIn6Addr(netip.AddrFrom4(local)),
			RemoteIP:            IPToIn6Addr(netip.AddrFrom4(remote)),
		},
		Qer: QerInfo{GateStatusDL: 0, Qfi: qfi, MaxBitrateDL: 0},
	}
}

func putDownlinkPDR(t *testing.T, obj *BpfObjects, ueIP [4]byte, teid uint32, local, remote [4]byte, qfi uint8) {
	t.Helper()

	if err := obj.PutPdrDownlink(netip.AddrFrom4(ueIP), ipv4OuterDownlinkPDR(teid, local, remote, qfi)); err != nil {
		t.Fatalf("install downlink PDR: %v", err)
	}
}

func putDownlinkPDRFiltered(t *testing.T, obj *BpfObjects, ueIP [4]byte, teid uint32, local, remote [4]byte, qfi uint8, filterIndex uint32) {
	t.Helper()

	pdr := ipv4OuterDownlinkPDR(teid, local, remote, qfi)
	pdr.FilterMapIndex = filterIndex

	if err := obj.PutPdrDownlink(netip.AddrFrom4(ueIP), pdr); err != nil {
		t.Fatalf("install filtered downlink PDR: %v", err)
	}
}

func putDownlinkPDRv6UE(t *testing.T, obj *BpfObjects, uePrefix netip.Addr, teid uint32, local, remote [4]byte, qfi uint8) {
	t.Helper()

	if err := obj.PutPdrDownlink(uePrefix, ipv4OuterDownlinkPDR(teid, local, remote, qfi)); err != nil {
		t.Fatalf("install downlink IPv6 PDR: %v", err)
	}
}

func ipv6OuterDownlinkPDR(teid uint32, local, remote [16]byte, qfi uint8) PdrInfo {
	return PdrInfo{
		IMSI: "001010000000001",
		Far: FarInfo{
			Action:              0x02,
			OuterHeaderCreation: 0x02,
			TeID:                teid,
			LocalIP:             local,
			RemoteIP:            remote,
		},
		Qer: QerInfo{GateStatusDL: 0, Qfi: qfi, MaxBitrateDL: 0},
	}
}

func putDownlinkPDRv6Outer(t *testing.T, obj *BpfObjects, ueIP [4]byte, teid uint32, local, remote [16]byte, qfi uint8) { //nolint:unparam // signature mirrors putDownlinkPDR
	t.Helper()

	if err := obj.PutPdrDownlink(netip.AddrFrom4(ueIP), ipv6OuterDownlinkPDR(teid, local, remote, qfi)); err != nil {
		t.Fatalf("install downlink IPv6-transport PDR: %v", err)
	}
}

func TestGTPEncapsulationInnerIPv6ExtensionHeaderChecksum(t *testing.T) {
	requireProgTestRun(t)

	const (
		teid = 0x77779999
		qfi  = 5
	)

	local := netip.MustParseAddr("2001:db8:44::1").As16()
	remote := netip.MustParseAddr("2001:db8:44::9").As16()
	serverV6 := [16]byte{0x20, 0x01, 0x48, 0x60, 0x48, 0x60, 0, 0, 0, 0, 0, 0, 0, 0, 0x88, 0x88}

	udp := udpDatagramChecksummedV6(serverV6, testUEv6, 4000, 53, []byte{9, 9, 9, 9})

	tests := []struct {
		name  string
		inner []byte
	}{
		{"plain UDP", ipv6Packet(serverV6, testUEv6, 17, udp)},
		{
			"behind a Hop-by-Hop Options header",
			ipv6Packet(serverV6, testUEv6, ipprotoHopOpts, append(hopByHopHeader(17), udp...)),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			obj := loadProgram(t, 1, 0)

			pdr := ipv4OuterDownlinkPDR(teid, testUPFN3IP, testGNBIP, qfi)
			pdr.Far.OuterHeaderCreation = 0x02
			pdr.Far.LocalIP = local
			pdr.Far.RemoteIP = remote

			if err := obj.PutPdrDownlink(netip.MustParseAddr("2001:db8::"), pdr); err != nil {
				t.Fatalf("install downlink IPv6 PDR: %v", err)
			}

			action, out := runXDPOut(t, obj.UpfEntryFunc, ethFrame(0x86DD, tc.inner))

			if action == ActionDrop || action == ActionAborted {
				t.Fatalf("downlink packet got XDP action %d", action)
			}

			if len(out) != ethHdrLen+gtpV6EncapLen+len(tc.inner) {
				t.Fatalf("encapsulated frame length = %d, want %d", len(out), ethHdrLen+gtpV6EncapLen+len(tc.inner))
			}

			f := parseGTPv6Frame(t, out)

			if !f.udpChecksumOK {
				t.Error("outer UDP checksum invalid")
			}

			if !bytes.Equal(f.inner, tc.inner) {
				t.Errorf("inner packet altered by encapsulation:\n got %x\nwant %x", f.inner, tc.inner)
			}
		})
	}
}
