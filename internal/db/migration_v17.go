// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"fmt"
)

func migrateV17(ctx context.Context, tx *sql.Tx) error {
	stmt := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
  singleton BOOLEAN PRIMARY KEY DEFAULT TRUE,
  enabled   BOOLEAN NOT NULL DEFAULT FALSE,
  CHECK (singleton)
);`, LocalSwitchSettingsTableName)

	if _, err := tx.ExecContext(ctx, stmt); err != nil {
		return fmt.Errorf("failed to create local_switch_settings table: %w", err)
	}

	return nil
}
