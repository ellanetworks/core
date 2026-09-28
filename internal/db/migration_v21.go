// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"fmt"
)

func migrateV21(ctx context.Context, tx *sql.Tx) error {
	stmts := []string{
		fmt.Sprintf(`CREATE TABLE %s (
			imsi             TEXT    NOT NULL REFERENCES subscribers(imsi) ON DELETE CASCADE,
			type             TEXT    NOT NULL,
			nodeID           TEXT    NOT NULL,
			purged           INTEGER NOT NULL DEFAULT 0,
			registrationTime INTEGER NOT NULL,
			PRIMARY KEY (imsi, type)
		)`, UERegistrationsTableName),
		fmt.Sprintf("CREATE INDEX idx_ue_registrations_node ON %s(nodeID)", UERegistrationsTableName),
	}

	for _, stmt := range stmts {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("create %s: %w", UERegistrationsTableName, err)
		}
	}

	return nil
}
