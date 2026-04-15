// Copyright 2026 Ella Networks

package db

import (
	"context"
	"database/sql"
	"fmt"
)

const FsmStateTableName = "fsm_state"

// ensureFsmStateTable creates the local-only fsm_state singleton and seeds
// it with lastApplied = 0. The table tracks the Raft index of the last
// successfully applied log entry so that crash-recovery replay can skip
// already-applied entries (changesets are not idempotent).
func ensureFsmStateTable(ctx context.Context, conn *sql.DB) error {
	if _, err := conn.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS fsm_state (
			id INTEGER PRIMARY KEY CHECK(id = 1),
			lastApplied INTEGER NOT NULL DEFAULT 0
		)`); err != nil {
		return fmt.Errorf("create fsm_state table: %w", err)
	}

	if _, err := conn.ExecContext(ctx,
		"INSERT OR IGNORE INTO fsm_state (id, lastApplied) VALUES (1, 0)"); err != nil {
		return fmt.Errorf("seed fsm_state: %w", err)
	}

	return nil
}
