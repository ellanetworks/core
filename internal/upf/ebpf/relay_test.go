// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package ebpf

import (
	"bytes"
	"encoding/binary"
	"net/netip"
	"strings"
	"testing"
)

const (
	gtpMsgEndMarker = 254
	relayDSCP       = uint8(0xB8)
)

var testTargetIP = [4]byte{10, 0, 0, 3}

var testTargetv6 = [16]byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0xaa, 0, 0, 0, 0, 0, 0, 0, 0x03}

func forwardingUplinkPDR(outTEID uint32, local, remote [4]byte) PdrInfo {
	return PdrInfo{
		IMSI:       "001010000000001",
		Forwarding: true,
		Far: FarInfo{
			Action:              0x02,
			OuterHeaderCreation: 0x01,
			TeID:                outTEID,
			LocalIP:             IPToIn6Addr(netip.AddrFrom4(local)),
			RemoteIP:            IPToIn6Addr(netip.AddrFrom4(remote)),
		},
	}
}

func forwardingUplinkPDRv6(outTEID uint32, local, remote [16]byte) PdrInfo {
	return PdrInfo{
		IMSI:       "001010000000001",
		Forwarding: true,
		Far: FarInfo{
			Action:              0x02,
			OuterHeaderCreation: 0x02,
			TeID:                outTEID,
			LocalIP:             local,
			RemoteIP:            remote,
		},
	}
}

func endMarkerPDU(teid uint32) []byte {
	gtp := make([]byte, 8)
	gtp[0] = 0x30
	gtp[1] = gtpMsgEndMarker
	binary.BigEndian.PutUint16(gtp[2:4], 0)
	binary.BigEndian.PutUint32(gtp[4:8], teid)

	return gtp
}

func TestRelayForwardedGPDUIPv4(t *testing.T) {
	requireProgTestRun(t)

	const (
		inTEID  = uint32(0x5A5A0001)
		outTEID = uint32(0x5A5A0002)
	)

	obj := loadN3N6Program(t)

	if err := obj.PutPdrUplink(inTEID, forwardingUplinkPDR(outTEID, testUPFN3IP, testTargetIP)); err != nil {
		t.Fatalf("install forwarding PDR: %v", err)
	}

	inner := innerIPv4UDP([4]byte{8, 8, 8, 8}, 53)
	in := uplinkGPDU(inTEID, inner)

	action, out := runXDPOut(t, obj.UpfEntryFunc, in)

	if action == ActionDrop || action == ActionAborted {
		t.Fatalf("relayed G-PDU got XDP action %d, want a forwarding action", action)
	}

	if len(out) != len(in) {
		t.Fatalf("relayed frame length = %d, want %d (the relay must not resize)", len(out), len(in))
	}

	ip := out[ethHdrLen:]

	if got := [4]byte(ip[12:16]); got != testUPFN3IP {
		t.Errorf("outer source = %v, want the UPF N3 address %v", got, testUPFN3IP)
	}

	if got := [4]byte(ip[16:20]); got != testTargetIP {
		t.Errorf("outer destination = %v, want the target %v", got, testTargetIP)
	}

	hdr := bytes.Clone(ip[:20])
	binary.BigEndian.PutUint16(hdr[10:12], 0)

	if got, want := binary.BigEndian.Uint16(ip[10:12]), ipv4HeaderChecksum(hdr); got != want {
		t.Errorf("outer IPv4 checksum = %#04x, want %#04x", got, want)
	}

	udp := ip[20:]

	if got := binary.BigEndian.Uint16(udp[6:8]); got != 0 {
		t.Errorf("outer UDP checksum = %#04x, want 0 over IPv4", got)
	}

	gtp := udp[8:]

	if got := binary.BigEndian.Uint32(gtp[4:8]); got != outTEID {
		t.Errorf("relayed TEID = %#x, want the target's %#x", got, outTEID)
	}

	inGTP := in[ethHdrLen+20+8:]

	if !bytes.Equal(gtp[8:], inGTP[8:]) {
		t.Errorf("the relay altered the GTP payload:\n got %x\nwant %x", gtp[8:], inGTP[8:])
	}
}

