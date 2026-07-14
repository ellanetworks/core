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
	"github.com/ellanetworks/core/internal/upf/ebpf"
	"github.com/ellanetworks/core/internal/upf/engine"
)

// TestEstablishSessionStampsUplinkUEAddresses asserts that establishing a
// dual-stack session records both UE source addresses on the session and stamps
// them onto the uplink PDR programmed into pdrs_uplink (anti-spoofing). The IPv6
// address is stored as the /64 base. Requires root/CAP_BPF.
func TestEstablishSessionStampsUplinkUEAddresses(t *testing.T) {
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

	obj := ebpf.NewBpfObjects(false, false, 1, 0, 0, 0)
	if err := obj.Load(); err != nil {
		t.Fatalf("load eBPF objects: %v", err)
	}

	t.Cleanup(func() { _ = obj.Close() })

	rm, err := engine.NewFteIDResourceManager(1000)
	if err != nil {
		t.Fatalf("fteid manager: %v", err)
	}

	conn, err := engine.NewSessionEngine("1.2.3.4", "nodeId", "2.3.4.5", "", "2.3.4.5", "", obj, rm)
	if err != nil {
		t.Fatalf("new session engine: %v", err)
	}

	ueV4 := netip.MustParseAddr("10.45.0.1")
	ueV6 := netip.MustParseAddr("2001:db8:1::") // /64 base

	// PDRs arrive uplink-first, IPv6 downlink last — the order that makes a
	// per-PDR (rather than pre-scan) population miss the IPv6 /64.
	req := &models.EstablishRequest{
		LocalSEID: 1,
		IMSI:      "001010000000001",
		PDRs: []models.PDR{
			{PDRID: 1, FARID: 1, QERID: 1, PDI: models.PDI{LocalFTEID: &models.FTEID{}}},
			{PDRID: 2, FARID: 1, QERID: 1, PDI: models.PDI{UEIPAddress: ueV4}},
			{PDRID: 3, FARID: 1, QERID: 1, PDI: models.PDI{UEIPAddress: ueV6}},
		},
		FARs: []models.FAR{{FARID: 1, ApplyAction: models.ApplyAction{Forw: true}}},
		QERs: []models.QER{{QERID: 1, QFI: 9}},
	}

	if _, err := conn.EstablishSession(context.Background(), req); err != nil {
		t.Fatalf("establishment failed: %v", err)
	}

	sess := conn.GetSession(1)
	if sess == nil {
		t.Fatal("session not found after establishment")
	}

	gotV4, gotV6 := sess.UEAddresses()
	if gotV4 != ueV4 {
		t.Errorf("session UE IPv4 = %v, want %v", gotV4, ueV4)
	}

	if gotV6 != ueV6 {
		t.Errorf("session UE IPv6 = %v, want %v", gotV6, ueV6)
	}

	// Read the single uplink PDR back from the map and confirm both families are
	// stamped (IPv4 as ::ffff-mapped, IPv6 as the /64 base).
	var (
		teid  uint32
		pi    ebpf.N3N6EntrypointPdrInfo
		iter  = obj.PdrsUplink.Iterate()
		found bool
	)

	for iter.Next(&teid, &pi) {
		found = true
		break
	}

	if err := iter.Err(); err != nil {
		t.Fatalf("iterate pdrs_uplink: %v", err)
	}

	if !found {
		t.Fatal("no uplink PDR programmed")
	}

	if got := netip.AddrFrom16(pi.UeIpv4.In6U.U6Addr8).Unmap(); got != ueV4 {
		t.Errorf("uplink PDR UE IPv4 = %v, want %v", got, ueV4)
	}

	if got := netip.AddrFrom16(pi.UeIpv6.In6U.U6Addr8); got != ueV6 {
		t.Errorf("uplink PDR UE IPv6 = %v, want %v", got, ueV6)
	}
}
