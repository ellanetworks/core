// Copyright 2026 Ella Networks

package db

import (
	"context"
	"database/sql"
	"fmt"
)

// ---------------------------------------------------------------------------
// V3 migration — add radio_name column to network_logs.
// ---------------------------------------------------------------------------

func migrateV3(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx,
		fmt.Sprintf("ALTER TABLE %s ADD COLUMN radio_name TEXT NOT NULL DEFAULT ''", RadioEventsTableName))
	if err != nil {
		return fmt.Errorf("failed to add radio_name column: %w", err)
	}

	_, err = tx.ExecContext(ctx,
		fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_network_logs_radio_name ON %s(radio_name)", RadioEventsTableName))
	if err != nil {
		return fmt.Errorf("failed to create radio_name index: %w", err)
	}

	return nil
}