func TestRelayForwardedEndMarkerIPv4(t *testing.T) {
	requireProgTestRun(t)

	const (
		inTEID  = uint32(0x5A5A0011)
		outTEID = uint32(0x5A5A0012)
	)

	obj := loadN3N6Program(t)

	if err := obj.PutPdrUplink(inTEID, forwardingUplinkPDR(outTEID, testUPFN3IP, testTargetIP)); err != nil {
		t.Fatalf("install forwarding PDR: %v", err)
	}

	action, out := runXDPOut(t, obj.UpfEntryFunc, gtpV4Outer(endMarkerPDU(inTEID)))

	if action == ActionDrop || action == ActionAborted {
		t.Fatalf("relayed End Marker got XDP action %d, want a forwarding action", action)
	}

	ip := out[ethHdrLen:]
	gtp := ip[20+8:]

	if got := [4]byte(ip[16:20]); got != testTargetIP {
		t.Errorf("outer destination = %v, want the target %v", got, testTargetIP)
	}

	if gtp[1] != gtpMsgEndMarker {
		t.Errorf("relayed message type = %d, want an End Marker (%d)", gtp[1], gtpMsgEndMarker)
	}

	if got := binary.BigEndian.Uint32(gtp[4:8]); got != outTEID {
		t.Errorf("relayed TEID = %#x, want the target's %#x", got, outTEID)
	}
}

func TestRelayEndMarkerWithoutForwardingPDRNotRelayed(t *testing.T) {
	requireProgTestRun(t)

	const teid = uint32(0x5A5A0021)

	obj := loadN3N6Program(t)
	putForwardingUplinkPDR(t, obj, teid, 0)

	action, out := runXDPOut(t, obj.UpfEntryFunc, gtpV4Outer(endMarkerPDU(teid)))

	if action == ActionDrop || action == ActionAborted {
		t.Fatalf("End Marker on an ordinary session got XDP action %d, want it passed to the stack", action)
	}

	ip := out[ethHdrLen:]

	if got := [4]byte(ip[16:20]); got != testUPFN3IP {
		t.Errorf("outer destination = %v, want it left at the UPF %v", got, testUPFN3IP)
	}
}

func TestRelayForwardedGPDUIPv6(t *testing.T) {
	requireProgTestRun(t)

	const (
		inTEID  = uint32(0x5A5A0031)
		outTEID = uint32(0x5A5A0032)
	)

	obj := loadN3N6Program(t)

	if err := obj.PutPdrUplink(inTEID, forwardingUplinkPDRv6(outTEID, testUPFN3v6, testTargetv6)); err != nil {
		t.Fatalf("install forwarding PDR: %v", err)
	}

	inner := innerIPv4UDP([4]byte{8, 8, 8, 8}, 53)
	gtp := gtpHeader(inTEID, inner)
	udp := udpDatagramChecksummedV6(testGNBv6, testUPFN3v6, GTPUDPPort, GTPUDPPort, gtp)
	in := ethFrame(0x86DD, ipv6Packet(testGNBv6, testUPFN3v6, 17, udp))

	if !validUDPv6Checksum(testGNBv6, testUPFN3v6, udp) {
		t.Fatalf("the test built an invalid inbound UDP checksum")
	}

	action, out := runXDPOut(t, obj.UpfEntryFunc, in)

	if action == ActionDrop || action == ActionAborted {
		t.Fatalf("relayed IPv6 G-PDU got XDP action %d, want a forwarding action", action)
	}

	ip := out[ethHdrLen:]

	if got := [16]byte(ip[8:24]); got != testUPFN3v6 {
		t.Errorf("outer source = %x, want the UPF N3 address %x", got, testUPFN3v6)
	}

	if got := [16]byte(ip[24:40]); got != testTargetv6 {
		t.Errorf("outer destination = %x, want the target %x", got, testTargetv6)
	}

	outUDP := ip[40:]

	if !validUDPv6Checksum(testUPFN3v6, testTargetv6, outUDP) {
		t.Errorf("the relay left an invalid outer UDP checksum over IPv6")
	}

	if got := binary.BigEndian.Uint32(outUDP[8+4 : 8+8]); got != outTEID {
		t.Errorf("relayed TEID = %#x, want the target's %#x", got, outTEID)
	}
}

