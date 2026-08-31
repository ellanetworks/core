// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package ebpf

import (
	"bytes"
	"net/netip"
	"testing"
)

var framedUEIP = netip.AddrFrom4([4]byte{10, 0, 0, 1})

// TS 23.501 §5.6.14
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

	if action == ActionAborted {
		t.Fatal("framed-route downlink packet got ActionAborted; encapsulation failed")
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

// TS 23.501 §5.6.14
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

	if action == ActionAborted {
		t.Fatal("framed-route IPv6 downlink got ActionAborted; encapsulation failed")
	}

	f := parseGTPv4Frame(t, out)

	if f.teid != teid {
		t.Errorf("GTP TEID = %#x, want %#x (framed route did not redirect to the session)", f.teid, uint32(teid))
	}

	if !bytes.Equal(f.inner, inner) {
		t.Errorf("inner IPv6 packet altered by framed-route encapsulation")
	}
}

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

	dropPdr := ipv4OuterDownlinkPDR(teid, local, remote, qfi)
	dropPdr.Far.Action = 0x01

	if err := obj.PutPdrDownlink(framedUEIP, dropPdr); err != nil {
		t.Fatalf("install drop downlink PDR: %v", err)
	}

	if err := obj.PutFramedDownlink(netip.MustParsePrefix("192.168.50.0/24"), framedUEIP); err != nil {
		t.Fatalf("install framed route: %v", err)
	}

	dst := [4]byte{192, 168, 50, 9}
	inner := ipv4Packet([4]byte{8, 8, 8, 8}, dst, 17, udpDatagram(4000, 4001, []byte{0xde, 0xad, 0xbe, 0xef}))

	action := runXDP(t, obj.UpfEntryFunc, ethFrame(0x0800, inner))
	if action == ActionTx || action == ActionRedirect {
		t.Fatalf("framed downlink forwarded while owning FAR was drop (action %d)", action)
	}

	if err := obj.PutPdrDownlink(framedUEIP, ipv4OuterDownlinkPDR(teid, local, remote, qfi)); err != nil {
		t.Fatalf("update downlink PDR to forward: %v", err)
	}

	action, out := runXDPOut(t, obj.UpfEntryFunc, ethFrame(0x0800, inner))
	if action == ActionAborted {
		t.Fatal("framed-route downlink got ActionAborted after FAR became forward")
	}

	f := parseGTPv4Frame(t, out)
	if f.teid != teid {
		t.Errorf("GTP TEID = %#x, want %#x (framed route did not follow the updated downlink PDR)", f.teid, uint32(teid))
	}
}

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

	if action == ActionAborted {
		t.Fatal("framed-route downlink after reload got ActionAborted")
	}

	f := parseGTPv4Frame(t, out)

	if f.teid != teid {
		t.Errorf("GTP TEID = %#x, want %#x (framed route lost across reload)", f.teid, uint32(teid))
	}
}

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

	if action != ActionPass {
		t.Fatalf("unmatched downlink got XDP action %d, want ActionPass (%d)", action, ActionPass)
	}
}
