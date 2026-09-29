// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"fmt"
)

func migrateV22(ctx context.Context, tx *sql.Tx) error {
	stmts := []string{
		fmt.Sprintf("ALTER TABLE %s ADD COLUMN msisdn TEXT NOT NULL DEFAULT ''", SubscribersTableName),
		fmt.Sprintf("CREATE UNIQUE INDEX idx_subscribers_msisdn ON %s(msisdn) WHERE msisdn != ''", SubscribersTableName),
		fmt.Sprintf(`CREATE TABLE %s (
			singleton   BOOLEAN PRIMARY KEY DEFAULT TRUE,
			smscAddress TEXT    NOT NULL DEFAULT '',
			smscPort    INTEGER NOT NULL DEFAULT %d,
			smsNumber   TEXT    NOT NULL DEFAULT '',
			CHECK (singleton)
		)`, SMSSettingsTableName, DefaultSMSCPort),
	}

	for _, stmt := range stmts {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("add SMS schema: %w", err)
		}
	}

	return nil
}
