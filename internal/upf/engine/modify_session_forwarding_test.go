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

func TestModifySessionForwarding(t *testing.T) {
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

	conn, err := engine.NewSessionEngine("1.2.3.4", "nodeId", netip.MustParseAddr("2.3.4.5"), netip.Addr{}, netip.MustParseAddr("2.3.4.5"), netip.Addr{}, obj, rm)
	if err != nil {
		t.Fatalf("new session engine: %v", err)
	}

	conn.SetNetworkInstances(map[string]uint8{"ims": 1})

	const seid = uint64(31)

	ueIP := netip.MustParseAddr("10.0.0.1")

	establish := &models.EstablishRequest{
		SEID: seid,
		IMSI: "001010000000001",
		URRs: []models.URR{{URRID: 1}, {URRID: 2}},
		FARs: []models.FAR{{FARID: 1, ApplyAction: models.ApplyAction{Forw: true}}},
		PDRs: []models.PDR{{PDRID: 1, FARID: 1, URRID: 1, PDI: models.PDI{UEIPAddress: ueIP}}},
	}

	if _, err := conn.EstablishSession(context.Background(), establish); err != nil {
		t.Fatalf("establish: %v", err)
	}

	downlink := func() upfebpf.N3N6EntrypointPdrInfo {
		t.Helper()

		var v upfebpf.N3N6EntrypointPdrInfo
		if err := obj.PdrsDownlinkIp4.Lookup(ueIP.As4(), &v); err != nil {
			t.Fatalf("lookup downlink PDR: %v", err)
		}

		return v
	}

	flags := func() uint8 {
		t.Helper()

		return downlink().Flags & (upfebpf.PdrFlagNATExempt | upfebpf.PdrFlagLocalSwitch)
	}

	if got := flags(); got != 0 {
		t.Fatalf("flags after establish = %#x, want 0", got)
	}

	failing := &models.ModifyRequest{
		SEID:       seid,
		Forwarding: &models.SessionForwarding{NATExempt: true, LocalSwitch: true, NetworkInstance: "ims"},
		UpdatePDRs: []models.PDR{{PDRID: 3, FARID: 1, PDI: models.PDI{}}},
	}

	if _, err := conn.ModifySession(context.Background(), failing); err == nil {
		t.Fatal("expected modify to fail on the malformed PDR")
	}

	if got := flags(); got != 0 {
		t.Fatalf("flags after a failed modify = %#x, want 0", got)
	}

	if got := downlink().NiId; got != 0 {
		t.Fatalf("network instance after a failed modify = %d, want 0", got)
	}

	resp, err := conn.ModifySession(context.Background(), &models.ModifyRequest{
		SEID:       seid,
		Forwarding: &models.SessionForwarding{NATExempt: true, LocalSwitch: true, NetworkInstance: "ims"},
		RemoveURRs: []uint32{2},
	})
	if err != nil {
		t.Fatalf("modify forwarding: %v", err)
	}

	if len(resp.Usage) != 1 {
		t.Fatalf("usage reports = %d, want 1 for the removed URR", len(resp.Usage))
	}

	if got := flags(); got != upfebpf.PdrFlagNATExempt|upfebpf.PdrFlagLocalSwitch {
		t.Fatalf("flags after modify = %#x, want NAT exempt and local switch", got)
	}

	if got := downlink().NiId; got != 1 {
		t.Fatalf("network instance after modify = %d, want 1", got)
	}
}