func TestRelayFamilyMismatchDropped(t *testing.T) {
	requireProgTestRun(t)

	const (
		inTEID  = uint32(0x5A5A0041)
		outTEID = uint32(0x5A5A0042)
	)

	obj := loadN3N6Program(t)

	if err := obj.PutPdrUplink(inTEID, forwardingUplinkPDRv6(outTEID, testUPFN3v6, testTargetv6)); err != nil {
		t.Fatalf("install forwarding PDR: %v", err)
	}

	inner := innerIPv4UDP([4]byte{8, 8, 8, 8}, 53)

	action := runXDP(t, obj.UpfEntryFunc, uplinkGPDU(inTEID, inner))

	if action != ActionDrop {
		t.Fatalf("IPv4 transport relayed to an IPv6 tunnel: got XDP action %d, want ActionDrop (%d)", action, ActionDrop)
	}

	if got := DropCount(obj, Uplink, "far_unsupported"); got != 1 {
		t.Errorf("far_unsupported = %d, want 1", got)
	}
}

func TestRelayNotForwardingFARDropped(t *testing.T) {
	requireProgTestRun(t)

	const inTEID = uint32(0x5A5A0051)

	obj := loadN3N6Program(t)

	pdr := forwardingUplinkPDR(0x5A5A0052, testUPFN3IP, testTargetIP)
	pdr.Far.Action = 0x01

	if err := obj.PutPdrUplink(inTEID, pdr); err != nil {
		t.Fatalf("install forwarding PDR: %v", err)
	}

	inner := innerIPv4UDP([4]byte{8, 8, 8, 8}, 53)

	action := runXDP(t, obj.UpfEntryFunc, uplinkGPDU(inTEID, inner))

	if action != ActionDrop {
		t.Fatalf("forwarding PDR with FAR DROP: got XDP action %d, want ActionDrop (%d)", action, ActionDrop)
	}

	if got := DropCount(obj, Uplink, "far_no_forward"); got != 1 {
		t.Errorf("far_no_forward = %d, want 1", got)
	}
}

func TestRelayIndependentOfLocalSwitch(t *testing.T) {
	requireProgTestRun(t)

	const (
		inTEID  = uint32(0x5A5A0061)
		outTEID = uint32(0x5A5A0062)
		ueIP    = "10.0.0.9"
	)

	obj := loadN3N6Program(t)

	if err := obj.PutPdrUplink(inTEID, forwardingUplinkPDR(outTEID, testUPFN3IP, testTargetIP)); err != nil {
		t.Fatalf("install forwarding PDR: %v", err)
	}

	putDownlinkPDR(t, obj, [4]byte{10, 0, 0, 9}, 0x1234, testUPFN3IP, testGNBIP, 5)

	inner := ipv4Packet([4]byte{8, 8, 8, 8}, [4]byte{10, 0, 0, 9}, 17, udpDatagram(53, 4000, nil))

	action, out := runXDPOut(t, obj.UpfEntryFunc, uplinkGPDU(inTEID, inner))

	if action == ActionDrop || action == ActionAborted {
		t.Fatalf("relayed G-PDU got XDP action %d, want a forwarding action", action)
	}

	ip := out[ethHdrLen:]

	if got := [4]byte(ip[16:20]); got != testTargetIP {
		t.Fatalf("outer destination = %v, want the target %v; a packet addressed to a local UE must still relay", got, testTargetIP)
	}

	if got := binary.BigEndian.Uint32(ip[20+8+4 : 20+8+8]); got != outTEID {
		t.Errorf("relayed TEID = %#x, want the target's %#x", got, outTEID)
	}
}

func uplinkGPDUChecksummed(teid uint32, inner []byte) []byte {
	return ethFrame(0x0800, ipv4Packet(testGNBIP, testUPFN3IP, 17,
		udpDatagramChecksummed(testGNBIP, testUPFN3IP, GTPUDPPort, GTPUDPPort,
			gtpHeader(teid, inner))))
}

func uplinkGPDUv6Checksummed(teid uint32, inner []byte) []byte {
	return ethFrame(0x86DD, ipv6Packet(testGNBv6, testUPFN3v6, 17,
		udpDatagramChecksummedV6(testGNBv6, testUPFN3v6, GTPUDPPort, GTPUDPPort,
			gtpHeader(teid, inner))))
}

func withOuterTTLv4(frame []byte, ttl uint8) []byte {
	frame[ethHdrLen+8] = ttl

	binary.BigEndian.PutUint16(frame[ethHdrLen+10:ethHdrLen+12], 0)
	binary.BigEndian.PutUint16(frame[ethHdrLen+10:ethHdrLen+12],
		ipv4HeaderChecksum(frame[ethHdrLen:ethHdrLen+20]))

	return frame
}

