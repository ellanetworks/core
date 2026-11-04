// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"fmt"
)

func migrateV26(ctx context.Context, tx *sql.Tx) error {
	stmt := fmt.Sprintf("CREATE INDEX idx_daily_usage_imsi ON %s (imsi)", DailyUsageTableName)

	if _, err := tx.ExecContext(ctx, stmt); err != nil {
		return fmt.Errorf("create index on %s: %w", DailyUsageTableName, err)
	}

	return nil
}
