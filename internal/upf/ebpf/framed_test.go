// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package ebpf

import (
	"bytes"
	"net/netip"
	"testing"
)

// framedUEIP is the owning session's downlink UE address that framed routes in
// these tests redirect to.
var framedUEIP = netip.AddrFrom4([4]byte{10, 0, 0, 1})

// TestFramedRouteDownlinkIPv4 checks that a downlink packet destined to an
// address inside a framed route (not a UE address) misses the exact-match table,
// redirects through the framed LPM table to the owning UE's downlink PDR, and is
// encapsulated toward that session (TS 23.501 §5.6.14, TS 29.244 §5.16).
func TestFramedRouteDownlinkIPv4(t *testing.T) {
	requireProgTestRun(t)

	obj := loadProgram(t, 1, 0)

	var (
		local  = [4]byte{192, 168, 100, 1}
		remote = [4]byte{192, 168, 100, 9}
	)

	const (
		teid = 0x66667788
		qfi  = 9
	)

	if err := obj.PutPdrDownlink(framedUEIP, ipv4OuterDownlinkPDR(teid, local, remote, qfi)); err != nil {
		t.Fatalf("install downlink PDR: %v", err)
	}

	if err := obj.PutFramedDownlink(netip.MustParsePrefix("192.168.50.0/24"), framedUEIP); err != nil {
		t.Fatalf("install framed route: %v", err)
	}

	dst := [4]byte{192, 168, 50, 9}
	inner := ipv4Packet([4]byte{8, 8, 8, 8}, dst, 17, udpDatagram(4000, 4001, []byte{0xde, 0xad, 0xbe, 0xef}))

	action, out := runXDPOut(t, obj.UpfEntryFunc, ethFrame(0x0800, inner))

	if action == XDP_ABORTED {
		t.Fatal("framed-route downlink packet got XDP_ABORTED; encapsulation failed")
	}

	f := parseGTPv4Frame(t, out)

	if f.teid != teid {
		t.Errorf("GTP TEID = %#x, want %#x (framed route did not reuse the session)", f.teid, uint32(teid))
	}

	if f.qfi != qfi {
		t.Errorf("QFI = %d, want %d", f.qfi, qfi)
	}

	if !bytes.Equal(f.inner, inner) {
		t.Errorf("inner packet altered by framed-route encapsulation")
	}
}

// TestFramedRouteDownlinkIPv6 checks the IPv6 datapath: a downlink packet
// destined inside a framed prefix misses the UE /64 exact-match table, redirects
// through the framed LPM to the owning UE prefix's downlink PDR, and is
// encapsulated toward that session (TS 23.501 §5.6.14, TS 29.244 §5.16).
func TestFramedRouteDownlinkIPv6(t *testing.T) {
	requireProgTestRun(t)

	obj := loadProgram(t, 1, 0)

	uePrefix := netip.MustParseAddr("2001:db8:1::")
	local := [4]byte{192, 168, 100, 1}
	remote := [4]byte{192, 168, 100, 9}

	const (
		teid = 0x0A0B0C0D
		qfi  = 5
	)

	putDownlinkPDRv6UE(t, obj, uePrefix, teid, local, remote, qfi)

	framed := netip.MustParsePrefix("fd00:beef::/48")
	if err := obj.PutFramedDownlink(framed, uePrefix); err != nil {
		t.Fatalf("install framed route: %v", err)
	}

	server := netip.MustParseAddr("2001:4860:4860::8888").As16()
	dst := netip.MustParseAddr("fd00:beef::9").As16()
	inner := ipv6Packet(server, dst, 17, udpDatagram(4000, 4001, []byte{0xde, 0xad}))

	action, out := runXDPOut(t, obj.UpfEntryFunc, ethFrame(0x86DD, inner))

	if action == XDP_ABORTED {
		t.Fatal("framed-route IPv6 downlink got XDP_ABORTED; encapsulation failed")
	}

	f := parseGTPv4Frame(t, out)

	if f.teid != teid {
		t.Errorf("GTP TEID = %#x, want %#x (framed route did not redirect to the session)", f.teid, uint32(teid))
	}

	if !bytes.Equal(f.inner, inner) {
		t.Errorf("inner IPv6 packet altered by framed-route encapsulation")
	}
}

