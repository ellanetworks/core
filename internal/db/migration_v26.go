// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"fmt"
)

func migrateV26(ctx context.Context, tx *sql.Tx) error {
	for _, idx := range []struct{ name, table, column string }{
		{"idx_daily_usage_imsi", DailyUsageTableName, "imsi"},
		{"idx_ue_registrations_version", UERegistrationsTableName, "version"},
	} {
		stmt := fmt.Sprintf("CREATE INDEX %s ON %s (%s)", idx.name, idx.table, idx.column)

		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("create index on %s: %w", idx.table, err)
		}
	}

	return nil
}
