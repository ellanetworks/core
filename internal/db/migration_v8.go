// Copyright 2026 Ella Networks

package db

import (
	"context"
	"database/sql"
	"fmt"
)

// ---------------------------------------------------------------------------
// V8 migration — add action to flow reports to enable tracking of dropped flows
// ---------------------------------------------------------------------------

func migrateV8(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx,
		fmt.Sprintf("ALTER TABLE %s ADD COLUMN action INT NOT NULL DEFAULT 0", FlowReportsTableName))
	if err != nil {
		return fmt.Errorf("failed to add action column: %w", err)
	}

	return nil
}
