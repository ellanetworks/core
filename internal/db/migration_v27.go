// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"fmt"
)

func migrateV27(ctx context.Context, tx *sql.Tx) error {
	stmt := fmt.Sprintf("CREATE INDEX idx_ue_registrations_version ON %s (version)", UERegistrationsTableName)

	if _, err := tx.ExecContext(ctx, stmt); err != nil {
		return fmt.Errorf("create index on %s: %w", UERegistrationsTableName, err)
	}

	return nil
}
