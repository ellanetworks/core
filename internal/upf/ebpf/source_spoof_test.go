// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package ebpf

import (
	"net/netip"
	"testing"
)

func putUplinkPDRSourceCheck(t *testing.T, obj *BpfObjects, teid uint32, ueV4, ueV6Prefix netip.Addr) {
	t.Helper()

	pdr := PdrInfo{
		OuterHeaderRemoval: 0,
		IMSI:               "001010000000001",
		Far:                FarInfo{Action: 0x02},
		Qer:                QerInfo{GateStatusUL: 0, MaxBitrateUL: 0},
		UEIPv4:             ueV4,
		UEIPv6Prefix:       ueV6Prefix,
	}
	if err := obj.PutPdrUplink(teid, pdr); err != nil {
		t.Fatalf("install uplink PDR: %v", err)
	}
}

func TestUplinkSourceOwnIPv4Accepted(t *testing.T) {
	requireProgTestRun(t)

	const teid = 0x5A010001

	obj := loadN3N6Program(t)
	putUplinkPDRSourceCheck(t, obj, teid, canonicalUEv4, canonicalUEv6Prefix)

	inner := ipv4Packet(canonicalUEv4.As4(), [4]byte{8, 8, 8, 8}, 17, udpDatagram(4000, 53, nil))

	action, _ := runXDPOut(t, obj.UpfEntryFunc, uplinkGPDU(teid, inner))

	if action == ActionDrop {
		t.Fatal("UE-own IPv4 source was dropped")
	}

	if got := DropCount(obj, Uplink, "source_spoof_ipv4"); got != 0 {
		t.Errorf("source_spoof_drop_ip4 = %d, want 0", got)
	}
}

func TestUplinkSourceSpoofedIPv4Dropped(t *testing.T) {
	requireProgTestRun(t)

	const teid = 0x5A010002

	obj := loadN3N6Program(t)
	putUplinkPDRSourceCheck(t, obj, teid, canonicalUEv4, canonicalUEv6Prefix)

	inner := ipv4Packet([4]byte{10, 0, 0, 99}, [4]byte{8, 8, 8, 8}, 17, udpDatagram(4000, 53, nil))

	action := runXDP(t, obj.UpfEntryFunc, uplinkGPDU(teid, inner))

	if action != ActionDrop {
		t.Fatalf("spoofed IPv4 source got action %d, want ActionDrop (%d)", action, ActionDrop)
	}

	if got := DropCount(obj, Uplink, "source_spoof_ipv4"); got != 1 {
		t.Errorf("source_spoof_drop_ip4 = %d, want 1", got)
	}
}

func TestUplinkSourceOwnIPv6DifferentIIDAccepted(t *testing.T) {
	requireProgTestRun(t)

	const teid = 0x5A010003

	obj := loadN3N6Program(t)
	putUplinkPDRSourceCheck(t, obj, teid, canonicalUEv4, canonicalUEv6Prefix)

	src := netip.MustParseAddr("2001:db8::dead:beef").As16()
	inner := ipv6Packet(src, netip.MustParseAddr("2001:4860:4860::8888").As16(), 17, udpDatagram(4000, 53, nil))

	action, _ := runXDPOut(t, obj.UpfEntryFunc, uplinkGPDU(teid, inner))

	if action == ActionDrop {
		t.Fatal("UE-own IPv6 /64 source (different IID) was dropped")
	}

	if got := DropCount(obj, Uplink, "source_spoof_ipv6"); got != 0 {
		t.Errorf("source_spoof_drop_ip6 = %d, want 0", got)
	}
}

