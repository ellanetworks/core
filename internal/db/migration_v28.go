// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"fmt"
)

func migrateV28(ctx context.Context, tx *sql.Tx) error {
	stmts := []string{
		fmt.Sprintf("ALTER TABLE %s ADD COLUMN radio_ref TEXT NOT NULL DEFAULT ''", RadioEventsTableName),
		fmt.Sprintf("CREATE INDEX idx_network_logs_radio_ref ON %s (radio_ref)", RadioEventsTableName),
	}

	for _, stmt := range stmts {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("add radio_ref to network_logs: %w", err)
		}
	}

	return nil
}
