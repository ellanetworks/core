// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package engine

import (
	"context"
	"net/netip"
	"os"
	"testing"

	"github.com/cilium/ebpf/rlimit"
	"github.com/ellanetworks/core/internal/models"
	upfebpf "github.com/ellanetworks/core/internal/upf/ebpf"
)

func TestModifySessionPolicyChangeRepointsFilterIndex(t *testing.T) {
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

	obj := upfebpf.NewBpfObjects(false, false, false, 1, 0, 0, 0)
	if err := obj.Load(); err != nil {
		t.Fatalf("load eBPF objects: %v", err)
	}

	t.Cleanup(func() { _ = obj.Close() })

	rm, err := NewFteIDResourceManager(1024)
	if err != nil {
		t.Fatalf("new fteid resource manager: %v", err)
	}

	conn, err := NewSessionEngine("1.2.3.4", "nodeId", "2.3.4.5", "", "2.3.4.5", "", obj, rm)
	if err != nil {
		t.Fatalf("new session engine: %v", err)
	}

	ctx := context.Background()

	const (
		seid    = uint64(31)
		policyA = "policy-a"
		policyB = "policy-b"
	)

	deny := []models.FilterRule{{Protocol: 6, PortLow: 80, PortHigh: 80, Action: models.Deny}}

	for _, policyID := range []string{policyA, policyB} {
		for _, dir := range []models.Direction{models.DirectionUplink, models.DirectionDownlink} {
			if err := conn.UpdateFilters(ctx, policyID, dir, deny); err != nil {
				t.Fatalf("install %s filters for %s: %v", dir, policyID, err)
			}
		}
	}

	ueIP := netip.MustParseAddr("10.0.0.11")

	pdrs := []models.PDR{
		{PDRID: 1, FARID: 1, URRID: 1, PDI: models.PDI{LocalFTEID: &models.FTEID{}}},
		{PDRID: 2, FARID: 1, URRID: 1, PDI: models.PDI{UEIPAddress: ueIP}},
	}

	establish := &models.EstablishRequest{
		SEID:     seid,
		IMSI:     "001010000000001",
		PolicyID: policyA,
		URRs:     []models.URR{{URRID: 1}},
		FARs:     []models.FAR{{FARID: 1, ApplyAction: models.ApplyAction{Forw: true}}},
		PDRs:     pdrs,
	}

	if _, err := conn.EstablishSession(ctx, establish); err != nil {
		t.Fatalf("establish: %v", err)
	}

	if err := conn.ModifySession(ctx, &models.ModifyRequest{
		SEID:       seid,
		PolicyID:   policyB,
		UpdatePDRs: pdrs,
	}); err != nil {
		t.Fatalf("modify onto %s: %v", policyB, err)
	}

	session := conn.GetSession(seid)
	if session == nil {
		t.Fatal("session is gone after the modification")
	}

	for pdrID, dir := range map[uint32]models.Direction{1: models.DirectionUplink, 2: models.DirectionDownlink} {
		want := filterIndex(conn, policyB, dir)
		if want == upfebpf.NoFilterIndex {
			t.Fatalf("%s filters for %s were not installed", dir, policyB)
		}

		if got := session.GetPDR(pdrID).PdrInfo.FilterMapIndex; got != want {
			t.Errorf("PDR %d holds filter index %d after the move to %s, want %d: it still points at %s's slot, which %s's release frees and the allocator reissues",
				pdrID, got, policyB, want, policyA, policyA)
		}
	}
}
