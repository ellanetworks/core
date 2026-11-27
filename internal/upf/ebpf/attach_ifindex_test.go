// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

package ebpf

import (
	"context"
	"net/netip"
	"os/exec"
	"slices"
	"testing"
	"time"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
)

func TestEntryIdentifiesInterfaceByAttachment(t *testing.T) {
	const (
		teid      = 0x41545431
		memberDev = "ellattm0"
		peerDev   = "ellattp0"
	)

	requireProgTestRun(t)

	f := setupT2(t, false)

	_, _ = ipCmd("link", "del", memberDev)
	addVethPair(t, memberDev, peerDev)

	member := ifByName(t, memberDev)

	putForwardingUplinkPDRUE(t, f.obj, teid, 0, netip.AddrFrom4(ueIP), netip.Addr{})

	frame := uplinkGPDU(teid, ipv4Packet(ueIP, serverIP, 17, udpDatagram(4000, 53, nil)))

	runVLANVRFPath(t, f.obj.UpfEntryFunc, member.Index, frame, ActionPass)

	entry, created, err := f.obj.InterfaceEntry(f.n3Dev.Index)
	if err != nil || !created {
		t.Fatalf("InterfaceEntry = %v, created %t; want a newly loaded entry", err, created)
	}

	runVLANVRFPath(t, entry, member.Index, frame, ActionRedirect)

	again, created, err := f.obj.InterfaceEntry(f.n3Dev.Index)
	if err != nil || created || again != entry {
		t.Fatalf("second InterfaceEntry = %v, created %t, %v; want the loaded entry", again, created, err)
	}

	requireSharedMap(t, f.obj.interfaces[uint32(f.n3Dev.Index)].UpfUplinkFunc, f.obj.RuntimeSettingsMap)

	release, err := f.obj.ReloadPrograms()
	if err != nil {
		t.Fatalf("ReloadPrograms: %v", err)
	}

	release()

	reloaded, created, err := f.obj.InterfaceEntry(f.n3Dev.Index)
	if err != nil || created || reloaded == entry {
		t.Fatalf("InterfaceEntry after reload = %v, created %t, %v; want the reloaded entry", reloaded, created, err)
	}

	runVLANVRFPath(t, reloaded, member.Index, frame, ActionRedirect)
	requireSharedMap(t, f.obj.interfaces[uint32(f.n3Dev.Index)].UpfUplinkFunc, f.obj.RuntimeSettingsMap)

	f.obj.ReleaseInterface(f.n3Dev.Index)

	if _, created, err := f.obj.InterfaceEntry(f.n3Dev.Index); err != nil || !created {
		t.Fatalf("InterfaceEntry after release = %v, created %t; want a newly loaded entry", err, created)
	}
}

func requireSharedMap(t *testing.T, prog *ebpf.Program, m *ebpf.Map) {
	t.Helper()

	progInfo, err := prog.Info()
	if err != nil {
		t.Fatalf("program info: %v", err)
	}

	mapIDs, ok := progInfo.MapIDs()
	if !ok {
		t.Skip("kernel does not report program map IDs")
	}

	mapInfo, err := m.Info()
	if err != nil {
		t.Fatalf("map info: %v", err)
	}

	mapID, ok := mapInfo.ID()
	if !ok {
		t.Skip("kernel does not report map IDs")
	}

	if !slices.Contains(mapIDs, mapID) {
		t.Fatalf("program uses maps %v, want the shared map %d among them", mapIDs, mapID)
	}
}

func TestNativeXDPOnBond(t *testing.T) {
	const (
		teid      = 0x424f4e44
		bondDev   = "ellbond0"
		memberDev = "ellbondm0"
		peerDev   = "ellbondp0"
		n6Dev     = "ellbondn6"
		n6PeerDev = "ellbondn6p"
	)

	requireProgTestRun(t)

	if testAttachModeTCX() {
		t.Skip("native XDP only")
	}

	_, _ = ipCmd("link", "del", bondDev)
	_, _ = ipCmd("link", "del", memberDev)
	_, _ = ipCmd("link", "del", n6Dev)

	if out, err := ipCmd("link", "add", bondDev, "type", "bond", "mode", "active-backup", "miimon", "100"); err != nil {
		t.Fatalf("create bond: %v: %s", err, out)
	}

	t.Cleanup(func() { _, _ = ipCmd("link", "del", bondDev) })

	addVethPair(t, memberDev, peerDev, func() {
		if out, err := ipCmd("link", "set", memberDev, "master", bondDev); err != nil {
			t.Fatalf("enslave %s: %v: %s", memberDev, err, out)
		}
	})

	if out, err := ipCmd("link", "set", bondDev, "up"); err != nil {
		t.Fatalf("set bond up: %v: %s", err, out)
	}

	addAddr(t, bondDev, addrCIDR(testUPFN3IP))
	addNeigh(t, bondDev, testGNBIP, "02:00:00:00:00:aa")

	addVethPair(t, n6Dev, n6PeerDev)
	addAddr(t, n6Dev, addrCIDR(natPublicIP))
	addRoute(t, "198.51.100.0/24", n6Dev, natPublicIP)
	addNeigh(t, n6Dev, serverIP, "02:00:00:00:00:bb")

	if out, err := exec.CommandContext(context.Background(), "ethtool", "-K", n6PeerDev, "gro", "on").CombinedOutput(); err != nil {
		t.Fatalf("enable GRO on %s: %v: %s", n6PeerDev, err, out)
	}

	bond := ifByName(t, bondDev)
	n6 := ifByName(t, n6Dev)

	obj := loadAttachedProgramConfig(t, false, false, bond.Index, n6.Index, 0, 0)

	entry, _, err := obj.InterfaceEntry(bond.Index)
	if err != nil {
		t.Fatalf("InterfaceEntry: %v", err)
	}

	l, err := link.AttachXDP(link.XDPOptions{Program: entry, Interface: bond.Index, Flags: link.XDPDriverMode})
	if err != nil {
		t.Fatalf("attach native XDP to the bond: %v", err)
	}

	t.Cleanup(func() { _ = l.Close() })

	putForwardingUplinkPDRUE(t, obj, teid, 0, netip.AddrFrom4(ueIP), netip.Addr{})

	capture := openCapture(t, ifByName(t, n6PeerDev).Index)

	inject(t, ifByName(t, peerDev).Index, uplinkGPDU(teid, ipv4Packet(ueIP, serverIP, 17, udpDatagram(4000, 53, nil))))

	if fr := captureMatching(capture, time.Second, func(fr []byte) bool {
		return isInnerIPv4(fr, 17, serverIP)
	}); fr == nil {
		t.Fatal("uplink packet received on a bond member did not egress on N6")
	}
}
