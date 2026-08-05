// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package ebpf

import (
	"encoding/binary"
	"net/netip"
	"testing"
	"time"
)

// TestDownlinkFragmentRecordsWirePorts: the downlink fragment record must key on
// the ports the datagram carried on the wire, as it already does for addresses.
// Port preservation normally makes the two identical, which hides a mismatch, so
// this occupies the preferred NAT tuple first to force a different port.
func TestDownlinkFragmentRecordsWirePorts(t *testing.T) {
	requireProgTestRun(t)

	const (
		ulTEID = 0x4E415450
		dlTEID = 0x4E415451
		qfi    = 7
		ueSP   = 1250
		srvDP  = 80
		srvID  = 0xAB01
	)

	f := setupT2(t, true)
	putForwardingUplinkPDRUE(t, f.obj, ulTEID, 0, netip.AddrFrom4(ueIP), netip.Addr{})
	putDownlinkPDR(t, f.obj, ueIP, dlTEID, testUPFN3IP, testGNBIP, qfi)

	// Occupy {natPublicIP, serverIP, ueSP, srvDP} with a foreign flow so the
	// datapath cannot preserve ueSP and must allocate a different port.
	squat := natFiveTuple(natPublicIP, serverIP, ueSP, srvDP, 6)
	foreign := natFiveTuple([4]byte{10, 99, 99, 99}, serverIP, 9999, srvDP, 6)

	if err := f.obj.NatCt.Put(&squat, &N3N6EntrypointNatEntry{Peer: foreign}); err != nil {
		t.Fatalf("seed nat_ct: %v", err)
	}

	n6 := f.captureN6(t)

	f.injectUplink(t, uplinkGPDU(ulTEID,
		ipv4Packet(ueIP, serverIP, 6, tcpSegmentChecksummed(ueIP, serverIP, ueSP, srvDP, bytesOf(40)))))

	out := captureMatching(n6, time.Second, func(fr []byte) bool {
		return isInnerIPv4(fr, 6, serverIP)
	})
	if out == nil {
		t.Fatal("uplink did not egress on N6")
	}

	// The port the world sees, which downlink replies will be addressed to.
	wirePort := binary.BigEndian.Uint16(out[ethHdrLen+20 : ethHdrLen+22])
	t.Logf("UE port %d -> wire port %d", ueSP, wirePort)

	if wirePort == ueSP {
		t.Skip("NAT preserved the port despite the squat; test cannot distinguish")
	}

	capFD := f.captureN3(t)

	first := asFirstFragment(
		ipv4Packet(serverIP, natPublicIP, 6,
			tcpSegmentChecksummed(serverIP, natPublicIP, srvDP, wirePort, bytesOf(64))), srvID)
	later := asLaterFragment(
		ipv4Packet(serverIP, natPublicIP, 6, bytesOf(64)), srvID)

	encapsulated := func(fr []byte) bool {
		inner := gtpInner(fr)

		return len(inner) >= 20 && inner[9] == 6
	}

	f.injectDownlink(t, ethFrame(0x0800, first))

	if captureMatching(capFD, time.Second, encapsulated) == nil {
		t.Fatal("first fragment did not egress on N3")
	}

	// What got recorded: the wire port, or the post-NAT UE port?
	key := N3N6EntrypointFragKey4{
		Saddr: binary.NativeEndian.Uint32(serverIP[:]),
		Daddr: binary.NativeEndian.Uint32(natPublicIP[:]),
		Id:    htons(srvID),
		Proto: 6,
	}

	var ports N3N6EntrypointFragPorts
	if err := f.obj.FragPortsIp4.Lookup(&key, &ports); err != nil {
		t.Fatalf("frag_ports_ip4 lookup: %v", err)
	}

	// frag_record4 stores ctx->l4_dport, which the parser already converted
	// to host order, so no swap here.
	gotDport := ports.Dport
	if gotDport != wirePort {
		t.Errorf("recorded dport = %d, want the wire port %d (UE port is %d): the record was taken after destination NAT rewrote it",
			gotDport, wirePort, ueSP)
	}

	f.injectDownlink(t, ethFrame(0x0800, later))

	if captureMatching(capFD, time.Second, encapsulated) == nil {
		t.Error("later fragment did not egress on N3: its recovered ports did not match the NAT mapping")
	}
}

// TestDownlinkFragmentSurvivesSDFAfterNAT: a later fragment whose ports were
// recovered from the fragment map must still be filterable after destination
// NAT. Under the skb build the apply re-parses the frame, and a later fragment
// has no L4 header for the re-parse to read, so the recovered ports were lost
// and every port-constrained rule returned UNFILTERABLE.
func TestDownlinkFragmentSurvivesSDFAfterNAT(t *testing.T) {
	requireProgTestRun(t)

	const (
		ulTEID      = 0x46524744
		dlTEID      = 0x46524745
		qfi         = 7
		filterIndex = 3
		ueSP        = 1260
		srvDP       = 80
		srvID       = 0xBEE1
	)

	f := setupT2(t, true)
	putForwardingUplinkPDRUE(t, f.obj, ulTEID, 0, netip.AddrFrom4(ueIP), netip.Addr{})

	// A port-scoped allow for the server's port. It matches the datagram, so
	// the fragment must be filterable against it rather than refused for
	// being unreadable.
	putSDFFilter(t, f.obj, filterIndex, []SdfRule{
		sdfRuleIPv4(serverIP, 32, srvDP, srvDP, 6, SdfActionAllow),
	})
	putDownlinkPDRFiltered(t, f.obj, ueIP, dlTEID, testUPFN3IP, testGNBIP, qfi, filterIndex)

	n6 := f.captureN6(t)

	f.injectUplink(t, uplinkGPDU(ulTEID,
		ipv4Packet(ueIP, serverIP, 6, tcpSegmentChecksummed(ueIP, serverIP, ueSP, srvDP, bytesOf(40)))))

	out := captureMatching(n6, time.Second, func(fr []byte) bool {
		return isInnerIPv4(fr, 6, serverIP)
	})
	if out == nil {
		t.Fatal("uplink did not egress on N6")
	}

	wirePort := binary.BigEndian.Uint16(out[ethHdrLen+20 : ethHdrLen+22])

	capFD := f.captureN3(t)

	encapsulated := func(fr []byte) bool {
		inner := gtpInner(fr)

		return len(inner) >= 20 && inner[9] == 6
	}

	f.injectDownlink(t, ethFrame(0x0800, asFirstFragment(
		ipv4Packet(serverIP, natPublicIP, 6,
			tcpSegmentChecksummed(serverIP, natPublicIP, srvDP, wirePort, bytesOf(64))), srvID)))

	if captureMatching(capFD, time.Second, encapsulated) == nil {
		t.Fatal("first fragment did not egress on N3")
	}

	before := DropCount(f.obj, Downlink, "fragment_unfilterable")

	f.injectDownlink(t, ethFrame(0x0800, asLaterFragment(
		ipv4Packet(serverIP, natPublicIP, 6, bytesOf(64)), srvID)))

	if captureMatching(capFD, time.Second, encapsulated) == nil {
		t.Errorf("later fragment did not egress on N3 (fragment_unfilterable %d -> %d): its recovered ports were discarded by the NAT re-parse",
			before, DropCount(f.obj, Downlink, "fragment_unfilterable"))
	}
}
