// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package ebpf

import (
	"net/netip"
	"testing"
)

// putUplinkPDRSourceCheck installs an uplink decap PDR carrying the given
// authorized UE source addresses. A zero netip.Addr leaves that family unset
// (fail closed).
func putUplinkPDRSourceCheck(t *testing.T, obj *BpfObjects, teid uint32, ueV4, ueV6Prefix netip.Addr) {
	t.Helper()

	pdr := PdrInfo{
		OuterHeaderRemoval: 0, // OHR_GTP_U_UDP_IPv4
		IMSI:               "001010000000001",
		Far:                FarInfo{Action: 0x02 /* FAR_FORW */},
		Qer:                QerInfo{GateStatusUL: 0 /* GATE_STATUS_OPEN */, MaxBitrateUL: 0 /* unlimited */},
		UEIPv4:             ueV4,
		UEIPv6Prefix:       ueV6Prefix,
	}
	if err := obj.PutPdrUplink(teid, pdr); err != nil {
		t.Fatalf("install uplink PDR: %v", err)
	}
}

// TestUplinkSourceOwnIPv4Accepted checks that an uplink packet sourced from the
// UE's own IPv4 address passes source validation (not spoof-dropped).
func TestUplinkSourceOwnIPv4Accepted(t *testing.T) {
	requireProgTestRun(t)

	const teid = 0x5A010001

	obj := loadN3N6Program(t)
	putUplinkPDRSourceCheck(t, obj, teid, canonicalUEv4, canonicalUEv6Prefix)

	inner := ipv4Packet(canonicalUEv4.As4(), [4]byte{8, 8, 8, 8}, 17, udpDatagram(4000, 53, nil))

	action, _ := runXDPOut(t, obj.UpfEntryFunc, uplinkGPDU(teid, inner))

	if action == XDP_DROP {
		t.Fatal("UE-own IPv4 source was dropped")
	}

	if got := GetN3SourceSpoofDropIPv4(obj); got != 0 {
		t.Errorf("source_spoof_drop_ip4 = %d, want 0", got)
	}
}

// TestUplinkSourceSpoofedIPv4Dropped checks that an uplink packet whose inner
// source is not the UE address (nor a framed prefix) is dropped and counted.
func TestUplinkSourceSpoofedIPv4Dropped(t *testing.T) {
	requireProgTestRun(t)

	const teid = 0x5A010002

	obj := loadN3N6Program(t)
	putUplinkPDRSourceCheck(t, obj, teid, canonicalUEv4, canonicalUEv6Prefix)

	// Source belongs to another subscriber, not this session's UE.
	inner := ipv4Packet([4]byte{10, 0, 0, 99}, [4]byte{8, 8, 8, 8}, 17, udpDatagram(4000, 53, nil))

	action := runXDP(t, obj.UpfEntryFunc, uplinkGPDU(teid, inner))

	if action != XDP_DROP {
		t.Fatalf("spoofed IPv4 source got action %d, want XDP_DROP (%d)", action, XDP_DROP)
	}

	if got := GetN3SourceSpoofDropIPv4(obj); got != 1 {
		t.Errorf("source_spoof_drop_ip4 = %d, want 1", got)
	}
}

// TestUplinkSourceOwnIPv6DifferentIIDAccepted checks that an uplink packet from
// any address inside the UE's /64 (a different interface identifier) is accepted
// — the UE forms multiple SLAAC addresses in its prefix.
func TestUplinkSourceOwnIPv6DifferentIIDAccepted(t *testing.T) {
	requireProgTestRun(t)

	const teid = 0x5A010003

	obj := loadN3N6Program(t)
	putUplinkPDRSourceCheck(t, obj, teid, canonicalUEv4, canonicalUEv6Prefix)

	src := netip.MustParseAddr("2001:db8::dead:beef").As16() // same /64, different IID
	inner := ipv6Packet(src, netip.MustParseAddr("2001:4860:4860::8888").As16(), 17, udpDatagram(4000, 53, nil))

	action, _ := runXDPOut(t, obj.UpfEntryFunc, uplinkGPDU(teid, inner))

	if action == XDP_DROP {
		t.Fatal("UE-own IPv6 /64 source (different IID) was dropped")
	}

	if got := GetN3SourceSpoofDropIPv6(obj); got != 0 {
		t.Errorf("source_spoof_drop_ip6 = %d, want 0", got)
	}
}

