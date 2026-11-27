// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

package ebpf

import (
	"net/netip"
	"testing"
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

	entry, err := f.obj.InterfaceEntry(f.n3Dev.Index)
	if err != nil {
		t.Fatalf("load N3 interface entry: %v", err)
	}

	runVLANVRFPath(t, entry, member.Index, frame, ActionRedirect)

	again, err := f.obj.InterfaceEntry(f.n3Dev.Index)
	if err != nil || again != entry {
		t.Fatalf("second InterfaceEntry = %v, %v; want the loaded entry", again, err)
	}

	f.obj.ReleaseInterface(f.n3Dev.Index)

	reloaded, err := f.obj.InterfaceEntry(f.n3Dev.Index)
	if err != nil || reloaded == entry {
		t.Fatalf("InterfaceEntry after release = %v, %v; want a new entry", reloaded, err)
	}
}
