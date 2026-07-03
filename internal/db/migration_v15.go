// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"fmt"
)

// migrateV15 creates the positioning_sessions table for LMF session tracking
// and the cell_positions table for LMF Cell-ID/E-CID antenna coordinates.
func migrateV15(ctx context.Context, tx *sql.Tx) error {
	stmt := fmt.Sprintf(`CREATE TABLE %s (
		id TEXT PRIMARY KEY,
		supi TEXT NOT NULL,
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

	cellStmt := fmt.Sprintf(`CREATE TABLE %s (
		id TEXT PRIMARY KEY,
		rat TEXT NOT NULL,
		mcc TEXT NOT NULL,
		mnc TEXT NOT NULL,
		cell_identity TEXT NOT NULL,
		gnb_id TEXT,
		latitude REAL NOT NULL,
		longitude REAL NOT NULL,
		altitude REAL,
		uncertainty_semi_major REAL,
		uncertainty_semi_minor REAL,
		orientation_major INTEGER,
		confidence INTEGER,
		source TEXT NOT NULL DEFAULT 'provisioned',
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)`, CellPositionsTableName)

	if _, err := tx.ExecContext(ctx, cellStmt); err != nil {
		return fmt.Errorf("failed to create cell_positions table: %w", err)
	}

	// A cell (rat + PLMN + cell identity) may only be provisioned once.
	if _, err := tx.ExecContext(ctx, fmt.Sprintf(
		"CREATE UNIQUE INDEX idx_cell_positions_cell ON %s(rat, mcc, mnc, cell_identity)",
		CellPositionsTableName)); err != nil {
		return fmt.Errorf("create cell_positions unique cell index: %w", err)
	}

	return nil
}