func withOuterTTLv6(frame []byte, hopLimit uint8) []byte {
	frame[ethHdrLen+7] = hopLimit

	return frame
}

func markedForwardingPDR(pdr PdrInfo) PdrInfo {
	pdr.Far.TransportLevelMarking = uint16(relayDSCP)<<8 | 0xFC

	return pdr
}

func relayCases(inTEID, outTEID uint32) []struct {
	name  string
	pdr   PdrInfo
	frame []byte
} {
	inner := innerIPv4UDP([4]byte{8, 8, 8, 8}, 53)
	v4 := forwardingUplinkPDR(outTEID, testUPFN3IP, testTargetIP)
	v6 := forwardingUplinkPDRv6(outTEID, testUPFN3v6, testTargetv6)
	optioned := ethFrame(0x0800, ipv4PacketWithOptions(testGNBIP, testUPFN3IP, 17,
		[]byte{7, 8, 4, 0, 10, 0, 0, 1},
		udpDatagramChecksummed(testGNBIP, testUPFN3IP, GTPUDPPort, GTPUDPPort,
			gtpHeader(inTEID, inner))))

	return []struct {
		name  string
		pdr   PdrInfo
		frame []byte
	}{
		{"ipv4-no-checksum", markedForwardingPDR(v4), withOuterTTLv4(uplinkGPDU(inTEID, inner), 3)},
		{"ipv4-checksummed", markedForwardingPDR(v4), withOuterTTLv4(uplinkGPDUChecksummed(inTEID, inner), 3)},
		{"ipv4-unmarked", v4, withOuterTTLv4(uplinkGPDUChecksummed(inTEID, inner), 3)},
		{"ipv4-options", markedForwardingPDR(v4), optioned},
		{"ipv6", markedForwardingPDR(v6), withOuterTTLv6(uplinkGPDUv6Checksummed(inTEID, inner), 3)},
		{"ipv6-unmarked", v6, withOuterTTLv6(uplinkGPDUv6Checksummed(inTEID, inner), 3)},
	}
}

func TestRelayTCMatchesXDPOutput(t *testing.T) {
	requireProgTestRun(t)

	const (
		inTEID  = uint32(0x5A5A0071)
		outTEID = uint32(0x5A5A0072)
	)

	for _, tc := range relayCases(inTEID, outTEID) {
		t.Run(tc.name, func(t *testing.T) {
			xdpObj := loadN3N6Program(t)
			tcObjs := loadTCProgramConfig(t, false, 0, 1)

			if err := xdpObj.PutPdrUplink(inTEID, tc.pdr); err != nil {
				t.Fatalf("install XDP forwarding PDR: %v", err)
			}

			putTCPdrUplink(t, tcObjs, inTEID, tc.pdr)

			xdpAction, xdpOut := runXDPOut(t, xdpObj.UpfEntryFunc, tc.frame)
			tcAction, tcOut := runTC(t, tcObjs.UpfEntryFunc, tc.frame)

			if xdpAction == ActionDrop || xdpAction == ActionAborted {
				t.Fatalf("relayed frame dropped on XDP (action %d)", xdpAction)
			}

			if !verdictsEquivalent(xdpAction, tcAction) {
				t.Errorf("verdicts diverge: XDP %d, TC %d", xdpAction, tcAction)
			}

			if !bytes.Equal(xdpOut, tcOut) {
				t.Errorf("relayed frames diverge:\nXDP (%d bytes): %x\nTC  (%d bytes): %x",
					len(xdpOut), xdpOut, len(tcOut), tcOut)
			}
		})
	}
}

