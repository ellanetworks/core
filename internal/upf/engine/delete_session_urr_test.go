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

// TestDeleteSessionRemovesURR asserts that deleting a session removes its
// urr_map entry, keyed by (SEID, URR ID). The per-session key means a released
// session leaves no orphan counter for a later session to inherit. Requires root
// to load the eBPF maps.
func TestDeleteSessionRemovesURR(t *testing.T) {
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

	conn, err := engine.NewSessionEngine("1.2.3.4", "nodeId", "2.3.4.5", "", "2.3.4.5", "", obj, nil)
	if err != nil {
		t.Fatalf("new session engine: %v", err)
	}

	const (
		seid  = uint64(7)
		urrID = uint32(2)
		pdrID = uint32(2)
	)

	ueAddr := netip.MustParseAddr("10.0.0.1")

	if err := obj.PutPdrDownlink(ueAddr, upfebpf.PdrInfo{SEID: seid, PdrID: pdrID, UrrID: urrID}); err != nil {
		t.Fatalf("install downlink PDR: %v", err)
	}

	if err := obj.NewUrr(seid, urrID); err != nil {
		t.Fatalf("install URR: %v", err)
	}

	sess := engine.NewSession(seid)
	sess.PutPDR(pdrID, engine.SPDRInfo{
		PdrID:   pdrID,
		UEIP:    ueAddr,
		PdrInfo: upfebpf.PdrInfo{SEID: seid, PdrID: pdrID, UrrID: urrID},
	})
	conn.AddSession(seid, sess)

	key := upfebpf.N3N6EntrypointUrrKey{Seid: seid, UrrId: urrID}

	var perCPU []uint64
	if err := obj.UrrMap.Lookup(key, &perCPU); err != nil {
		t.Fatalf("URR missing before delete: %v", err)
	}

	if err := conn.DeleteSession(context.Background(), &models.DeleteRequest{SEID: seid}); err != nil {
		t.Fatalf("delete session: %v", err)
	}

	if err := obj.UrrMap.Lookup(key, &perCPU); !errors.Is(err, ebpf.ErrKeyNotExist) {
		t.Fatalf("urr_map entry still present after session delete; want ErrKeyNotExist, got %v", err)
	}
}
