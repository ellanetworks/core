// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package upf

import (
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/upf/ebpf"
)

func natTuple(saddr uint32, sport uint16, proto uint16) ebpf.N3N6EntrypointFiveTuple {
	return ebpf.N3N6EntrypointFiveTuple{
		Saddr: saddr,
		Daddr: 0x0a0a0a0a,
		Sport: sport,
		Dport: 80,
		Proto: proto,
	}
}

// pair returns a UE-side and NAT-side entry pair with the given age.
func pair(ueKey, natKey ebpf.N3N6EntrypointFiveTuple, nowNs uint64, age time.Duration, state, replied uint8) (ebpf.N3N6EntrypointNatEntry, ebpf.N3N6EntrypointNatEntry) {
	ts := nowNs - uint64(age.Nanoseconds())

	return ebpf.N3N6EntrypointNatEntry{Src: natKey, RefreshTs: ts, State: state, Replied: replied, UeSide: 1},
		ebpf.N3N6EntrypointNatEntry{Src: ueKey, RefreshTs: ts}
}

func TestNatEntryTimeout(t *testing.T) {
	cases := []struct {
		name    string
		proto   uint16
		state   uint8
		replied uint8
		want    time.Duration
	}{
		{"tcp new", protoTCP, 0, 0, natTCPTransitoryTimeout},
		{"tcp established", protoTCP, natStateEstablished, 1, natTCPEstablishedTimeout},
		{"tcp closing", protoTCP, natStateClosing, 1, natTCPClosingTimeout},
		{"udp unreplied", protoUDP, 0, 0, natUDPUnrepliedTimeout},
		{"udp replied", protoUDP, 0, 1, natUDPRepliedTimeout},
		{"icmp", protoICMP, 0, 1, natICMPTimeout},
		{"unknown proto", 132, 0, 0, ConnTrackTimeout},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := natEntryTimeout(tc.proto, tc.state, tc.replied); got != tc.want {
				t.Errorf("natEntryTimeout(%d, %d, %d) = %v, want %v", tc.proto, tc.state, tc.replied, got, tc.want)
			}
		})
	}
}

func TestNatExpiredKeysPairsAndTimeouts(t *testing.T) {
	nowNs := uint64(100 * time.Hour.Nanoseconds())

	freshUEKey := natTuple(1, 1000, protoTCP)
	freshNATKey := natTuple(100, 1000, protoTCP)
	freshUE, freshNAT := pair(freshUEKey, freshNATKey, nowNs, 2*time.Hour, natStateEstablished, 1)

	deadUEKey := natTuple(2, 2000, protoTCP)
	deadNATKey := natTuple(100, 2000, protoTCP)
	deadUE, deadNAT := pair(deadUEKey, deadNATKey, nowNs, 3*time.Hour, natStateEstablished, 1)

	closingUEKey := natTuple(3, 3000, protoTCP)
	closingNATKey := natTuple(100, 3000, protoTCP)
	closingUE, closingNAT := pair(closingUEKey, closingNATKey, nowNs, 30*time.Second, natStateClosing, 1)

	udpProbeUEKey := natTuple(4, 4000, protoUDP)
	udpProbeNATKey := natTuple(100, 4000, protoUDP)
	udpProbeUE, udpProbeNAT := pair(udpProbeUEKey, udpProbeNATKey, nowNs, time.Minute, 0, 0)

	udpRepliedUEKey := natTuple(5, 5000, protoUDP)
	udpRepliedNATKey := natTuple(100, 5000, protoUDP)
	udpRepliedUE, udpRepliedNAT := pair(udpRepliedUEKey, udpRepliedNATKey, nowNs, time.Minute, 0, 1)

	snapshot := map[ebpf.N3N6EntrypointFiveTuple]ebpf.N3N6EntrypointNatEntry{
		freshUEKey:       freshUE,
		freshNATKey:      freshNAT,
		deadUEKey:        deadUE,
		deadNATKey:       deadNAT,
		closingUEKey:     closingUE,
		closingNATKey:    closingNAT,
		udpProbeUEKey:    udpProbeUE,
		udpProbeNATKey:   udpProbeNAT,
		udpRepliedUEKey:  udpRepliedUE,
		udpRepliedNATKey: udpRepliedNAT,
	}

	got := make(map[ebpf.N3N6EntrypointFiveTuple]bool)
	for _, k := range natExpiredKeys(snapshot, nowNs) {
		got[k] = true
	}

	want := map[ebpf.N3N6EntrypointFiveTuple]bool{
		deadUEKey:      true, // 3h > 7440s established timeout; pair goes together
		deadNATKey:     true,
		closingUEKey:   true, // 30s > 10s closing timeout
		closingNATKey:  true,
		udpProbeUEKey:  true, // 60s > 30s unreplied timeout
		udpProbeNATKey: true,
	}

	for k := range want {
		if !got[k] {
			t.Errorf("key %+v missing from expired set", k)
		}
	}

	for k := range got {
		if !want[k] {
			t.Errorf("key %+v unexpectedly expired", k)
		}
	}
}

