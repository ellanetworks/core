// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/ellanetworks/core/internal/models"
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
		state       INTEGER NOT NULL CHECK (state IN (0, 1, 2)),
		serverName  TEXT    NOT NULL,
		authPending INTEGER NOT NULL CHECK (authPending IN (0, 1)),
		originHost  TEXT    NOT NULL,
		originRealm TEXT    NOT NULL,
		updatedAt   INTEGER NOT NULL,
		CHECK (state != 0 OR authPending = 1)
	)`, IMSRegistrationsTableName)

	if _, err := tx.ExecContext(ctx, stmt); err != nil {
		return fmt.Errorf("create %s: %w", IMSRegistrationsTableName, err)
	}

	stmt = fmt.Sprintf(`UPDATE %s SET var5qi = %d WHERE var5qi != %d AND dataNetworkID IN (SELECT id FROM %s WHERE name = '%s')`,
		PoliciesTableName, models.IMSSignalling5QI, models.IMSSignalling5QI, DataNetworksTableName, models.IMSDataNetworkName)

	if _, err := tx.ExecContext(ctx, stmt); err != nil {
		return fmt.Errorf("set 5QI %d on the policies of the %s data network: %w", models.IMSSignalling5QI, models.IMSDataNetworkName, err)
	}

	return nil
}
