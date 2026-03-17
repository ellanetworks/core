// Copyright 2026 Ella Networks

package db

import (
	"context"
	"database/sql"
	"fmt"
)

// ---------------------------------------------------------------------------
// V4 migration — rename NAS security algorithm columns.
// ---------------------------------------------------------------------------

func migrateV4(ctx context.Context, tx *sql.Tx) error {
	stmts := []string{
		fmt.Sprintf("ALTER TABLE %s RENAME COLUMN cipheringOrder TO ciphering", OperatorTableName),
		fmt.Sprintf("ALTER TABLE %s RENAME COLUMN integrityOrder TO integrity", OperatorTableName),
	}

	for _, stmt := range stmts {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("failed to execute %q: %w", stmt, err)
		}
	}

	return nil
}
