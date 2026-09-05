// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package raft

import (
	"context"
	"testing"
	"time"
)

func TestWaitForLeaderDoesNotConsumeLeaderCh(t *testing.T) {
	t.Parallel()

	applier := newTestApplier(t)

	cfg := ClusterConfig{
		HeartbeatTimeout:   2 * time.Second,
		ElectionTimeout:    2 * time.Second,
		LeaderLeaseTimeout: 1 * time.Second,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	mgr, err := NewManager(ctx, cfg, applier, t.TempDir())
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	t.Cleanup(func() { _ = mgr.Shutdown() })

	mgr.LeaderObserver().Stop()

	if err := mgr.WaitForLeader(ctx); err != nil {
		t.Fatalf("WaitForLeader: %v", err)
	}

	select {
	case isLeader := <-mgr.raft.LeaderCh():
		if !isLeader {
			t.Fatal("expected the pending notification to report leadership")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("waitForLeader took the transition off raft.LeaderCh(); raft delivers each value to exactly one receiver, so LeaderObserver must be its sole consumer or leadership callbacks are lost")
	}
}
