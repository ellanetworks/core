// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"fmt"
)

func migrateV23(ctx context.Context, tx *sql.Tx) error {
	stmt := fmt.Sprintf(`CREATE TABLE %s (
		priority INTEGER PRIMARY KEY CHECK (priority >= 0),
		address  TEXT    NOT NULL UNIQUE
	)`, PCSCFAddressesTableName)

	if _, err := tx.ExecContext(ctx, stmt); err != nil {
		return fmt.Errorf("create %s: %w", PCSCFAddressesTableName, err)
	}

	stmt = fmt.Sprintf(`CREATE TABLE %s (
		imsi        TEXT    PRIMARY KEY REFERENCES subscribers(imsi) ON DELETE CASCADE,
		state       INTEGER NOT NULL,
		serverName  TEXT    NOT NULL,
		authPending INTEGER NOT NULL,
		originHost  TEXT    NOT NULL,
		originRealm TEXT    NOT NULL,
		updatedAt   INTEGER NOT NULL
	)`, IMSRegistrationsTableName)

	if _, err := tx.ExecContext(ctx, stmt); err != nil {
		return fmt.Errorf("create %s: %w", IMSRegistrationsTableName, err)
	}

	return nil
}
