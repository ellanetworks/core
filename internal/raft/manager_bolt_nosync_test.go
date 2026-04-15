// Copyright 2026 Ella Networks

package raft

import (
	"context"
	"testing"
	"time"
)

// TestManagerBoltNoSyncStandalone asserts that a standalone manager opens the
// raft log store with fsync disabled. Single-server nodes rely on ella.db's
// own COMMIT fsync for durability; fsyncing the raft log per entry doubles
// write latency and, on slow CI storage, pushed the batched-DELETE cleanup
// between integration test cases past the client's 5s timeout.
func TestManagerBoltNoSyncStandalone(t *testing.T) {
	t.Parallel()

	applier := newTestApplier(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	mgr, err := NewManager(ctx, ClusterConfig{}, applier, t.TempDir())
	if err != nil {
		t.Fatalf("NewManager standalone: %v", err)
	}

	t.Cleanup(func() { _ = mgr.Shutdown() })

	if !mgr.BoltNoSync() {
		t.Fatalf("standalone manager must open raft log store with NoSync=true; got NoSync=false (every raft.Apply pays an extra fsync, doubling write latency)")
	}
}