// TestFramedRouteFollowsDownlinkPDRUpdate is the core guarantee of the redirect
// design: a framed route reflects a later change to the owning downlink PDR
// without being re-installed. The downlink FAR starts as drop (as at 5G
// establishment, before the gNB downlink F-TEID is known) and later becomes
// forward (session modification); the framed route must follow both.
func TestFramedRouteFollowsDownlinkPDRUpdate(t *testing.T) {
	requireProgTestRun(t)

	obj := loadProgram(t, 1, 0)

	var (
		local  = [4]byte{192, 168, 100, 1}
		remote = [4]byte{192, 168, 100, 9}
	)

	const (
		teid = 0x66667788
		qfi  = 9
	)

	// Downlink PDR present but not yet forwarding (FAR = drop).
	dropPdr := ipv4OuterDownlinkPDR(teid, local, remote, qfi)
	dropPdr.Far.Action = 0x01 // FAR_DROP

	if err := obj.PutPdrDownlink(framedUEIP, dropPdr); err != nil {
		t.Fatalf("install drop downlink PDR: %v", err)
	}

	if err := obj.PutFramedDownlink(netip.MustParsePrefix("192.168.50.0/24"), framedUEIP); err != nil {
		t.Fatalf("install framed route: %v", err)
	}

	dst := [4]byte{192, 168, 50, 9}
	inner := ipv4Packet([4]byte{8, 8, 8, 8}, dst, 17, udpDatagram(4000, 4001, []byte{0xde, 0xad, 0xbe, 0xef}))

	action := runXDP(t, obj.UpfEntryFunc, ethFrame(0x0800, inner))
	if action == XDP_TX || action == XDP_REDIRECT {
		t.Fatalf("framed downlink forwarded while owning FAR was drop (action %d)", action)
	}

	// Session modification flips the owning downlink PDR to forward. The framed
	// route is NOT touched.
	if err := obj.PutPdrDownlink(framedUEIP, ipv4OuterDownlinkPDR(teid, local, remote, qfi)); err != nil {
		t.Fatalf("update downlink PDR to forward: %v", err)
	}

	action, out := runXDPOut(t, obj.UpfEntryFunc, ethFrame(0x0800, inner))
	if action == XDP_ABORTED {
		t.Fatal("framed-route downlink got XDP_ABORTED after FAR became forward")
	}

	f := parseGTPv4Frame(t, out)
	if f.teid != teid {
		t.Errorf("GTP TEID = %#x, want %#x (framed route did not follow the updated downlink PDR)", f.teid, uint32(teid))
	}
}

// TestFramedRouteDownlinkSurvivesReload checks that framed forwarding still works
// after a map-preserving reload (LoadWithMapReplacements), as happens when NAT is
// toggled. Both the framed LPM map and the pdrs_downlink map it redirects to must
// be in the preserved set; otherwise the datapath reads a fresh empty map.
func TestFramedRouteDownlinkSurvivesReload(t *testing.T) {
	requireProgTestRun(t)

	obj := loadProgram(t, 1, 0)

	const (
		teid = 0x66667788
		qfi  = 9
	)

	var (
		local  = [4]byte{192, 168, 100, 1}
		remote = [4]byte{192, 168, 100, 9}
	)

	if err := obj.LoadWithMapReplacements(); err != nil {
		t.Fatalf("reload: %v", err)
	}

	if err := obj.PutPdrDownlink(framedUEIP, ipv4OuterDownlinkPDR(teid, local, remote, qfi)); err != nil {
		t.Fatalf("install downlink PDR: %v", err)
	}

	if err := obj.PutFramedDownlink(netip.MustParsePrefix("192.168.50.0/24"), framedUEIP); err != nil {
		t.Fatalf("install framed route: %v", err)
	}

	dst := [4]byte{192, 168, 50, 9}
	inner := ipv4Packet([4]byte{8, 8, 8, 8}, dst, 17, udpDatagram(4000, 4001, []byte{0xde, 0xad, 0xbe, 0xef}))

	action, out := runXDPOut(t, obj.UpfEntryFunc, ethFrame(0x0800, inner))

	if action == XDP_ABORTED {
		t.Fatal("framed-route downlink after reload got XDP_ABORTED")
	}

	f := parseGTPv4Frame(t, out)

	if f.teid != teid {
		t.Errorf("GTP TEID = %#x, want %#x (framed route lost across reload)", f.teid, uint32(teid))
	}
}

// TestFramedRouteDownlinkMiss checks that a downlink address matching neither a
// UE address nor any framed route is passed through unchanged.
func TestFramedRouteDownlinkMiss(t *testing.T) {
	requireProgTestRun(t)

	obj := loadProgram(t, 1, 0)

	if err := obj.PutPdrDownlink(framedUEIP, ipv4OuterDownlinkPDR(0x1234, [4]byte{192, 168, 100, 1}, [4]byte{192, 168, 100, 9}, 9)); err != nil {
		t.Fatalf("install downlink PDR: %v", err)
	}

	if err := obj.PutFramedDownlink(netip.MustParsePrefix("192.168.50.0/24"), framedUEIP); err != nil {
		t.Fatalf("install framed route: %v", err)
	}

	inner := ipv4Packet([4]byte{8, 8, 8, 8}, [4]byte{203, 0, 113, 5}, 17, udpDatagram(4000, 4001, nil))

	action := runXDP(t, obj.UpfEntryFunc, ethFrame(0x0800, inner))

	if action != XDP_PASS {
		t.Fatalf("unmatched downlink got XDP action %d, want XDP_PASS (%d)", action, XDP_PASS)
	}
}
