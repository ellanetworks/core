// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package engine_test

import (
	"context"
	"encoding/binary"
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

func natTestTuple(saddr netip.Addr, sport uint16) upfebpf.N3N6EntrypointFiveTuple {
	addr4 := saddr.As4()

	return upfebpf.N3N6EntrypointFiveTuple{
		Saddr: binary.NativeEndian.Uint32(addr4[:]),
		Daddr: 0x32649cc6,
		Sport: sport,
		Dport: 80,
		Proto: 6,
	}
}

// TestDeleteSessionPurgesNATConntrack verifies that deleting a session
// removes the UE's nat_ct entries in both directions while other UEs'
// entries stay intact. Requires root.
func TestDeleteSessionPurgesNATConntrack(t *testing.T) {
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

	obj := upfebpf.NewBpfObjects(false, true, 1, 0, 0, 0)
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

	const seid = uint64(21)

	ueIP := netip.MustParseAddr("10.45.0.1")
	otherUEIP := netip.MustParseAddr("10.45.0.9")
	natIP := netip.MustParseAddr("192.0.2.1")

	req := &models.EstablishRequest{
		LocalSEID: seid,
		IMSI:      "001010000000001",
		URRs:      []models.URR{{URRID: 1}},
		FARs:      []models.FAR{{FARID: 1, ApplyAction: models.ApplyAction{Forw: true}}},
		PDRs:      []models.PDR{{PDRID: 2, FARID: 1, URRID: 1, PDI: models.PDI{UEIPAddress: ueIP}}},
	}

	if _, err := conn.EstablishSession(context.Background(), req); err != nil {
		t.Fatalf("establish: %v", err)
	}

	// One conntrack pair for the released UE and one for another UE.
	ueKey := natTestTuple(ueIP, 1000)
	natKey := natTestTuple(natIP, 1000)
	otherUEKey := natTestTuple(otherUEIP, 2000)
	otherNATKey := natTestTuple(natIP, 2000)

	entries := map[upfebpf.N3N6EntrypointFiveTuple]upfebpf.N3N6EntrypointNatEntry{
		ueKey:       {Peer: natKey, UeSide: 1},
		natKey:      {Peer: ueKey},
		otherUEKey:  {Peer: otherNATKey, UeSide: 1},
		otherNATKey: {Peer: otherUEKey},
	}

	for k, v := range entries {
		if err := obj.NatCt.Put(&k, &v); err != nil {
			t.Fatalf("insert nat_ct entry: %v", err)
		}
	}

	if err := conn.DeleteSession(context.Background(), &models.DeleteRequest{SEID: seid}); err != nil {
		t.Fatalf("delete session: %v", err)
	}

	var val upfebpf.N3N6EntrypointNatEntry

	for _, k := range []upfebpf.N3N6EntrypointFiveTuple{ueKey, natKey} {
		if err := obj.NatCt.Lookup(&k, &val); !errors.Is(err, ebpf.ErrKeyNotExist) {
			t.Errorf("released UE's nat_ct entry %+v still present (err=%v)", k, err)
		}
	}

	for _, k := range []upfebpf.N3N6EntrypointFiveTuple{otherUEKey, otherNATKey} {
		if err := obj.NatCt.Lookup(&k, &val); err != nil {
			t.Errorf("other UE's nat_ct entry %+v removed (err=%v)", k, err)
		}
	}
}