func TestRelayTCMaintainsChecksumComplete(t *testing.T) {
	requireProgTestRun(t)

	const (
		inTEID  = uint32(0x5A5A0081)
		outTEID = uint32(0x5A5A0082)
	)

	for _, tc := range relayCases(inTEID, outTEID) {
		t.Run(tc.name, func(t *testing.T) {
			tcObjs := loadTCProgramConfig(t, false, 0, 1)
			putTCPdrUplink(t, tcObjs, inTEID, tc.pdr)

			action, out := runTCChecksumComplete(t, tcObjs.UpfEntryFunc, tc.frame)

			if action == tcActShot {
				t.Fatalf("relayed frame got TC_ACT_SHOT, want a forwarding action")
			}

			if strings.HasPrefix(tc.name, "ipv6") {
				if !validUDPv6Checksum(testUPFN3v6, testTargetv6, out[ethHdrLen+40:]) {
					t.Errorf("the relay left an invalid outer UDP checksum over IPv6")
				}

				return
			}

			ip := out[ethHdrLen:]
			hdrLen := int(ip[0]&0x0f) * 4
			udp := ip[hdrLen:]

			if onesComplement16(ip[:hdrLen]) != 0 {
				t.Errorf("the relay left an invalid outer IPv4 checksum (%#04x)",
					binary.BigEndian.Uint16(ip[10:12]))
			}

			if got := binary.BigEndian.Uint16(udp[6:8]); got != 0 &&
				!validUDPv4Checksum(testUPFN3IP, testTargetIP, udp) {
				t.Errorf("the relay left an invalid outer UDP checksum over IPv4 (%#04x)", got)
			}
		})
	}
}

func validUDPv4Checksum(src, dst [4]byte, udpSegment []byte) bool {
	pseudo := make([]byte, 12+len(udpSegment))

	copy(pseudo[0:4], src[:])
	copy(pseudo[4:8], dst[:])
	pseudo[9] = 17
	binary.BigEndian.PutUint16(pseudo[10:12], uint16(len(udpSegment)))
	copy(pseudo[12:], udpSegment)

	return onesComplement16(pseudo) == 0
}

func TestRelayOnlyMarkedForwardingPDRRelays(t *testing.T) {
	requireProgTestRun(t)

	const (
		inTEID  = uint32(0x5A5A0071)
		outTEID = uint32(0x5A5A0072)
	)

	obj := loadN3N6Program(t)

	pdr := forwardingUplinkPDR(outTEID, testUPFN3IP, testTargetIP)
	pdr.Forwarding = false

	if err := obj.PutPdrUplink(inTEID, pdr); err != nil {
		t.Fatalf("install PDR: %v", err)
	}

	inner := innerIPv4UDP([4]byte{8, 8, 8, 8}, 53)

	action := runXDP(t, obj.UpfEntryFunc, uplinkGPDU(inTEID, inner))

	if action != ActionDrop {
		t.Fatalf("an encapsulating FAR outside a forwarding tunnel got XDP action %d, want ActionDrop (%d)", action, ActionDrop)
	}

	if got := DropCount(obj, Uplink, "far_unsupported"); got != 1 {
		t.Errorf("far_unsupported = %d, want 1", got)
	}
}

func TestRelayOriginatesOuterHeaderIPv4(t *testing.T) {
	requireProgTestRun(t)

	const (
		inTEID  = uint32(0x5A5A0091)
		outTEID = uint32(0x5A5A0092)
	)

	obj := loadN3N6Program(t)

	pdr := forwardingUplinkPDR(outTEID, testUPFN3IP, testTargetIP)
	pdr.Far.TransportLevelMarking = uint16(relayDSCP)<<8 | 0xFC

	if err := obj.PutPdrUplink(inTEID, pdr); err != nil {
		t.Fatalf("install forwarding PDR: %v", err)
	}

	in := uplinkGPDU(inTEID, innerIPv4UDP([4]byte{8, 8, 8, 8}, 53))
	in[ethHdrLen+8] = 3

	binary.BigEndian.PutUint16(in[ethHdrLen+10:ethHdrLen+12], 0)
	binary.BigEndian.PutUint16(in[ethHdrLen+10:ethHdrLen+12],
		ipv4HeaderChecksum(in[ethHdrLen:ethHdrLen+20]))

	action, out := runXDPOut(t, obj.UpfEntryFunc, in)

	if action == ActionDrop || action == ActionAborted {
		t.Fatalf("relayed G-PDU got XDP action %d, want a forwarding action", action)
	}

	ip := out[ethHdrLen:]

	if got := ip[8]; got != 64 {
		t.Errorf("outer TTL = %d, want the UPF's own %d, not the source's", got, 64)
	}

	if got := ip[1]; got != relayDSCP {
		t.Errorf("outer ToS = %#02x, want the FAR's transport level marking %#02x", got, relayDSCP)
	}

	hdr := bytes.Clone(ip[:20])
	binary.BigEndian.PutUint16(hdr[10:12], 0)

	if got, want := binary.BigEndian.Uint16(ip[10:12]), ipv4HeaderChecksum(hdr); got != want {
		t.Errorf("outer IPv4 checksum = %#04x, want %#04x", got, want)
	}
}

