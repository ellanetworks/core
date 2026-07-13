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

// TestDeleteSessionRemovesFramedRoutes asserts that deleting a session removes
// its framed-route LPM entries from the datapath, so a released session leaves
// no orphan framed routes (TS 29.244 §5.16). Requires root to load the eBPF
// maps.
func TestDeleteSessionRemovesFramedRoutes(t *testing.T) {
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

	conn, err := engine.NewSessionEngine("1.2.3.4", "nodeId", "2.3.4.5", "", "2.3.4.5", "", obj, nil)
	if err != nil {
		t.Fatalf("new session engine: %v", err)
	}

	const seid = uint64(1)

	prefix := netip.MustParsePrefix("192.168.50.0/24")
	ueAddr := netip.MustParseAddr("10.0.0.1")

	if err := obj.PutFramedDownlink(prefix, ueAddr); err != nil {
		t.Fatalf("install framed route: %v", err)
	}

	sess := engine.NewSession(seid)
	sess.SetFramedRoutes([]netip.Prefix{prefix})
	conn.AddSession(seid, sess)

	if has, err := obj.HasFramedDownlink(prefix); err != nil || !has {
		t.Fatalf("framed route missing before delete: has=%v err=%v", has, err)
	}

	if err := conn.DeleteSession(context.Background(), &models.DeleteRequest{SEID: seid}); err != nil {
		t.Fatalf("delete session: %v", err)
	}

	has, err := obj.HasFramedDownlink(prefix)
	if err != nil {
		t.Fatalf("lookup framed route after delete: %v", err)
	}

	if has {
		t.Fatal("framed route still present after session delete; entry leaked")
	}
}
