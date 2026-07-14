// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package engine_test

import (
	"context"
	"errors"
	"net/netip"
	"os"
	"testing"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/rlimit"
	"github.com/ellanetworks/core/internal/models"
	upfebpf "github.com/ellanetworks/core/internal/upf/ebpf"
	"github.com/ellanetworks/core/internal/upf/engine"
)

// TestModifySessionRollsBackOnFailure asserts that a mid-modify failure restores
// the session's in-memory rules and unwinds the eBPF entries applied so far, so
// the live session and the data plane never diverge. Requires root.
func TestModifySessionRollsBackOnFailure(t *testing.T) {
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

	const seid = uint64(11)

	establish := &models.EstablishRequest{
		LocalSEID: seid,
		IMSI:      "001010000000001",
		URRs:      []models.URR{{URRID: 1}},
		FARs:      []models.FAR{{FARID: 1, ApplyAction: models.ApplyAction{Forw: true}}},
		PDRs:      []models.PDR{{PDRID: 1, FARID: 1, URRID: 1, PDI: models.PDI{LocalFTEID: &models.FTEID{}}}},
	}

	if _, err := conn.EstablishSession(context.Background(), establish); err != nil {
		t.Fatalf("establish: %v", err)
	}

	before := len(conn.GetSession(seid).ListPDRs())

	ueIP := netip.MustParseAddr("10.0.0.1")

	// A valid downlink PDR (applies to pdrs_downlink_ip4) followed by a malformed
	// PDR (no F-TEID, no UE IP → ExtractPDR fails) forces a mid-modify rollback.
	modify := &models.ModifyRequest{
		SEID: seid,
		CreatePDRs: []models.PDR{
			{PDRID: 2, FARID: 1, PDI: models.PDI{UEIPAddress: ueIP}},
			{PDRID: 3, FARID: 1, PDI: models.PDI{}},
		},
	}

	if err := conn.ModifySession(context.Background(), modify); err == nil {
		t.Fatal("expected modify to fail on the malformed PDR")
	}

	after := conn.GetSession(seid).ListPDRs()
	if len(after) != before {
		t.Fatalf("session PDRs not restored: before=%d after=%d", before, len(after))
	}

	if _, ok := after[2]; ok {
		t.Fatal("created PDR 2 leaked into the session after rollback")
	}

	var v upfebpf.N3N6EntrypointPdrInfo
	if lookupErr := obj.PdrsDownlinkIp4.Lookup(ueIP.As4(), &v); !errors.Is(lookupErr, ebpf.ErrKeyNotExist) {
		t.Fatalf("pdrs_downlink_ip4 entry leaked after rollback: want ErrKeyNotExist, got %v", lookupErr)
	}
}