func TestRelayOriginatesOuterHeaderIPv6(t *testing.T) {
	requireProgTestRun(t)

	const (
		inTEID  = uint32(0x5A5A00a1)
		outTEID = uint32(0x5A5A00a2)
	)

	obj := loadN3N6Program(t)

	pdr := forwardingUplinkPDRv6(outTEID, testUPFN3v6, testTargetv6)
	pdr.Far.TransportLevelMarking = uint16(relayDSCP)<<8 | 0xFC

	if err := obj.PutPdrUplink(inTEID, pdr); err != nil {
		t.Fatalf("install forwarding PDR: %v", err)
	}

	in := uplinkGPDUv6Checksummed(inTEID, innerIPv4UDP([4]byte{8, 8, 8, 8}, 53))
	in[ethHdrLen+7] = 3
	in[ethHdrLen+1] = 0x0f
	in[ethHdrLen+2] = 0xcd
	in[ethHdrLen+3] = 0xef

	action, out := runXDPOut(t, obj.UpfEntryFunc, in)

	if action == ActionDrop || action == ActionAborted {
		t.Fatalf("relayed G-PDU got XDP action %d, want a forwarding action", action)
	}

	ip := out[ethHdrLen:]

	if got := ip[7]; got != 64 {
		t.Errorf("outer hop limit = %d, want the UPF's own %d, not the source's", got, 64)
	}

	if got := ((ip[0] & 0x0f) << 4) | (ip[1] >> 4); got != relayDSCP {
		t.Errorf("outer traffic class = %#02x, want the FAR's transport level marking %#02x", got, relayDSCP)
	}

	if got := uint32(ip[1]&0x0f)<<16 | uint32(ip[2])<<8 | uint32(ip[3]); got != 0x0fcdef {
		t.Errorf("outer flow label = %#06x, want it preserved as %#06x", got, 0x0fcdef)
	}

	if !validUDPv6Checksum(testUPFN3v6, testTargetv6, ip[40:]) {
		t.Errorf("the relay left an invalid outer UDP checksum over IPv6")
	}
}

func ipv4PacketWithOptions(src, dst [4]byte, proto uint8, options, payload []byte) []byte {
	if len(options)%4 != 0 || len(options) == 0 || len(options) > 40 {
		panic("IPv4 options must be a non-empty multiple of 4, at most 40 bytes")
	}

	hdrLen := 20 + len(options)
	pkt := make([]byte, hdrLen+len(payload))

	pkt[0] = 0x40 | uint8(hdrLen/4)
	binary.BigEndian.PutUint16(pkt[2:4], uint16(hdrLen+len(payload)))
	pkt[8] = 64
	pkt[9] = proto
	copy(pkt[12:16], src[:])
	copy(pkt[16:20], dst[:])
	copy(pkt[20:hdrLen], options)
	binary.BigEndian.PutUint16(pkt[10:12], ipv4HeaderChecksum(pkt[:hdrLen]))
	copy(pkt[hdrLen:], payload)

	return pkt
}

func TestRelayOuterIPv4Options(t *testing.T) {
	requireProgTestRun(t)

	const (
		inTEID  = uint32(0x5A5A00b1)
		outTEID = uint32(0x5A5A00b2)
	)

	obj := loadN3N6Program(t)

	if err := obj.PutPdrUplink(inTEID, forwardingUplinkPDR(outTEID, testUPFN3IP, testTargetIP)); err != nil {
		t.Fatalf("install forwarding PDR: %v", err)
	}

	options := []byte{7, 8, 4, 0, 10, 0, 0, 1}
	gtp := gtpHeader(inTEID, innerIPv4UDP([4]byte{8, 8, 8, 8}, 53))
	in := ethFrame(0x0800, ipv4PacketWithOptions(testGNBIP, testUPFN3IP, 17, options,
		udpDatagram(GTPUDPPort, GTPUDPPort, gtp)))

	action, out := runXDPOut(t, obj.UpfEntryFunc, in)

	if action == ActionDrop || action == ActionAborted {
		t.Fatalf("G-PDU with outer IPv4 options got XDP action %d, want a forwarding action", action)
	}

	ip := out[ethHdrLen:]
	hdrLen := int(ip[0]&0x0f) * 4

	if hdrLen != 28 {
		t.Fatalf("outer header length = %d, want the options preserved at 28", hdrLen)
	}

	if !bytes.Equal(ip[20:28], options) {
		t.Errorf("outer IPv4 options = %x, want them preserved as %x", ip[20:28], options)
	}

	if got := [4]byte(ip[16:20]); got != testTargetIP {
		t.Errorf("outer destination = %v, want the target %v", got, testTargetIP)
	}

	if onesComplement16(ip[:hdrLen]) != 0 {
		t.Errorf("outer IPv4 checksum %#04x does not cover the option bytes",
			binary.BigEndian.Uint16(ip[10:12]))
	}

	if got := binary.BigEndian.Uint32(ip[hdrLen+8+4 : hdrLen+8+8]); got != outTEID {
		t.Errorf("relayed TEID = %#x, want the target's %#x", got, outTEID)
	}
}

