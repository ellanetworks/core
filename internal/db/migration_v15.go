// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"fmt"
)

// migrateV15 creates the positioning_sessions table for LMF session tracking.
func migrateV15(ctx context.Context, tx *sql.Tx) error {
	stmt := fmt.Sprintf(`CREATE TABLE %s (
		id TEXT PRIMARY KEY,
		supi TEXT NOT NULL,
		amf_id TEXT NOT NULL,
		session_type INTEGER NOT NULL DEFAULT 0,
		method TEXT NOT NULL DEFAULT 'cell_id',
		qos_response_time_ms INTEGER,
		qos_horizontal_accuracy_m INTEGER,
		status INTEGER NOT NULL DEFAULT 0,
		last_result TEXT,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)`, PositioningSessionsTableName)

	if _, err := tx.ExecContext(ctx, stmt); err != nil {
		return fmt.Errorf("failed to create positioning_sessions table: %w", err)
	}

	if _, err := tx.ExecContext(ctx, fmt.Sprintf(
		"CREATE INDEX idx_pos_sessions_supi ON %s(supi)", PositioningSessionsTableName)); err != nil {
		return fmt.Errorf("create positioning_sessions supi index: %w", err)
	}

	if _, err := tx.ExecContext(ctx, fmt.Sprintf(
		"CREATE INDEX idx_pos_sessions_status ON %s(status)", PositioningSessionsTableName)); err != nil {
		return fmt.Errorf("create positioning_sessions status index: %w", err)
	}

	return nil
}
