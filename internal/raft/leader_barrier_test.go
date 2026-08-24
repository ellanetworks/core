// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package raft

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestStandaloneRestartBarriersBeforeReadsAreServed(t *testing.T) {
	t.Parallel()

	applier := newTestApplier(t)
	dataDir := t.TempDir()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	first, err := NewManager(ctx, FastTestConfig(), applier, dataDir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	if err := first.WaitForLeaderBarrier(ctx); err != nil {
		t.Fatalf("first barrier: %v", err)
	}

	for i := range 500 {
		cmd, err := NewCommand(CmdChangeset, map[string]int{"n": i})
		if err != nil {
			t.Fatalf("NewCommand: %v", err)
		}

		if _, err := first.Propose(cmd, first.ProposeTimeout()); err != nil {
			t.Fatalf("propose %d: %v", i, err)
		}
	}

	committed := first.AppliedIndex()

	if err := first.Shutdown(); err != nil {
		t.Fatalf("shutdown: %v", err)
	}

	second, err := NewManager(ctx, FastTestConfig(), applier, dataDir)
	if err != nil {
		t.Fatalf("NewManager (reopen): %v", err)
	}

	t.Cleanup(func() { _ = second.Shutdown() })

	if err := second.WaitForLeaderBarrier(ctx); err != nil {
		t.Fatalf("post-restart barrier never completed: %v", err)
	}

	if got := second.AppliedIndex(); got < committed {
		t.Fatalf("FSM still lags the committed log after the barrier: applied %d, committed before restart %d", got, committed)
	}
}

// TestBarrierForLeadershipAbortsOnShutdown pins the ordering hazard in
// Manager.Shutdown: it stops the observer before shutting raft down, so a
// leadership barrier still waiting on the FSM would hold shutdown open.
func TestBarrierForLeadershipAbortsOnShutdown(t *testing.T) {
	t.Parallel()

	applier := newTestApplier(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	mgr, err := NewManager(ctx, FastTestConfig(), applier, t.TempDir())
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	if err := mgr.WaitForLeaderBarrier(ctx); err != nil {
		t.Fatalf("barrier: %v", err)
	}

	// Install a barrier attempt that never completes, standing in for an FSM
	// still draining a large replay backlog.
	mgr.barrieredTerm.Store(0)

	mgr.barrierMu.Lock()
	mgr.barrier = &barrierAttempt{term: mgr.raft.CurrentTerm(), done: make(chan struct{})}
	mgr.barrierMu.Unlock()

	done := make(chan error, 1)

	go func() { done <- mgr.barrierForLeadership() }()

	// Let the waiter park on the barrier before shutdown overtakes it.
	time.Sleep(200 * time.Millisecond)

	if err := mgr.Shutdown(); err != nil {
		t.Fatalf("shutdown: %v", err)
	}

	select {
	case err := <-done:
		if !errors.Is(err, errShuttingDown) {
			t.Fatalf("want errShuttingDown, got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("barrier ignored shutdown; Manager.Shutdown stops the observer first, so this deadlocks shutdown")
	}
}
