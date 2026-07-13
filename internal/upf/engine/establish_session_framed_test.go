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

// TestEstablishSessionSkipsMismatchedFamilyFramedRoute asserts that a framed
// route whose family the session does not carry (here an IPv6 route on an
// IPv4-only session) is skipped, not fatal: a dormant cross-family route must
// not deny the UE all connectivity (TS 23.501 §5.6.14). Requires root.
func TestEstablishSessionSkipsMismatchedFamilyFramedRoute(t *testing.T) {
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

	v4Route := netip.MustParsePrefix("10.10.0.0/24")
	v6Route := netip.MustParsePrefix("fd00:beef::/48")

	req := &models.EstablishRequest{
		LocalSEID: 1,
		IMSI:      "001010000000001",
		PDRs: []models.PDR{{
			PDRID: 1, FARID: 1, QERID: 1, URRID: 1,
			PDI: models.PDI{UEIPAddress: netip.MustParseAddr("10.45.0.1")},
		}},
		FARs:         []models.FAR{{FARID: 1, ApplyAction: models.ApplyAction{Forw: true}}},
		QERs:         []models.QER{{QERID: 1, QFI: 9}},
		URRs:         []models.URR{{URRID: 1}},
		FramedRoutes: []netip.Prefix{v4Route, v6Route},
	}

	if _, err := conn.EstablishSession(context.Background(), req); err != nil {
		t.Fatalf("establishment failed with a mismatched-family framed route (it should be skipped): %v", err)
	}

	if has, err := obj.HasFramedDownlink(v4Route); err != nil || !has {
		t.Fatalf("same-family IPv4 framed route not installed: has=%v err=%v", has, err)
	}

	if has, err := obj.HasFramedDownlink(v6Route); err != nil || has {
		t.Fatalf("IPv6 framed route installed on an IPv4-only session (should be skipped): has=%v err=%v", has, err)
	}
}
