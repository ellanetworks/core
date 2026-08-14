// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package engine_test

import (
	"context"
	"net/netip"
	"os"
	"testing"

	"github.com/cilium/ebpf/rlimit"
	"github.com/ellanetworks/core/internal/models"
	upfebpf "github.com/ellanetworks/core/internal/upf/ebpf"
	"github.com/ellanetworks/core/internal/upf/engine"
)

func TestModifySessionFailureKeepsTheLiveUplinkTEID(t *testing.T) {
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

	rm, err := engine.NewFteIDResourceManager(1)
	if err != nil {
		t.Fatalf("new fteid resource manager: %v", err)
	}

	conn, err := engine.NewSessionEngine("1.2.3.4", "nodeId", "2.3.4.5", "", "2.3.4.5", "", obj, rm)
	if err != nil {
		t.Fatalf("new session engine: %v", err)
	}

	ctx := context.Background()

	const seid = uint64(41)

	ueIP := netip.MustParseAddr("10.0.0.21")

	establish := &models.EstablishRequest{
		SEID: seid,
		IMSI: "001010000000001",
		URRs: []models.URR{{URRID: 1}},
		FARs: []models.FAR{{FARID: 1, ApplyAction: models.ApplyAction{Forw: true}}},
		PDRs: []models.PDR{
			{PDRID: 1, FARID: 1, URRID: 1, PDI: models.PDI{LocalFTEID: &models.FTEID{}}},
			{PDRID: 2, FARID: 1, URRID: 1, PDI: models.PDI{UEIPAddress: ueIP}},
		},
	}

	resp, err := conn.EstablishSession(ctx, establish)
	if err != nil {
		t.Fatalf("establish: %v", err)
	}

	if resp.N3TEID == 0 {
		t.Fatal("establish assigned no uplink TEID")
	}

	modify := &models.ModifyRequest{
		SEID: seid,
		UpdatePDRs: []models.PDR{
			{PDRID: 1, FARID: 1, URRID: 1, PDI: models.PDI{LocalFTEID: &models.FTEID{}}},
			{PDRID: 3, FARID: 1, PDI: models.PDI{}},
		},
	}

	if err := conn.ModifySession(ctx, modify); err == nil {
		t.Fatal("expected the modification to fail on the malformed PDR")
	}

	if teid, err := rm.AllocateTEID(seid + 1); err == nil {
		t.Fatalf("the allocator handed out TEID %d after a failed modification: the session is still serving on %d",
			teid, resp.N3TEID)
	}

	session := conn.GetSession(seid)
	if session == nil {
		t.Fatal("session is gone after the failed modification")
	}

	if got := session.GetPDR(1).TeID; got != resp.N3TEID {
		t.Errorf("uplink PDR holds TEID %d, want the %d it was established with", got, resp.N3TEID)
	}
}