func TestUplinkSourceSpoofedIPv6Dropped(t *testing.T) {
	requireProgTestRun(t)

	const teid = 0x5A010004

	obj := loadN3N6Program(t)
	putUplinkPDRSourceCheck(t, obj, teid, canonicalUEv4, canonicalUEv6Prefix)

	src := netip.MustParseAddr("2001:dead::9").As16()
	inner := ipv6Packet(src, netip.MustParseAddr("2001:4860:4860::8888").As16(), 17, udpDatagram(4000, 53, nil))

	action := runXDP(t, obj.UpfEntryFunc, uplinkGPDU(teid, inner))

	if action != ActionDrop {
		t.Fatalf("spoofed IPv6 source got action %d, want ActionDrop (%d)", action, ActionDrop)
	}

	if got := DropCount(obj, Uplink, "source_spoof_ipv6"); got != 1 {
		t.Errorf("source_spoof_drop_ip6 = %d, want 1", got)
	}
}

// TS 29.244 §5.16
func TestUplinkSourceFramedAccepted(t *testing.T) {
	requireProgTestRun(t)

	const teid = 0x5A010005

	obj := loadN3N6Program(t)
	putUplinkPDRSourceCheck(t, obj, teid, canonicalUEv4, canonicalUEv6Prefix)

	if err := obj.PutFramedDownlink(netip.MustParsePrefix("192.168.50.0/24"), canonicalUEv4); err != nil {
		t.Fatalf("install framed route: %v", err)
	}

	inner := ipv4Packet([4]byte{192, 168, 50, 5}, [4]byte{8, 8, 8, 8}, 17, udpDatagram(4000, 53, nil))

	action, _ := runXDPOut(t, obj.UpfEntryFunc, uplinkGPDU(teid, inner))

	if action == ActionDrop {
		t.Fatal("framed-subnet source was dropped")
	}

	if got := DropCount(obj, Uplink, "source_spoof_ipv4"); got != 0 {
		t.Errorf("source_spoof_drop_ip4 = %d, want 0", got)
	}
}

// TS 29.244 §5.16
func TestUplinkSourceFramedIPv6Accepted(t *testing.T) {
	requireProgTestRun(t)

	const teid = 0x5A01000A

	obj := loadN3N6Program(t)
	putUplinkPDRSourceCheck(t, obj, teid, canonicalUEv4, canonicalUEv6Prefix)

	if err := obj.PutFramedDownlink(netip.MustParsePrefix("fd00:beef::/48"), canonicalUEv6Prefix); err != nil {
		t.Fatalf("install framed route: %v", err)
	}

	src := netip.MustParseAddr("fd00:beef::5").As16()
	inner := ipv6Packet(src, netip.MustParseAddr("2001:4860:4860::8888").As16(), 17, udpDatagram(4000, 53, nil))

	action, _ := runXDPOut(t, obj.UpfEntryFunc, uplinkGPDU(teid, inner))

	if action == ActionDrop {
		t.Fatal("IPv6 framed-subnet source was dropped")
	}

	if got := DropCount(obj, Uplink, "source_spoof_ipv6"); got != 0 {
		t.Errorf("source_spoof_drop_ip6 = %d, want 0", got)
	}
}

func TestUplinkSourceFramedRejectedOtherSession(t *testing.T) {
	requireProgTestRun(t)

	const teid = 0x5A01000B

	obj := loadN3N6Program(t)
	putUplinkPDRSourceCheck(t, obj, teid, canonicalUEv4, canonicalUEv6Prefix)

	otherUE := netip.AddrFrom4([4]byte{10, 0, 0, 200})
	if err := obj.PutFramedDownlink(netip.MustParsePrefix("192.168.60.0/24"), otherUE); err != nil {
		t.Fatalf("install framed route: %v", err)
	}

	inner := ipv4Packet([4]byte{192, 168, 60, 5}, [4]byte{8, 8, 8, 8}, 17, udpDatagram(4000, 53, nil))

	action := runXDP(t, obj.UpfEntryFunc, uplinkGPDU(teid, inner))

	if action != ActionDrop {
		t.Fatalf("framed prefix owned by another session got action %d, want ActionDrop (%d)", action, ActionDrop)
	}

	if got := DropCount(obj, Uplink, "source_spoof_ipv4"); got != 1 {
		t.Errorf("source_spoof_drop_ip4 = %d, want 1", got)
	}
}

