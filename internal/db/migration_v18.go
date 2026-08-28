// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"fmt"
)

// migrateV18 adds a free-text operator note to subscribers. Add-column only:
// the table keeps its existing rows and constraints, and pre-existing rows
// default to the empty string.
func migrateV18(ctx context.Context, tx *sql.Tx) error {
	stmt := fmt.Sprintf("ALTER TABLE %s ADD COLUMN description TEXT NOT NULL DEFAULT ''", SubscribersTableName)
	if _, err := tx.ExecContext(ctx, stmt); err != nil {
		return fmt.Errorf("failed to add description column: %w", err)
	}

	return nil
}
