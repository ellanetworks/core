// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package engine_test

import (
	"context"
	"errors"
	"net/netip"
	"testing"

	"github.com/cilium/ebpf"
	"github.com/ellanetworks/core/internal/models"
	upfebpf "github.com/ellanetworks/core/internal/upf/ebpf"
)

// TestDeleteSessionRemovesURR asserts that deleting a session removes its
// urr_map entry, keyed by (SEID, URR ID). The per-session key means a released
// session leaves no orphan counter for a later session to inherit. Requires root
// to load the eBPF maps.
func TestDeleteSessionRemovesURR(t *testing.T) {
	conn, obj := privilegedEngine(t)

	const (
		seid  = uint64(7)
		urrID = uint32(2)
	)

	ueAddr := netip.MustParseAddr("10.0.0.1")

	state := &models.SessionState{
		SEID: seid,
		IMSI: "001010000000001",
		URRs: []models.URR{{URRID: urrID}},
		FARs: []models.FAR{{FARID: 1, ApplyAction: models.ApplyAction{Forw: true}}},
		PDRs: []models.PDR{{PDRID: 2, FARID: 1, URRID: urrID, PDI: models.PDI{UEIPAddress: ueAddr}}},
	}

	if _, err := conn.Apply(context.Background(), state); err != nil {
		t.Fatalf("establish: %v", err)
	}

	key := upfebpf.N3N6EntrypointUrrKey{Seid: seid, UrrId: urrID}

	var perCPU []uint64
	if err := obj.UrrMap.Lookup(key, &perCPU); err != nil {
		t.Fatalf("URR missing before delete: %v", err)
	}

	if err := conn.Delete(context.Background(), seid); err != nil {
		t.Fatalf("delete session: %v", err)
	}

	if err := obj.UrrMap.Lookup(key, &perCPU); !errors.Is(err, ebpf.ErrKeyNotExist) {
		t.Fatalf("urr_map entry still present after session delete; want ErrKeyNotExist, got %v", err)
	}
}

// TestApplyKeepsURRCounters asserts that restating a URR the session already
// holds leaves its counter alone. Creating one zeroes it, which would drop
// every byte accounted since the last poll on each apply.
func TestApplyKeepsURRCounters(t *testing.T) {
	conn, obj := privilegedEngine(t)

	const (
		seid  = uint64(8)
		urrID = uint32(1)
	)

	ueAddr := netip.MustParseAddr("10.0.0.2")

	state := &models.SessionState{
		SEID: seid,
		IMSI: "001010000000001",
		URRs: []models.URR{{URRID: urrID}},
		FARs: []models.FAR{{FARID: 1, ApplyAction: models.ApplyAction{Forw: true}}},
		PDRs: []models.PDR{{PDRID: 2, FARID: 1, URRID: urrID, PDI: models.PDI{UEIPAddress: ueAddr}}},
	}

	if _, err := conn.Apply(context.Background(), state); err != nil {
		t.Fatalf("establish: %v", err)
	}

	const accounted = uint64(4096)
	if err := obj.AddUrr(seid, urrID, accounted); err != nil {
		t.Fatalf("account bytes: %v", err)
	}

	if _, err := conn.Apply(context.Background(), state); err != nil {
		t.Fatalf("converge: %v", err)
	}

	got, err := obj.GetAndResetUrr(seid, urrID)
	if err != nil {
		t.Fatalf("read URR: %v", err)
	}

	if got != accounted {
		t.Fatalf("URR counter after a second apply = %d, want %d", got, accounted)
	}
}