func TestUplinkSourceLinkLocalDropped(t *testing.T) {
	requireProgTestRun(t)

	const teid = 0x5A010006

	obj := loadN3N6Program(t)
	putUplinkPDRSourceCheck(t, obj, teid, canonicalUEv4, canonicalUEv6Prefix)

	src := netip.MustParseAddr("fe80::1").As16()
	inner := ipv6Packet(src, netip.MustParseAddr("2001:4860:4860::8888").As16(), 17, udpDatagram(4000, 53, nil))

	action := runXDP(t, obj.UpfEntryFunc, uplinkGPDU(teid, inner))

	if action != ActionDrop {
		t.Fatalf("link-local IPv6 source got action %d, want ActionDrop (%d)", action, ActionDrop)
	}

	if got := DropCount(obj, Uplink, "source_spoof_ipv6"); got != 1 {
		t.Errorf("source_spoof_drop_ip6 = %d, want 1", got)
	}
}

func TestUplinkSourceFailClosedMissingFamily(t *testing.T) {
	requireProgTestRun(t)

	t.Run("ipv4-only drops ipv6", func(t *testing.T) {
		const teid = 0x5A010007

		obj := loadN3N6Program(t)
		putUplinkPDRSourceCheck(t, obj, teid, canonicalUEv4, netip.Addr{})

		src := netip.MustParseAddr("2001:db8::9").As16()
		inner := ipv6Packet(src, netip.MustParseAddr("2001:4860:4860::8888").As16(), 17, udpDatagram(4000, 53, nil))

		if action := runXDP(t, obj.UpfEntryFunc, uplinkGPDU(teid, inner)); action != ActionDrop {
			t.Fatalf("IPv6 uplink on IPv4-only session got action %d, want ActionDrop", action)
		}

		if got := DropCount(obj, Uplink, "source_spoof_ipv6"); got != 1 {
			t.Errorf("source_spoof_drop_ip6 = %d, want 1", got)
		}
	})

	t.Run("ipv6-only drops ipv4", func(t *testing.T) {
		const teid = 0x5A010008

		obj := loadN3N6Program(t)
		putUplinkPDRSourceCheck(t, obj, teid, netip.Addr{}, canonicalUEv6Prefix)

		inner := ipv4Packet(canonicalUEv4.As4(), [4]byte{8, 8, 8, 8}, 17, udpDatagram(4000, 53, nil))

		if action := runXDP(t, obj.UpfEntryFunc, uplinkGPDU(teid, inner)); action != ActionDrop {
			t.Fatalf("IPv4 uplink on IPv6-only session got action %d, want ActionDrop", action)
		}

		if got := DropCount(obj, Uplink, "source_spoof_ipv4"); got != 1 {
			t.Errorf("source_spoof_drop_ip4 = %d, want 1", got)
		}
	})
}

func TestUplinkSpoofDropNoFlowEntry(t *testing.T) {
	requireProgTestRun(t)

	const teid = 0x5A010009

	obj := loadProgramConfig(t, true, false, 0, 1, 0, 0)
	putUplinkPDRSourceCheck(t, obj, teid, canonicalUEv4, canonicalUEv6Prefix)

	inner := ipv4Packet([4]byte{10, 0, 0, 99}, [4]byte{8, 8, 8, 8}, 17, udpDatagram(4000, 53, nil))

	if action := runXDP(t, obj.UpfEntryFunc, uplinkGPDU(teid, inner)); action != ActionDrop {
		t.Fatalf("spoofed packet got action %d, want ActionDrop", action)
	}

	var (
		key   N3N6EntrypointFlow
		value N3N6EntrypointFlowStats
		iter  = obj.FlowStats.Iterate()
		count int
	)

	for iter.Next(&key, &value) {
		count++
	}

	if err := iter.Err(); err != nil {
		t.Fatalf("iterate flow_stats: %v", err)
	}

	if count != 0 {
		t.Errorf("flow_stats has %d entries after a spoof drop, want 0", count)
	}
}