func TestNatExpiredKeysOrphans(t *testing.T) {
	nowNs := uint64(100 * time.Hour.Nanoseconds())

	ueKey := natTuple(1, 1000, protoTCP)

	// NAT-side entry with no partner, past the grace period.
	staleOrphanKey := natTuple(100, 1000, protoTCP)
	staleOrphan := ebpf.N3N6EntrypointNatEntry{Src: ueKey, RefreshTs: nowNs - uint64((2 * natOrphanGrace).Nanoseconds())}

	// NAT-side entry with no partner, inside the grace period (a pair whose
	// second insert is in flight must not be reaped).
	newOrphanKey := natTuple(100, 2000, protoTCP)
	newOrphan := ebpf.N3N6EntrypointNatEntry{Src: natTuple(2, 2000, protoTCP), RefreshTs: nowNs - uint64(time.Second.Nanoseconds())}

	// NAT-side entry whose partner exists but references a different
	// NAT-side tuple (stale after a port remap): orphan.
	mismatchedKey := natTuple(100, 3000, protoTCP)
	mismatchedUEKey := natTuple(3, 3000, protoTCP)
	mismatched := ebpf.N3N6EntrypointNatEntry{Src: mismatchedUEKey, RefreshTs: nowNs - uint64((2 * natOrphanGrace).Nanoseconds())}
	remappedNATKey := natTuple(100, 3333, protoTCP)
	mismatchedUE := ebpf.N3N6EntrypointNatEntry{Src: remappedNATKey, RefreshTs: nowNs, UeSide: 1}
	remappedNAT := ebpf.N3N6EntrypointNatEntry{Src: mismatchedUEKey, RefreshTs: nowNs}

	snapshot := map[ebpf.N3N6EntrypointFiveTuple]ebpf.N3N6EntrypointNatEntry{
		staleOrphanKey:  staleOrphan,
		newOrphanKey:    newOrphan,
		mismatchedKey:   mismatched,
		mismatchedUEKey: mismatchedUE,
		remappedNATKey:  remappedNAT,
	}

	got := make(map[ebpf.N3N6EntrypointFiveTuple]bool)
	for _, k := range natExpiredKeys(snapshot, nowNs) {
		got[k] = true
	}

	if !got[staleOrphanKey] {
		t.Error("stale orphan NAT-side entry not reaped")
	}

	if got[newOrphanKey] {
		t.Error("NAT-side entry inside the grace period reaped")
	}

	if !got[mismatchedKey] {
		t.Error("NAT-side entry with mismatched partner not reaped")
	}

	if got[mismatchedUEKey] || got[remappedNATKey] {
		t.Error("live remapped pair reaped")
	}
}
