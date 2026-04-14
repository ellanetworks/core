// Copyright 2026 Ella Networks

package raft

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb/v2"
)

// NewTestManager spins up a single-node Raft cluster over an in-memory
// transport, tuned for fast unit tests. Mirrors physical/raft/testing.go
// from Vault: no TCP bind, small trailing-log window, low snapshot
// threshold. Tests reach leader in milliseconds and don't compete for ports.
//
// Kept separate from NewStandaloneManager so production can move to a real
// TCP transport (HA prerequisite) without slowing the test suite.
//
// The returned cleanup func is idempotent and is also registered via
// t.Cleanup, so callers can shut down early or rely on automatic teardown.
func NewTestManager(t testing.TB, applier Applier) (*Manager, func()) {
	t.Helper()

	dataDir := t.TempDir()

	raftDir := filepath.Join(dataDir, "raft")
	if err := os.MkdirAll(raftDir, 0o700); err != nil {
		t.Fatalf("create raft directory: %v", err)
	}

	fsm := NewFSM(applier, dataDir)

	cfg := raft.DefaultConfig()
	cfg.LocalID = "1"
	cfg.Logger = newZapRaftLogger()
	cfg.HeartbeatTimeout = 50 * time.Millisecond
	cfg.ElectionTimeout = 50 * time.Millisecond
	cfg.LeaderLeaseTimeout = 50 * time.Millisecond
	cfg.CommitTimeout = 5 * time.Millisecond
	// Small trailing-log window + low snapshot threshold keep
	// snapshot/restore tests under a second.
	cfg.TrailingLogs = 100
	cfg.SnapshotThreshold = 100
	cfg.SnapshotInterval = time.Second

	boltPath := filepath.Join(raftDir, "raft.db")

	boltStore, err := raftboltdb.NewBoltStore(boltPath)
	if err != nil {
		t.Fatalf("create bolt store: %v", err)
	}

	snapshots, err := raft.NewFileSnapshotStore(raftDir, 3, newZapIOWriter("snapshot"))
	if err != nil {
		_ = boltStore.Close()

		t.Fatalf("create snapshot store: %v", err)
	}

	logCache, err := raft.NewLogCache(raftLogCacheSize, boltStore)
	if err != nil {
		_ = boltStore.Close()

		t.Fatalf("create log cache: %v", err)
	}

	addr, transport := raft.NewInmemTransport("")

	r, err := raft.NewRaft(cfg, fsm, logCache, boltStore, snapshots, transport)
	if err != nil {
		_ = boltStore.Close()

		t.Fatalf("create raft: %v", err)
	}

	bootCfg := raft.Configuration{
		Servers: []raft.Server{{ID: "1", Address: addr}},
	}

	if err := r.BootstrapCluster(bootCfg).Error(); err != nil {
		_ = r.Shutdown().Error()
		_ = boltStore.Close()

		t.Fatalf("bootstrap: %v", err)
	}

	m := &Manager{
		raft:      r,
		fsm:       fsm,
		transport: transport,
		logStore:  boltStore,
		snaps:     snapshots,
		config:    ClusterConfig{BindAddress: string(addr)},
		nodeID:    1,
		dataDir:   dataDir,
	}

	if err := waitForLeaderTest(t, m); err != nil {
		_ = r.Shutdown().Error()
		_ = boltStore.Close()

		t.Fatalf("wait for leader: %v", err)
	}

	var (
		once        sync.Once
		shutdownErr error
	)

	cleanup := func() {
		once.Do(func() {
			shutdownErr = m.Shutdown()
		})
	}

	t.Cleanup(func() {
		cleanup()

		if shutdownErr != nil && !errors.Is(shutdownErr, raft.ErrRaftShutdown) {
			t.Errorf("shutdown test manager: %v", shutdownErr)
		}
	})

	return m, cleanup
}

// waitForLeaderTest blocks until the test manager elects itself or the
// test's context is cancelled. Bounded at 5 s as a safety net — with 50 ms
// timeouts and an in-mem transport, election completes in microseconds.
func waitForLeaderTest(t testing.TB, m *Manager) error {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("no leader elected: %w", ctx.Err())
		case isLeader := <-m.raft.LeaderCh():
			if isLeader {
				return nil
			}
		}
	}
}
