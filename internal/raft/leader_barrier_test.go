// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package raft

import (
	"context"
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
