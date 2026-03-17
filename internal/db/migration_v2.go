// Copyright 2026 Ella Networks

package db

import (
	"context"
	"database/sql"
	"fmt"
)

// ---------------------------------------------------------------------------
// V2 migration — adds NAS security algorithm configuration columns to operator.
// ---------------------------------------------------------------------------

func migrateV2(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx,
		fmt.Sprintf("ALTER TABLE %s ADD COLUMN ciphering TEXT NOT NULL DEFAULT '[\"NEA2\",\"NEA1\",\"NEA0\"]'", OperatorTableName))
	if err != nil {
		return fmt.Errorf("failed to add ciphering column: %w", err)
	}

	_, err = tx.ExecContext(ctx,
		fmt.Sprintf("ALTER TABLE %s ADD COLUMN integrity TEXT NOT NULL DEFAULT '[\"NIA2\",\"NIA1\",\"NIA0\"]'", OperatorTableName))
	if err != nil {
		return fmt.Errorf("failed to add integrity column: %w", err)
	}

	return nil
}