// TestUplinkSourceSpoofedIPv6Dropped checks that an uplink packet from a
// different /64 than the UE's is dropped and counted.
func TestUplinkSourceSpoofedIPv6Dropped(t *testing.T) {
	requireProgTestRun(t)

	const teid = 0x5A010004

	obj := loadN3N6Program(t)
	putUplinkPDRSourceCheck(t, obj, teid, canonicalUEv4, canonicalUEv6Prefix)

	src := netip.MustParseAddr("2001:dead::9").As16() // different /64
	inner := ipv6Packet(src, netip.MustParseAddr("2001:4860:4860::8888").As16(), 17, udpDatagram(4000, 53, nil))

	action := runXDP(t, obj.UpfEntryFunc, uplinkGPDU(teid, inner))

	if action != XDP_DROP {
		t.Fatalf("spoofed IPv6 source got action %d, want XDP_DROP (%d)", action, XDP_DROP)
	}

	if got := GetN3SourceSpoofDropIPv6(obj); got != 1 {
		t.Errorf("source_spoof_drop_ip6 = %d, want 1", got)
	}
}

// TestUplinkSourceFramedAccepted checks that an uplink packet sourced from a
// framed prefix owned by this session (not the UE address) is accepted via the
// framed LPM table (TS 29.244 §5.16).
func TestUplinkSourceFramedAccepted(t *testing.T) {
	requireProgTestRun(t)

	const teid = 0x5A010005

	obj := loadN3N6Program(t)
	putUplinkPDRSourceCheck(t, obj, teid, canonicalUEv4, canonicalUEv6Prefix)

	// The framed route is owned by this session's UE (same value as ue_ipv4).
	if err := obj.PutFramedDownlink(netip.MustParsePrefix("192.168.50.0/24"), canonicalUEv4); err != nil {
		t.Fatalf("install framed route: %v", err)
	}

	inner := ipv4Packet([4]byte{192, 168, 50, 5}, [4]byte{8, 8, 8, 8}, 17, udpDatagram(4000, 53, nil))

	action, _ := runXDPOut(t, obj.UpfEntryFunc, uplinkGPDU(teid, inner))

	if action == XDP_DROP {
		t.Fatal("framed-subnet source was dropped")
	}

	if got := GetN3SourceSpoofDropIPv4(obj); got != 0 {
		t.Errorf("source_spoof_drop_ip4 = %d, want 0", got)
	}
}

// TestUplinkSourceFramedIPv6Accepted checks that an uplink packet sourced from
// an IPv6 framed prefix owned by this session is accepted via the framed LPM
// table (TS 29.244 §5.16).
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

	if action == XDP_DROP {
		t.Fatal("IPv6 framed-subnet source was dropped")
	}

	if got := GetN3SourceSpoofDropIPv6(obj); got != 0 {
		t.Errorf("source_spoof_drop_ip6 = %d, want 0", got)
	}
}

// TestUplinkSourceFramedRejectedOtherSession checks that a framed prefix owned by
// a different session's UE is NOT authorized — the ownership compare is what
// makes the framed lookup a security check rather than a blanket bypass.
func TestUplinkSourceFramedRejectedOtherSession(t *testing.T) {
	requireProgTestRun(t)

	const teid = 0x5A01000B

	obj := loadN3N6Program(t)
	putUplinkPDRSourceCheck(t, obj, teid, canonicalUEv4, canonicalUEv6Prefix)

	// The framed route belongs to a different UE, not this session.
	otherUE := netip.AddrFrom4([4]byte{10, 0, 0, 200})
	if err := obj.PutFramedDownlink(netip.MustParsePrefix("192.168.60.0/24"), otherUE); err != nil {
		t.Fatalf("install framed route: %v", err)
	}

	inner := ipv4Packet([4]byte{192, 168, 60, 5}, [4]byte{8, 8, 8, 8}, 17, udpDatagram(4000, 53, nil))

	action := runXDP(t, obj.UpfEntryFunc, uplinkGPDU(teid, inner))

	if action != XDP_DROP {
		t.Fatalf("framed prefix owned by another session got action %d, want XDP_DROP (%d)", action, XDP_DROP)
	}

	if got := GetN3SourceSpoofDropIPv4(obj); got != 1 {
		t.Errorf("source_spoof_drop_ip4 = %d, want 1", got)
	}
}

