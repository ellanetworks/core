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

// TestReEstablishTearsDownOldSession asserts that establishing a session for a
// SEID that already has a live session tears the old one down first, leaving no
// orphaned datapath entry from the previous session. Requires root.
func TestReEstablishTearsDownOldSession(t *testing.T) {
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

	const seid = uint64(12)

	oldIP := netip.MustParseAddr("10.0.0.1")
	newIP := netip.MustParseAddr("10.0.0.2")

	req := func(ueIP netip.Addr) *models.EstablishRequest {
		return &models.EstablishRequest{
			LocalSEID: seid,
			IMSI:      "001010000000001",
			URRs:      []models.URR{{URRID: 1}},
			FARs:      []models.FAR{{FARID: 1, ApplyAction: models.ApplyAction{Forw: true}}},
			PDRs:      []models.PDR{{PDRID: 2, FARID: 1, URRID: 1, PDI: models.PDI{UEIPAddress: ueIP}}},
		}
	}

	if _, err := conn.EstablishSession(context.Background(), req(oldIP)); err != nil {
		t.Fatalf("first establish: %v", err)
	}

	if _, err := conn.EstablishSession(context.Background(), req(newIP)); err != nil {
		t.Fatalf("re-establish: %v", err)
	}

	if conn.GetSession(seid) == nil {
		t.Fatal("session not registered after re-establish")
	}

	var v upfebpf.N3N6EntrypointPdrInfo
	if lookupErr := obj.PdrsDownlinkIp4.Lookup(oldIP.As4(), &v); !errors.Is(lookupErr, ebpf.ErrKeyNotExist) {
		t.Fatalf("old session's downlink PDR leaked: want ErrKeyNotExist for %s, got %v", oldIP, lookupErr)
	}

	if lookupErr := obj.PdrsDownlinkIp4.Lookup(newIP.As4(), &v); lookupErr != nil {
		t.Fatalf("new session's downlink PDR missing for %s: %v", newIP, lookupErr)
	}
}
