// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package raft

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/raft"
)

func soleServer(t *testing.T, m *Manager) raft.Server {
	t.Helper()

	future := m.raft.GetConfiguration()
	if err := future.Error(); err != nil {
		t.Fatalf("read raft configuration: %v", err)
	}

	servers := future.Configuration().Servers
	if len(servers) != 1 {
		t.Fatalf("expected exactly one server, got %d", len(servers))
	}

	return servers[0]
}

func proposeSome(t *testing.T, m *Manager, n int) {
	t.Helper()

	for i := 0; i < n; i++ {
		cmd, err := NewCommand(CmdChangeset, map[string]int{"n": i})
		if err != nil {
			t.Fatalf("NewCommand: %v", err)
		}

		if _, err := m.Propose(cmd, 10*time.Second); err != nil {
			t.Fatalf("Propose %d: %v", i, err)
		}
	}
}

func TestStandaloneToHARealignsStaleLoopbackAddress(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	dataDir := t.TempDir()

	standalone, err := NewManager(ctx, FastTestConfig(), newTestApplier(t), dataDir)
	if err != nil {
		t.Fatalf("NewManager standalone: %v", err)
	}

	if err := standalone.WaitForLeaderBarrier(ctx); err != nil {
		t.Fatalf("standalone barrier: %v", err)
	}

	bootstrapped := soleServer(t, standalone)
	if !strings.HasPrefix(string(bootstrapped.Address), "127.0.0.1:") {
		t.Fatalf("standalone should bootstrap on loopback, got %q", bootstrapped.Address)
	}

	proposeSome(t, standalone, 3)

	legacyLast, err := standalone.logStore.LastIndex()
	if err != nil {
		t.Fatalf("read legacy last log index: %v", err)
	}

	if legacyLast == 0 {
		t.Fatal("the standalone store should carry log entries to leave behind")
	}

	if err := standalone.Shutdown(); err != nil {
		t.Fatalf("shutdown standalone: %v", err)
	}

	advertise := fmt.Sprintf("127.0.0.1:%d", freePort(t))

	haCfg := FastTestConfig()
	haCfg.Enabled = true
	haCfg.BindAddress = advertise
	haCfg.AdvertiseAddress = advertise

	applier := newTestApplier(t)

	var restores atomic.Int32

	applier.reopenHook = func() { restores.Add(1) }

	ha, err := NewManager(ctx, haCfg, applier, dataDir)
	if err != nil {
		t.Fatalf("NewManager HA: %v", err)
	}

	t.Cleanup(func() { _ = ha.Shutdown() })

	if err := ha.WaitForLeaderBarrier(ctx); err != nil {
		t.Fatalf("realigned node must still be able to lead: %v", err)
	}

	if got := soleServer(t, ha).Address; string(got) != advertise {
		t.Fatalf("flipping to HA must rewrite the stale bootstrap address: got %q, want %q", got, advertise)
	}

	if n := restores.Load(); n != 0 {
		t.Fatalf("conversion must not restore a snapshot into the FSM (Reopen ran %d times); a live database ahead of its snapshots must be preserved", n)
	}

	snapLast, err := newestSnapshotIndex(ha.snaps)
	if err != nil {
		t.Fatalf("list snapshots after conversion: %v", err)
	}

	if snapLast < legacyLast {
		t.Fatalf("checkpoint snapshot index %d must cover the legacy log (last index %d)", snapLast, legacyLast)
	}

	first, err := ha.logStore.FirstIndex()
	if err != nil {
		t.Fatalf("read first log index after conversion: %v", err)
	}

	if first != 0 && first <= legacyLast {
		t.Fatalf("legacy log prefix survived the checkpoint: first index %d, legacy last index %d", first, legacyLast)
	}

	joinerAddr := fmt.Sprintf("127.0.0.1:%d", freePort(t))

	joinerCfg := FastTestConfig()
	joinerCfg.Enabled = true
	joinerCfg.RaftID = "2"
	joinerCfg.BindAddress = joinerAddr
	joinerCfg.AdvertiseAddress = joinerAddr

	joinerApplier := newTestApplier(t)

	var joinerRestores atomic.Int32

	joinerApplier.reopenHook = func() { joinerRestores.Add(1) }

	joiner, err := NewManager(ctx, joinerCfg, joinerApplier, t.TempDir())
	if err != nil {
		t.Fatalf("NewManager joiner: %v", err)
	}

	t.Cleanup(func() { _ = joiner.Shutdown() })

	if err := ha.AddVoter("2", joinerAddr); err != nil {
		t.Fatalf("admit joiner after checkpoint: %v", err)
	}

	for joinerRestores.Load() == 0 {
		if ctx.Err() != nil {
			t.Fatal("joiner never installed the checkpoint snapshot")
		}

		time.Sleep(10 * time.Millisecond)
	}

	if seen := joinerApplier.seen(); len(seen) != 0 {
		t.Fatalf("joiner replayed %d pre-checkpoint commands; it must start from the checkpoint snapshot", len(seen))
	}
}

