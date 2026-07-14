// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package engine_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/rlimit"
	"github.com/ellanetworks/core/internal/models"
	upfebpf "github.com/ellanetworks/core/internal/upf/ebpf"
	"github.com/ellanetworks/core/internal/upf/engine"
)

// TestEstablishSessionRollsBackOnFailure asserts that a mid-establish failure
// unwinds every datapath change applied so far, leaving no orphaned URR or PDR
// entry — the session is never registered, so nothing else could reclaim them.
// Requires root to load the eBPF maps.
func TestEstablishSessionRollsBackOnFailure(t *testing.T) {
	if os.Geteuid() != 0 {
		const msg = "loading eBPF maps requires root/CAP_BPF"
		if os.Getenv("EBPF_REQUIRE_PRIVILEGED") != "" {
			t.Fatal(msg)
		}

		t.Skip(msg + "; skipping")
	}

	if err := rlimit.RemoveMemlock(); err != nil {
		t.Fatalf("cannot remove memlock rlimit: %v", err)
	}

	obj := upfebpf.NewBpfObjects(false, false, 1, 0, 0, 0)
	if err := obj.Load(); err != nil {
		t.Fatalf("load eBPF objects: %v", err)
	}

	t.Cleanup(func() { _ = obj.Close() })

	rm, err := engine.NewFteIDResourceManager(1024)
	if err != nil {
		t.Fatalf("new fteid resource manager: %v", err)
	}

	conn, err := engine.NewSessionEngine("1.2.3.4", "nodeId", "2.3.4.5", "", "2.3.4.5", "", obj, rm)
	if err != nil {
		t.Fatalf("new session engine: %v", err)
	}

	const seid = uint64(9)

	// PDR 1 is a valid uplink PDR (allocates a TEID, applies to pdrs_uplink); PDR
	// 2 has neither an F-TEID nor a UE IP, so ExtractPDR fails after PDR 1 and the
	// URR are already installed.
	req := &models.EstablishRequest{
		LocalSEID: seid,
		IMSI:      "001010000000001",
		URRs:      []models.URR{{URRID: 1}},
		FARs:      []models.FAR{{FARID: 1, ApplyAction: models.ApplyAction{Forw: true}}},
		PDRs: []models.PDR{
			{PDRID: 1, FARID: 1, URRID: 1, PDI: models.PDI{LocalFTEID: &models.FTEID{}}},
			{PDRID: 2, FARID: 1, PDI: models.PDI{}},
		},
	}

	if _, err := conn.EstablishSession(context.Background(), req); err == nil {
		t.Fatal("expected establish to fail on the malformed PDR")
	}

	if conn.GetSession(seid) != nil {
		t.Fatal("session must not be registered after a failed establish")
	}

	var perCPU []uint64
	if lookupErr := obj.UrrMap.Lookup(upfebpf.N3N6EntrypointUrrKey{Seid: seid, UrrId: 1}, &perCPU); !errors.Is(lookupErr, ebpf.ErrKeyNotExist) {
		t.Fatalf("urr_map entry leaked: want ErrKeyNotExist, got %v", lookupErr)
	}

	if n := countUplinkPDRs(t, obj); n != 0 {
		t.Fatalf("pdrs_uplink leaked %d entries after failed establish", n)
	}
}

func countUplinkPDRs(t *testing.T, obj *upfebpf.BpfObjects) int {
	t.Helper()

	var (
		key   uint32
		value upfebpf.N3N6EntrypointPdrInfo
		count int
	)

	it := obj.PdrsUplink.Iterate()
	for it.Next(&key, &value) {
		count++
	}

	if err := it.Err(); err != nil {
		t.Fatalf("iterate pdrs_uplink: %v", err)
	}

	return count
}
