// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package engine

import (
	"context"
	"net/netip"
	"os"
	"sync"
	"testing"

	"github.com/cilium/ebpf/rlimit"
	"github.com/ellanetworks/core/internal/models"
	upfebpf "github.com/ellanetworks/core/internal/upf/ebpf"
)

func TestFilterReleaseVsSessionApplyNoSlotReuse(t *testing.T) {
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

	rm, err := NewFteIDResourceManager(65535)
	if err != nil {
		t.Fatalf("new fteid resource manager: %v", err)
	}

	conn, err := NewSessionEngine("1.2.3.4", "nodeId", "2.3.4.5", "", "2.3.4.5", "", obj, rm)
	if err != nil {
		t.Fatalf("new session engine: %v", err)
	}

	const (
		policyReleased = "policy-released"
		policyReuser   = "policy-reuser"
	)

	rules := []models.FilterRule{{Protocol: 6, PortLow: 80, PortHigh: 80, Action: models.Allow}}

	ctx := context.Background()

	for i := range 100 {
		seid := uint64(2000 + i)
		ueIP := netip.AddrFrom4([4]byte{10, 1, byte(i >> 8), byte(i)})

		if _, err := conn.EstablishSession(ctx, &models.EstablishRequest{
			SEID:     seid,
			IMSI:     "001010000000001",
			PolicyID: policyReleased,
			URRs:     []models.URR{{URRID: 1}},
			FARs:     []models.FAR{{FARID: 1, ApplyAction: models.ApplyAction{Forw: true}}},
			PDRs:     []models.PDR{{PDRID: 2, FARID: 1, URRID: 1, PDI: models.PDI{UEIPAddress: ueIP}}},
		}); err != nil {
			t.Fatalf("iter %d establish: %v", i, err)
		}

		if err := conn.UpdateFilters(ctx, policyReleased, models.DirectionDownlink, rules); err != nil {
			t.Fatalf("iter %d seed filters: %v", i, err)
		}

		var wg sync.WaitGroup

		wg.Add(2)

		go func() {
			defer wg.Done()

			if err := conn.UpdateFilters(ctx, policyReleased, models.DirectionDownlink, nil); err != nil {
				t.Errorf("iter %d release: %v", i, err)
				return
			}

			if err := conn.UpdateFilters(ctx, policyReuser, models.DirectionDownlink, rules); err != nil {
				t.Errorf("iter %d reuse: %v", i, err)
			}
		}()

		go func() {
			defer wg.Done()

			if err := conn.ModifySession(ctx, &models.ModifyRequest{
				SEID:       seid,
				PolicyID:   policyReleased,
				UpdatePDRs: []models.PDR{{PDRID: 2, FARID: 1, URRID: 1, PDI: models.PDI{UEIPAddress: ueIP}}},
			}); err != nil {
				t.Errorf("iter %d modify: %v", i, err)
			}
		}()

		wg.Wait()

		assertNoForeignFilterSlots(t, conn, i)

		if err := conn.DeleteSession(ctx, &models.DeleteRequest{SEID: seid}); err != nil {
			t.Fatalf("iter %d delete: %v", i, err)
		}

		if err := conn.UpdateFilters(ctx, policyReuser, models.DirectionDownlink, nil); err != nil {
			t.Fatalf("iter %d reuse cleanup: %v", i, err)
		}
	}
}

func assertNoForeignFilterSlots(t *testing.T, conn *SessionEngine, iter int) {
	t.Helper()

	for seid, session := range conn.ListSessions() {
		policyID := session.PolicyID()

		uplinkIdx := filterIndex(conn, policyID, models.DirectionUplink)
		downlinkIdx := filterIndex(conn, policyID, models.DirectionDownlink)

		for pdrID, spdrInfo := range session.ListPDRs() {
			want := uplinkIdx
			if spdrInfo.UEIP.IsValid() {
				want = downlinkIdx
			}

			got := spdrInfo.PdrInfo.FilterMapIndex
			if got != upfebpf.NoFilterIndex && got != want {
				t.Fatalf("iter %d: SEID %d PDR %d (policy %q) points at filter slot %d, but the policy's slot is %d",
					iter, seid, pdrID, policyID, got, want)
			}
		}
	}
}
