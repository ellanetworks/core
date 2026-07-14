// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package engine_test

import (
	"context"
	"errors"
	"net/netip"
	"os"
	"sync"
	"testing"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/rlimit"
	"github.com/ellanetworks/core/internal/models"
	upfebpf "github.com/ellanetworks/core/internal/upf/ebpf"
	"github.com/ellanetworks/core/internal/upf/engine"
)

// TestDeleteVsFilterPropagationNoResurrection races DeleteSession against the
// reconciler's filter propagation on the same session. The per-session op-lock
// must make the delete win cleanly: no PDR entry may survive in the data plane,
// and (under -race) there must be no data race on the session's deleted flag or
// rule maps. A deadlock in the lock ordering shows up as a test timeout.
func TestDeleteVsFilterPropagationNoResurrection(t *testing.T) {
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

	rm, err := engine.NewFteIDResourceManager(65535)
	if err != nil {
		t.Fatalf("new fteid resource manager: %v", err)
	}

	conn, err := engine.NewSessionEngine("1.2.3.4", "nodeId", "2.3.4.5", "", "2.3.4.5", "", obj, rm)
	if err != nil {
		t.Fatalf("new session engine: %v", err)
	}

	const policyID = "policy-race"

	rules := []models.FilterRule{{Protocol: 6, PortLow: 80, PortHigh: 80, Action: models.Allow}}

	ctx := context.Background()

	for i := range 100 {
		seid := uint64(1000 + i)
		ueIP := netip.AddrFrom4([4]byte{10, 0, byte(i >> 8), byte(i)})

		establish := &models.EstablishRequest{
			LocalSEID: seid,
			IMSI:      "001010000000001",
			PolicyID:  policyID,
			URRs:      []models.URR{{URRID: 1}},
			FARs:      []models.FAR{{FARID: 1, ApplyAction: models.ApplyAction{Forw: true}}},
			PDRs:      []models.PDR{{PDRID: 2, FARID: 1, URRID: 1, PDI: models.PDI{UEIPAddress: ueIP}}},
		}

		if _, err := conn.EstablishSession(ctx, establish); err != nil {
			t.Fatalf("iter %d establish: %v", i, err)
		}

		// Allocate the filter slot and propagate it to the session, so the empty
		// update below re-propagates (releasing the slot) concurrently with delete.
		if err := conn.UpdateFilters(ctx, policyID, models.DirectionDownlink, rules); err != nil {
			t.Fatalf("iter %d seed filters: %v", i, err)
		}

		var wg sync.WaitGroup

		wg.Add(2)

		go func() {
			defer wg.Done()

			if err := conn.DeleteSession(ctx, &models.DeleteRequest{SEID: seid}); err != nil {
				t.Errorf("iter %d delete: %v", i, err)
			}
		}()

		go func() {
			defer wg.Done()

			if err := conn.UpdateFilters(ctx, policyID, models.DirectionDownlink, nil); err != nil {
				t.Errorf("iter %d clear filters: %v", i, err)
			}
		}()

		wg.Wait()

		if conn.GetSession(seid) != nil {
			t.Fatalf("iter %d: session still registered after delete", i)
		}

		var v upfebpf.N3N6EntrypointPdrInfo
		if lookupErr := obj.PdrsDownlinkIp4.Lookup(ueIP.As4(), &v); !errors.Is(lookupErr, ebpf.ErrKeyNotExist) {
			t.Fatalf("iter %d: pdrs_downlink_ip4[%s] resurrected after delete: want ErrKeyNotExist, got %v", i, ueIP, lookupErr)
		}

		// Reset the slot for the next iteration (the clear above may have lost the
		// race and left the slot allocated).
		_ = conn.UpdateFilters(ctx, policyID, models.DirectionDownlink, nil)
	}
}
