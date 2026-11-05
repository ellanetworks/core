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
		if err := db.raftManager.WaitForLeader(ctx); err != nil {
			return fmt.Errorf("wait for raft leader: %w", err)
		}
	}

	if err := db.WaitForFSMCatchUp(ctx); err != nil {
		return fmt.Errorf("wait for the state machine to catch up: %w", err)
	}

	if err := db.WaitForStandaloneSchema(ctx); err != nil {
		return fmt.Errorf("wait for schema migrations: %w", err)
	}

	return db.WaitForInitialization(ctx, 0)
}

func (db *Database) WaitForStandaloneSchema(ctx context.Context) error {
	if db.raftManager == nil || db.raftManager.ClusterEnabled() {
		return nil
	}

	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		applied, err := db.CurrentSchemaVersion(ctx)
		if err != nil {
			return fmt.Errorf("read schema version: %w", err)
		}

		if applied >= SchemaVersion() {
			return nil
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("standalone shared schema still at %d, want >= %d: %w", applied, SchemaVersion(), ctx.Err())
		case <-ticker.C:
		}
	}
}

func (db *Database) holdForLeader(ctx context.Context) error {
	if db.raftManager == nil || db.raftManager.HasLeader() {
		return nil
	}

	timeout := db.proposeTimeout
	if timeout <= 0 {
		timeout = defaultHoldForLeaderTimeout
	}

	holdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if err := db.raftManager.WaitForLeader(holdCtx); err != nil {
		return fmt.Errorf("%w: no raft leader: %v", ErrProposeTimeout, err)
	}

	return nil
}