// TestUplinkSourceLinkLocalDropped checks that a non-RS uplink packet with an
// IPv6 link-local source is dropped (RS is intercepted before this check).
func TestUplinkSourceLinkLocalDropped(t *testing.T) {
	requireProgTestRun(t)

	const teid = 0x5A010006

	obj := loadN3N6Program(t)
	putUplinkPDRSourceCheck(t, obj, teid, canonicalUEv4, canonicalUEv6Prefix)

	src := netip.MustParseAddr("fe80::1").As16()
	inner := ipv6Packet(src, netip.MustParseAddr("2001:4860:4860::8888").As16(), 17, udpDatagram(4000, 53, nil))

	action := runXDP(t, obj.UpfEntryFunc, uplinkGPDU(teid, inner))

	if action != XDP_DROP {
		t.Fatalf("link-local IPv6 source got action %d, want XDP_DROP (%d)", action, XDP_DROP)
	}

	if got := GetN3SourceSpoofDropIPv6(obj); got != 1 {
		t.Errorf("source_spoof_drop_ip6 = %d, want 1", got)
	}
}

// TestUplinkSourceFailClosedMissingFamily checks fail-closed behaviour: a session
// with only an IPv4 address drops an IPv6-sourced uplink packet, and vice versa.
func TestUplinkSourceFailClosedMissingFamily(t *testing.T) {
	requireProgTestRun(t)

	t.Run("ipv4-only drops ipv6", func(t *testing.T) {
		const teid = 0x5A010007

		obj := loadN3N6Program(t)
		putUplinkPDRSourceCheck(t, obj, teid, canonicalUEv4, netip.Addr{}) // no IPv6

		src := netip.MustParseAddr("2001:db8::9").As16()
		inner := ipv6Packet(src, netip.MustParseAddr("2001:4860:4860::8888").As16(), 17, udpDatagram(4000, 53, nil))

		if action := runXDP(t, obj.UpfEntryFunc, uplinkGPDU(teid, inner)); action != XDP_DROP {
			t.Fatalf("IPv6 uplink on IPv4-only session got action %d, want XDP_DROP", action)
		}

		if got := GetN3SourceSpoofDropIPv6(obj); got != 1 {
			t.Errorf("source_spoof_drop_ip6 = %d, want 1", got)
		}
	})

	t.Run("ipv6-only drops ipv4", func(t *testing.T) {
		const teid = 0x5A010008

		obj := loadN3N6Program(t)
		putUplinkPDRSourceCheck(t, obj, teid, netip.Addr{}, canonicalUEv6Prefix) // no IPv4

		inner := ipv4Packet(canonicalUEv4.As4(), [4]byte{8, 8, 8, 8}, 17, udpDatagram(4000, 53, nil))

		if action := runXDP(t, obj.UpfEntryFunc, uplinkGPDU(teid, inner)); action != XDP_DROP {
			t.Fatalf("IPv4 uplink on IPv6-only session got action %d, want XDP_DROP", action)
		}

		if got := GetN3SourceSpoofDropIPv4(obj); got != 1 {
			t.Errorf("source_spoof_drop_ip4 = %d, want 1", got)
		}
	})
}

// TestUplinkSpoofDropNoFlowEntry checks that a spoof drop does not create a
// flow-accounting entry (decision: a random-source spoof flood must not churn
// the LRU flow map).
func TestUplinkSpoofDropNoFlowEntry(t *testing.T) {
	requireProgTestRun(t)

	const teid = 0x5A010009

	obj := loadProgramConfig(t, true /* flowAccounting */, false, 0, 1, 0, 0)
	putUplinkPDRSourceCheck(t, obj, teid, canonicalUEv4, canonicalUEv6Prefix)

	inner := ipv4Packet([4]byte{10, 0, 0, 99}, [4]byte{8, 8, 8, 8}, 17, udpDatagram(4000, 53, nil))

	if action := runXDP(t, obj.UpfEntryFunc, uplinkGPDU(teid, inner)); action != XDP_DROP {
		t.Fatalf("spoofed packet got action %d, want XDP_DROP", action)
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
