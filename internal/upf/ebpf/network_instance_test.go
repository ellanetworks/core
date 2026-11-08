// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ebpf

import (
	"net/netip"
	"testing"
)

func TestDownlinkOnAnotherNetworkInstanceIsDropped(t *testing.T) {
	requireProgTestRun(t)

	for _, tc := range []struct {
		name     string
		pdrNI    uint8
		wantDrop bool
	}{
		{name: "session on the default network instance", pdrNI: 0, wantDrop: true},
		{name: "session on the ingress network instance", pdrNI: 1, wantDrop: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			obj := loadProgram(t, 1, 0)

			if err := obj.PutNetworkInstance(NetworkInstance{ID: 1, Ifindex: 1, RoutingIfindex: 1}); err != nil {
				t.Fatalf("PutNetworkInstance: %v", err)
			}

			ueIP := [4]byte{10, 45, 0, 2}
			pdr := ipv4OuterDownlinkPDR(0x55667788, [4]byte{192, 168, 100, 1}, [4]byte{192, 168, 100, 9}, 9)
			pdr.NIID = tc.pdrNI

			if err := obj.PutPdrDownlink(netip.AddrFrom4(ueIP), pdr); err != nil {
				t.Fatalf("install downlink PDR: %v", err)
			}

			before := DropCount(obj, Downlink, "ni_mismatch")

			inner := ipv4Packet([4]byte{8, 8, 8, 8}, ueIP, 17, udpDatagram(4000, 4001, []byte{0xde, 0xad}))
			action, _ := runXDPOut(t, obj.UpfEntryFunc, ethFrame(0x0800, inner))

			dropped := DropCount(obj, Downlink, "ni_mismatch") > before
			if dropped != tc.wantDrop {
				t.Fatalf("ni_mismatch drop = %t (action %d), want %t", dropped, action, tc.wantDrop)
			}
		})
	}
}

func TestFrameOnAnInstanceInterfaceOutsideItsVLANIsNotForwarded(t *testing.T) {
	requireProgTestRun(t)

	const xdpPass = 2

	for _, tc := range []struct {
		name     string
		frame    func([]byte) []byte
		pdrNI    uint8
		wantPass bool
		wantDrop bool
	}{
		{name: "untagged", frame: func(l3 []byte) []byte { return ethFrame(0x0800, l3) }, wantPass: true},
		{name: "another VLAN", frame: func(l3 []byte) []byte { return vlanFrame(300, 0x0800, l3) }, wantPass: true},
		{name: "the instance VLAN to a default session", frame: func(l3 []byte) []byte { return vlanFrame(200, 0x0800, l3) }, wantDrop: true},
		{name: "the instance VLAN to an instance session", frame: func(l3 []byte) []byte { return vlanFrame(200, 0x0800, l3) }, pdrNI: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			obj := loadProgram(t, 11, 12)

			if err := obj.PutNetworkInstance(NetworkInstance{ID: 1, Ifindex: 1, RoutingIfindex: 1, Vlan: 200}); err != nil {
				t.Fatalf("PutNetworkInstance: %v", err)
			}

			ueIP := [4]byte{10, 45, 0, 2}
			pdr := ipv4OuterDownlinkPDR(0x55667788, [4]byte{192, 168, 100, 1}, [4]byte{192, 168, 100, 9}, 9)
			pdr.NIID = tc.pdrNI

			if err := obj.PutPdrDownlink(netip.AddrFrom4(ueIP), pdr); err != nil {
				t.Fatalf("install downlink PDR: %v", err)
			}

			before := DropCount(obj, Downlink, "ni_mismatch")

			inner := ipv4Packet([4]byte{8, 8, 8, 8}, ueIP, 17, udpDatagram(4000, 4001, []byte{0xde, 0xad}))
			action, _ := runXDPOut(t, obj.UpfEntryFunc, tc.frame(inner))

			if tc.wantPass && action != xdpPass {
				t.Fatalf("action = %d, want the frame passed to the kernel", action)
			}

			if dropped := DropCount(obj, Downlink, "ni_mismatch") > before; dropped != tc.wantDrop {
				t.Fatalf("ni_mismatch drop = %t (action %d), want %t", dropped, action, tc.wantDrop)
			}
		})
	}
}

func TestUplinkOnANetworkInstanceTheKernelWillNotRouteIsDropped(t *testing.T) {
	requireProgTestRun(t)

	const dev = "ellnifwd"

	_, _ = ipCmd("link", "del", dev)
	ipCmdOK(t, "link", "add", dev, "type", "dummy")
	t.Cleanup(func() { _, _ = ipCmd("link", "del", dev) })
	ipCmdOK(t, "link", "set", dev, "up")

	for _, key := range []string{"net.ipv4.conf." + dev + ".forwarding", "net.ipv6.conf." + dev + ".forwarding"} {
		if err := writeSysctl(key, "0"); err != nil {
			t.Fatalf("disable forwarding: %v", err)
		}
	}

	ifindex := uint32(ifByName(t, dev).Index)

	for _, tc := range []struct {
		name  string
		inner []byte
	}{
		{name: "ipv4", inner: innerIPv4UDP([4]byte{8, 8, 8, 8}, 53)},
		{name: "ipv6", inner: ipv6Packet(testUEv6, [16]byte{0x20, 0x01, 0x48, 0x60, 15: 0x88}, 17,
			udpDatagramChecksummedV6(testUEv6, [16]byte{0x20, 0x01, 0x48, 0x60, 15: 0x88}, 1234, 53, []byte{0xde, 0xad}))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			const teid = 0x31323334

			obj := loadN3N6Program(t)

			if err := obj.PutNetworkInstance(NetworkInstance{ID: 1, Ifindex: ifindex, RoutingIfindex: ifindex}); err != nil {
				t.Fatalf("PutNetworkInstance: %v", err)
			}

			pdr := PdrInfo{
				IMSI:         "001010000000001",
				Far:          FarInfo{Action: 0x02},
				UEIPv4:       canonicalUEv4,
				UEIPv6Prefix: canonicalUEv6Prefix,
				NIID:         1,
			}
			if err := obj.PutPdrUplink(teid, pdr); err != nil {
				t.Fatalf("install uplink PDR: %v", err)
			}

			action, _ := runXDPOut(t, obj.UpfEntryFunc, uplinkGPDU(teid, tc.inner))

			if action == ActionPass {
				t.Fatalf("decapsulated packet passed to the kernel on a network instance it cannot route")
			}

			if got := DropCount(obj, Uplink, "ni_unrouted"); got != 1 {
				t.Fatalf("ni_unrouted drops = %d (action %d), want 1", got, action)
			}
		})
	}
}
