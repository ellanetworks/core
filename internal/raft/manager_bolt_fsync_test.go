// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package raft

import (
	"context"
	"testing"
	"time"
)

func TestManagerBoltFsyncStandalone(t *testing.T) {
	t.Parallel()

	applier := newTestApplier(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	mgr, err := NewManager(ctx, FastTestConfig(), applier, t.TempDir())
	if err != nil {
		t.Fatalf("NewManager standalone: %v", err)
	}

	t.Cleanup(func() { _ = mgr.Shutdown() })

	if mgr.BoltNoSync() {
		t.Fatalf("standalone manager must open raft log store with NoSync=false; got NoSync=true (an acknowledged write would have no durable copy: ella.db runs WAL with synchronous=NORMAL, which does not fsync on COMMIT)")
	}
}