func TestRelayPreservesInvalidOuterIPv4Checksum(t *testing.T) {
	requireProgTestRun(t)

	const (
		inTEID  = uint32(0x5A5A00c1)
		outTEID = uint32(0x5A5A00c2)
	)

	obj := loadN3N6Program(t)

	if err := obj.PutPdrUplink(inTEID, forwardingUplinkPDR(outTEID, testUPFN3IP, testTargetIP)); err != nil {
		t.Fatalf("install forwarding PDR: %v", err)
	}

	in := uplinkGPDU(inTEID, innerIPv4UDP([4]byte{8, 8, 8, 8}, 53))

	corrupt := binary.BigEndian.Uint16(in[ethHdrLen+10:ethHdrLen+12]) ^ 0x0100
	binary.BigEndian.PutUint16(in[ethHdrLen+10:ethHdrLen+12], corrupt)

	_, out := runXDPOut(t, obj.UpfEntryFunc, in)

	if onesComplement16(out[ethHdrLen:ethHdrLen+20]) == 0 {
		t.Error("the relay recomputed a corrupt outer IPv4 checksum into a valid one; a forwarder carries the error through")
	}
}

func firstFragmentGPDU(teid uint32) []byte {
	gtp := gtpHeader(teid, innerIPv4UDP([4]byte{8, 8, 8, 8}, 53))
	ip4 := ipv4Packet(testGNBIP, testUPFN3IP, 17, udpDatagram(GTPUDPPort, GTPUDPPort, gtp))

	binary.BigEndian.PutUint16(ip4[6:8], 0x2000)
	binary.BigEndian.PutUint16(ip4[10:12], 0)
	binary.BigEndian.PutUint16(ip4[10:12], ipv4HeaderChecksum(ip4[:20]))

	return ethFrame(0x0800, ip4)
}

func TestFragmentedTransportNotRelayed(t *testing.T) {
	requireProgTestRun(t)

	const (
		inTEID  = uint32(0x5A5A00d1)
		outTEID = uint32(0x5A5A00d2)
	)

	obj := loadN3N6Program(t)

	if err := obj.PutPdrUplink(inTEID, forwardingUplinkPDR(outTEID, testUPFN3IP, testTargetIP)); err != nil {
		t.Fatalf("install forwarding PDR: %v", err)
	}

	action := runXDP(t, obj.UpfEntryFunc, firstFragmentGPDU(inTEID))

	if action != ActionDrop {
		t.Fatalf("first fragment of a GTP-U transport got XDP action %d, want ActionDrop (%d): only part of the payload is present", action, ActionDrop)
	}

	if got := DropCount(obj, Uplink, "fragmented_transport"); got != 1 {
		t.Errorf("fragmented_transport = %d, want 1", got)
	}
}

func TestFragmentedTransportNotDecapsulated(t *testing.T) {
	requireProgTestRun(t)

	const teid = uint32(0x5A5A00e1)

	obj := loadN3N6Program(t)
	putForwardingUplinkPDR(t, obj, teid, 0)

	action := runXDP(t, obj.UpfEntryFunc, firstFragmentGPDU(teid))

	if action != ActionDrop {
		t.Fatalf("first fragment of a GTP-U transport got XDP action %d, want ActionDrop (%d): decapsulating it yields a truncated inner packet", action, ActionDrop)
	}

	if got := DropCount(obj, Uplink, "fragmented_transport"); got != 1 {
		t.Errorf("fragmented_transport = %d, want 1", got)
	}
}
