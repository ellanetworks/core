// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"fmt"
	"time"
)

const (
	readyWaitTimeout            = 30 * time.Second
	defaultHoldForLeaderTimeout = 5 * time.Second
)

func (db *Database) HasLeader() bool {
	if db.raftManager == nil {
		return true
	}

	return db.raftManager.HasLeader()
}

func (db *Database) WaitForLeader(ctx context.Context) error {
	if db.raftManager == nil {
		return nil
	}

	return db.raftManager.WaitForLeader(ctx)
}

// WaitForFSMCatchUp blocks until local reads reflect every committed write.
// A standalone node is the only voter, so it always elects itself and the
// post-leadership barrier is guaranteed to land. An HA follower has no way to
// force its FSM forward, so it does not wait; its leader barriers on election.
func (db *Database) WaitForFSMCatchUp(ctx context.Context) error {
	if db.raftManager == nil || db.raftManager.ClusterEnabled() {
		return nil
	}

	return db.raftManager.WaitForLeaderBarrier(ctx)
}

func (db *Database) WaitUntilReady(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, readyWaitTimeout)
	defer cancel()

	if db.raftManager != nil {
		if err := db.raftManager.HoldForLeader(ctx, readyWaitTimeout); err != nil {
			return fmt.Errorf("wait for raft leader: %w", err)
		}
	}

	return db.WaitForInitialization(ctx, 0)
}

// holdForLeader waits out an in-progress election before a write is
// dispatched. It is a pure wait with no side effects, so it honours the
// caller's context: a client that has gone away, or a shutdown, stops the
// wait instead of pinning the request for the full budget. Exhausting the
// budget is reported as a transient failure so the caller does not go on to
// spend a second budget forwarding to a leader that does not exist.
func (db *Database) holdForLeader(ctx context.Context) error {
	if db.raftManager == nil || db.raftManager.HasLeader() {
		return nil
	}

	timeout := db.proposeTimeout
	if timeout <= 0 {
		timeout = defaultHoldForLeaderTimeout
	}

	if err := db.raftManager.HoldForLeader(ctx, timeout); err != nil {
		return fmt.Errorf("%w: no raft leader: %v", ErrProposeTimeout, err)
	}

	return nil
}
