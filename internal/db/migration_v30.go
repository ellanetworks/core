// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"fmt"
)

func migrateV30(ctx context.Context, tx *sql.Tx) error {
	stmt := fmt.Sprintf(`CREATE TABLE %s (
		singleton BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (singleton),
		enabled   BOOLEAN NOT NULL
	)`, RadioEventSettingsTableName)

	if _, err := tx.ExecContext(ctx, stmt); err != nil {
		return fmt.Errorf("add radio event settings: %w", err)
	}

	return nil
}
