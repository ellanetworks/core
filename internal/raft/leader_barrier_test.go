// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package raft

import (
	"context"
	"sync/atomic"
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

func TestShutdownCancelsLeaderHooksAndWaitsForThem(t *testing.T) {
	t.Parallel()

	applier := newTestApplier(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	mgr, err := NewManager(ctx, FastTestConfig(), applier, t.TempDir())
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	started := make(chan struct{})

	var returned atomic.Bool

	mgr.OnLeadership(func(ctx context.Context) {
		close(started)
		<-ctx.Done()
		time.Sleep(50 * time.Millisecond)
		returned.Store(true)
	})

	select {
	case <-started:
	case <-ctx.Done():
		t.Fatal("leader hook never started")
	}

	done := make(chan error, 1)

	go func() { done <- mgr.Shutdown() }()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("shutdown: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("shutdown did not complete; a leader hook blocked on its context was never cancelled")
	}

	if !returned.Load() {
		t.Fatal("Shutdown returned before the leader hook did")
	}
}

func TestLeaderHooksRunAfterTheTermBarrier(t *testing.T) {
	t.Parallel()

	applier := newTestApplier(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	mgr, err := NewManager(ctx, FastTestConfig(), applier, t.TempDir())
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	t.Cleanup(func() { _ = mgr.Shutdown() })

	barriered := make(chan bool, 2)

	check := func(context.Context) {
		term, _ := mgr.barrierState()
		barriered <- term != 0 && term == mgr.raft.CurrentTerm()
	}

	mgr.OnLeadership(check)

	if err := mgr.WaitForLeaderBarrier(ctx); err != nil {
		t.Fatalf("barrier: %v", err)
	}

	mgr.OnLeadership(check)

	for range 2 {
		select {
		case ok := <-barriered:
			if !ok {
				t.Fatal("leader hook ran before this term's barrier completed")
			}
		case <-ctx.Done():
			t.Fatal("leader hook never ran")
		}
	}

	if err := mgr.WriteBarrier(time.Second); err != nil {
		t.Fatalf("WriteBarrier after the term barrier: %v", err)
	}
}
