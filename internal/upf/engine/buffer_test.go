// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package engine_test

import (
	"context"
	"net/netip"
	"os"
	"sync"
	"testing"

	"github.com/cilium/ebpf/rlimit"
	"github.com/ellanetworks/core/internal/models"
	upfebpf "github.com/ellanetworks/core/internal/upf/ebpf"
	"github.com/ellanetworks/core/internal/upf/engine"
)

// recordingBuffer is a fake DownlinkBuffer that records Drain and Drop calls.
type recordingBuffer struct {
	mu     sync.Mutex
	drains []uint64
	drops  []uint64
}

func (r *recordingBuffer) Drain(seid uint64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.drains = append(r.drains, seid)
}

func (r *recordingBuffer) Drop(seid uint64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.drops = append(r.drops, seid)
}

func (r *recordingBuffer) drained() []uint64 {
	r.mu.Lock()
	defer r.mu.Unlock()

	return append([]uint64(nil), r.drains...)
}

func (r *recordingBuffer) dropped() []uint64 {
	r.mu.Lock()
	defer r.mu.Unlock()

	return append([]uint64(nil), r.drops...)
}

// newBufferTestEngine returns a session engine with one established session
// buffering packets. Requires root.
func newBufferTestEngine(t *testing.T, buf engine.DownlinkBuffer) *engine.SessionEngine {
	t.Helper()

	farForw := false

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

	rm, err := engine.NewFteIDResourceManager(1024)
	if err != nil {
		t.Fatalf("new fteid resource manager: %v", err)
	}

	conn, err := engine.NewSessionEngine("1.2.3.4", "nodeId", "2.3.4.5", "", "2.3.4.5", "", obj, rm)
	if err != nil {
		t.Fatalf("new session engine: %v", err)
	}

	conn.SetDownlinkBuffer(buf)

	const seid = uint64(21)

	establish := &models.EstablishRequest{
		SEID: seid,
		IMSI: "001010000000001",
		URRs: []models.URR{{URRID: 1}},
		FARs: []models.FAR{{FARID: 1, ApplyAction: models.ApplyAction{Forw: farForw, Buff: !farForw, Nocp: !farForw}}},
		PDRs: []models.PDR{{PDRID: 1, FARID: 1, URRID: 1, PDI: models.PDI{UEIPAddress: netip.MustParseAddr("10.0.0.1"), LocalFTEID: &models.FTEID{}}}},
	}

	if _, err := conn.EstablishSession(context.Background(), establish); err != nil {
		t.Fatalf("establish: %v", err)
	}

	return conn
}

// TestModifySessionDrainsOnFarForward checks that a modify flipping the FAR to
// FORW drains the session's buffered packets exactly once, and only after the
// transaction committed.
func TestModifySessionDrainsOnFarForward(t *testing.T) {
	const seid = uint64(21)

	buf := &recordingBuffer{}
	conn := newBufferTestEngine(t, buf)

	modify := &models.ModifyRequest{
		SEID:       seid,
		UpdateFARs: []models.FAR{{FARID: 1, ApplyAction: models.ApplyAction{Forw: true}}},
	}

	if err := conn.ModifySession(context.Background(), modify); err != nil {
		t.Fatalf("modify: %v", err)
	}

	if got := buf.drained(); len(got) != 1 || got[0] != seid {
		t.Errorf("drains = %v, want one drain for SEID %d", got, seid)
	}
}

// TestModifySessionNoDrainWithoutFarForward checks that a modify leaving the
// FAR at BUFF|NOCP does not drain: the packets stay queued until the UE
// actually returns.
func TestModifySessionNoDrainWithoutFarForward(t *testing.T) {
	const seid = uint64(21)

	buf := &recordingBuffer{}
	conn := newBufferTestEngine(t, buf)

	modify := &models.ModifyRequest{
		SEID:       seid,
		UpdatePDRs: []models.PDR{{PDRID: 1, FARID: 1, PDI: models.PDI{UEIPAddress: netip.MustParseAddr("10.0.0.1")}}},
	}

	if err := conn.ModifySession(context.Background(), modify); err != nil {
		t.Fatalf("modify: %v", err)
	}

	if got := buf.drained(); len(got) != 0 {
		t.Errorf("drains = %v, want none", got)
	}
}

// TestSuppressDropsBufferedPackets checks that a failed page drops the
// session's buffered packets rather than holding them to the TTL.
func TestSuppressDropsBufferedPackets(t *testing.T) {
	const seid = uint64(21)

	buf := &recordingBuffer{}
	conn := newBufferTestEngine(t, buf)

	conn.SuppressDownlinkDataNotification(seid)

	if got := buf.dropped(); len(got) != 1 || got[0] != seid {
		t.Errorf("drops = %v, want one drop for SEID %d", got, seid)
	}
}

// TestDeleteSessionDropsBufferedPackets checks that a deleted session's
// buffered packets are dropped, not leaked to the TTL sweeper.
func TestDeleteSessionDropsBufferedPackets(t *testing.T) {
	const seid = uint64(21)

	buf := &recordingBuffer{}
	conn := newBufferTestEngine(t, buf)

	if err := conn.DeleteSession(context.Background(), &models.DeleteRequest{SEID: seid}); err != nil {
		t.Fatalf("delete: %v", err)
	}

	if got := buf.dropped(); len(got) != 1 || got[0] != seid {
		t.Errorf("drops = %v, want one drop for SEID %d", got, seid)
	}
}