type blockingApplier struct {
	Applier

	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (a *blockingApplier) ApplyCommand(ctx context.Context, cmd *Command, index, term uint64) (any, error) {
	a.once.Do(func() {
		close(a.entered)
		<-a.release
	})

	return a.Applier.ApplyCommand(ctx, cmd, index, term)
}

func TestAdmissionWaitsForSoleServerCheckpoint(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	dataDir := t.TempDir()

	first, err := NewManager(ctx, FastTestConfig(), newTestApplier(t), dataDir)
	if err != nil {
		t.Fatalf("NewManager first boot: %v", err)
	}

	if err := first.WaitForLeaderBarrier(ctx); err != nil {
		t.Fatalf("first boot barrier: %v", err)
	}

	proposeSome(t, first, 1)

	legacyLast, err := first.logStore.LastIndex()
	if err != nil {
		t.Fatalf("read first boot last index: %v", err)
	}

	if err := first.Shutdown(); err != nil {
		t.Fatalf("shutdown first boot: %v", err)
	}

	applier := &blockingApplier{
		Applier: newTestApplier(t),
		entered: make(chan struct{}),
		release: make(chan struct{}),
	}

	var releaseOnce sync.Once

	release := func() { releaseOnce.Do(func() { close(applier.release) }) }
	defer release()

	second, err := NewManager(ctx, FastTestConfig(), applier, dataDir)
	if err != nil {
		t.Fatalf("NewManager restart: %v", err)
	}

	t.Cleanup(func() { _ = second.Shutdown() })

	select {
	case <-applier.entered:
	case <-ctx.Done():
		t.Fatal("startup replay never reached the applier")
	}

	joined := make(chan error, 1)

	go func() { joined <- second.AddNonvoter("joiner", "127.0.0.1:1") }()

	admittedEarly := 0

	deadline := time.Now().Add(300 * time.Millisecond)
	for time.Now().Before(deadline) {
		if servers := len(soleServerConfig(t, second)); servers != 1 {
			admittedEarly = servers
			break
		}

		time.Sleep(5 * time.Millisecond)
	}

	release()

	if admittedEarly != 0 {
		t.Fatalf("peer admitted during startup replay, before the checkpoint (%d servers)", admittedEarly)
	}

	select {
	case err := <-joined:
		if err != nil {
			t.Fatalf("join after checkpoint: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("join never completed")
	}

	if !second.checkpointDone.Load() {
		t.Fatal("join completed without the checkpoint")
	}

	firstAfter, err := second.logStore.FirstIndex()
	if err != nil {
		t.Fatalf("read first index after join: %v", err)
	}

	if firstAfter != 0 && firstAfter <= legacyLast {
		t.Fatalf("the peer was admitted before the checkpoint retired the first boot's log (first index %d, first boot last index %d)", firstAfter, legacyLast)
	}
}

func soleServerConfig(t *testing.T, m *Manager) []raft.Server {
	t.Helper()

	future := m.raft.GetConfiguration()
	if err := future.Error(); err != nil {
		t.Fatalf("read raft configuration: %v", err)
	}

	return future.Configuration().Servers
}

func TestAdmissionGateFailsFastOffLeader(t *testing.T) {
	t.Parallel()

	m, shutdown := NewTestManager(t, newTestApplier(t))
	shutdown()

	m.admissionPending.Store(true)

	start := time.Now()

	if err := m.awaitAdmitted(); !errors.Is(err, raft.ErrNotLeader) {
		t.Fatalf("awaitAdmitted off leader = %v, want ErrNotLeader", err)
	}

	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("awaitAdmitted off leader waited %s", elapsed)
	}
}

func TestHARestartLeavesMatchingAddressAlone(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	dataDir := t.TempDir()
	advertise := fmt.Sprintf("127.0.0.1:%d", freePort(t))

	cfg := FastTestConfig()
	cfg.Enabled = true
	cfg.RaftID = "1"
	cfg.BindAddress = advertise
	cfg.AdvertiseAddress = advertise
	cfg.Bootstrap = true

	first, err := NewManager(ctx, cfg, newTestApplier(t), dataDir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	if err := first.StartDiscovery(ctx); err == nil {
		t.Fatal("expected discovery to refuse without a cluster listener")
	}

	if err := first.raft.BootstrapCluster(raft.Configuration{
		Servers: []raft.Server{{ID: raft.ServerID("1"), Address: raft.ServerAddress(advertise)}},
	}).Error(); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	if err := first.WaitForLeaderBarrier(ctx); err != nil {
		t.Fatalf("barrier: %v", err)
	}

	if err := first.Shutdown(); err != nil {
		t.Fatalf("shutdown: %v", err)
	}

	second, err := NewManager(ctx, cfg, newTestApplier(t), dataDir)
	if err != nil {
		t.Fatalf("NewManager restart: %v", err)
	}

	t.Cleanup(func() { _ = second.Shutdown() })

	if err := second.WaitForLeaderBarrier(ctx); err != nil {
		t.Fatalf("restart barrier: %v", err)
	}

	if got := soleServer(t, second).Address; string(got) != advertise {
		t.Fatalf("an unchanged address must survive a restart untouched: got %q, want %q", got, advertise)
	}
}

func TestSoleNodeCheckpointRetiresLegacyLogTail(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	dataDir := t.TempDir()

	first, err := NewManager(ctx, FastTestConfig(), newTestApplier(t), dataDir)
	if err != nil {
		t.Fatalf("NewManager first boot: %v", err)
	}

	if err := first.WaitForLeaderBarrier(ctx); err != nil {
		t.Fatalf("first boot barrier: %v", err)
	}

	proposeSome(t, first, 3)

	if err := first.Snapshot(); err != nil {
		t.Fatalf("legacy snapshot: %v", err)
	}

	proposeSome(t, first, 4)

	legacyLast, err := first.logStore.LastIndex()
	if err != nil {
		t.Fatalf("read legacy last index: %v", err)
	}

	legacyFirst, err := first.logStore.FirstIndex()
	if err != nil {
		t.Fatalf("read legacy first index: %v", err)
	}

	if legacyFirst == 0 || legacyLast <= legacyFirst {
		t.Fatalf("legacy store should retain a log prefix and tail (first %d, last %d)", legacyFirst, legacyLast)
	}

	if err := first.Shutdown(); err != nil {
		t.Fatalf("shutdown first boot: %v", err)
	}

	second, err := NewManager(ctx, FastTestConfig(), newTestApplier(t), dataDir)
	if err != nil {
		t.Fatalf("NewManager restart: %v", err)
	}

	t.Cleanup(func() { _ = second.Shutdown() })

	if err := second.WaitForLeaderBarrier(ctx); err != nil {
		t.Fatalf("restart barrier: %v", err)
	}

	snapLast, err := newestSnapshotIndex(second.snaps)
	if err != nil {
		t.Fatalf("list snapshots after restart: %v", err)
	}

	if snapLast < legacyLast {
		t.Fatalf("checkpoint snapshot index %d must cover the legacy tail (last index %d)", snapLast, legacyLast)
	}

	firstAfter, err := second.logStore.FirstIndex()
	if err != nil {
		t.Fatalf("read first index after restart: %v", err)
	}

	if firstAfter != 0 && firstAfter <= legacyLast {
		t.Fatalf("legacy log tail survived the checkpoint: first index %d, legacy last index %d", firstAfter, legacyLast)
	}
}

func TestStandaloneToHACheckpointPersistsRealignedConfiguration(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	dataDir := t.TempDir()
	a, restores := countingApplier(t)

	standalone, err := NewManager(ctx, FastTestConfig(), a, dataDir)
	if err != nil {
		t.Fatalf("NewManager standalone: %v", err)
	}

	if err := standalone.WaitForLeaderBarrier(ctx); err != nil {
		t.Fatalf("standalone barrier: %v", err)
	}

	proposeSome(t, standalone, 3)

	if err := standalone.Shutdown(); err != nil {
		t.Fatalf("shutdown standalone: %v", err)
	}

	advertise := fmt.Sprintf("127.0.0.1:%d", freePort(t))

	haCfg := FastTestConfig()
	haCfg.Enabled = true
	haCfg.BindAddress = advertise
	haCfg.AdvertiseAddress = advertise

	for boot := 1; boot <= 2; boot++ {
		ha, err := NewManager(ctx, haCfg, a, dataDir)
		if err != nil {
			t.Fatalf("boot %d NewManager HA: %v", boot, err)
		}

		if err := ha.WaitForLeaderBarrier(ctx); err != nil {
			_ = ha.Shutdown()

			t.Fatalf("boot %d barrier: %v", boot, err)
		}

		for !ha.checkpointDone.Load() {
			if ctx.Err() != nil {
				_ = ha.Shutdown()

				t.Fatalf("boot %d: checkpoint never completed", boot)
			}

			time.Sleep(10 * time.Millisecond)
		}

		if got := soleServer(t, ha).Address; string(got) != advertise {
			_ = ha.Shutdown()

			t.Fatalf("boot %d: sole server address %q, want %q", boot, got, advertise)
		}

		state := appliedState(t, a)
		servers := raft.DecodeConfiguration(state.Configuration).Servers

		if len(servers) != 1 || string(servers[0].Address) != advertise {
			_ = ha.Shutdown()

			t.Fatalf("boot %d: fsm_state.configuration = %+v, want the realigned address %q", boot, servers, advertise)
		}

		first, err := ha.logStore.FirstIndex()
		if err != nil {
			_ = ha.Shutdown()

			t.Fatalf("boot %d: read first log index: %v", boot, err)
		}

		if first != 0 && first <= state.ConfigurationIndex {
			_ = ha.Shutdown()

			t.Fatalf("boot %d: the realigned configuration entry %d is still in the log (first index %d); the test no longer proves it is read from fsm_state",
				boot, state.ConfigurationIndex, first)
		}

		if err := ha.Shutdown(); err != nil {
			t.Fatalf("boot %d shutdown: %v", boot, err)
		}
	}

	if n := restores.Load(); n != 0 {
		t.Fatalf("conversion restored the database %d times", n)
	}

	if n := rowCount(t, a); n != 3 {
		t.Fatalf("converted database has %d rows, want 3", n)
	}
}
