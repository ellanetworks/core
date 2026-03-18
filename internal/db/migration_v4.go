// Copyright 2026 Ella Networks

package db

import (
	"context"
	"database/sql"
	"fmt"
)

// ---------------------------------------------------------------------------
// V4 migration — adds SPN (Service Provider Name) columns to operator table.
// ---------------------------------------------------------------------------

func migrateV4(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx,
		fmt.Sprintf("ALTER TABLE %s ADD COLUMN spnFullName TEXT NOT NULL DEFAULT 'Ella Networks'", OperatorTableName))
	if err != nil {
		return fmt.Errorf("failed to add spnFullName column: %w", err)
	}

	_, err = tx.ExecContext(ctx,
		fmt.Sprintf("ALTER TABLE %s ADD COLUMN spnShortName TEXT NOT NULL DEFAULT 'Ella'", OperatorTableName))
	if err != nil {
		return fmt.Errorf("failed to add spnShortName column: %w", err)
	}

	return nil
}
