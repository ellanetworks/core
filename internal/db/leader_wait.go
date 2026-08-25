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

	// A node that restored from a snapshot holds the snapshot's state until the
	// leader barrier clears, so a read taken before it misses every log entry
	// the snapshot does not cover.
	if err := db.WaitForFSMCatchUp(ctx); err != nil {
		return fmt.Errorf("wait for the state machine to catch up: %w", err)
	}

	return db.WaitForInitialization(ctx, 0)
}

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
