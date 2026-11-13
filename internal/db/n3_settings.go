// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

const (
	N3DefaultExternalAddress = ""
)

const N3SettingsTableName = "n3_settings"

const upsertN3SettingsSQL = `
INSERT INTO ` + N3SettingsTableName + ` (singleton, external_address) VALUES (TRUE, ?)
ON CONFLICT(singleton) DO UPDATE SET external_address=excluded.external_address;
`

const getN3SettingsSQL = `SELECT external_address FROM ` + N3SettingsTableName + ` WHERE singleton=TRUE;`

type N3Settings struct {
	ExternalAddress string
}

// InitializeN3Settings inserts the default N3 settings row if the
// singleton row does not yet exist. Idempotent.
func (db *Database) InitializeN3Settings(ctx context.Context) error {
	_, err := db.GetN3Settings(ctx)
	if err == nil {
		return nil
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("failed to check N3 settings: %w", err)
	}

	return db.UpdateN3Settings(ctx, N3DefaultExternalAddress)
}

func (db *Database) UpdateN3Settings(ctx context.Context, externalAddress string) (err error) {
	ctx, finish := startDBQuery(ctx, N3SettingsTableName, "UPSERT")
	defer func() { finish(err) }()

	_, err = db.applyUpdateN3Settings(ctx, &stringPayload{Value: externalAddress})
	if err != nil {
		return err
	}

	db.publishOpTopics([]Topic{TopicN3Settings})

	return nil
}

func (db *Database) GetN3Settings(ctx context.Context) (_ *N3Settings, err error) {
	querySummary := fmt.Sprintf("%s %s", "SELECT", N3SettingsTableName)

	ctx, finish := startDBQueryNamed(ctx, querySummary, N3SettingsTableName, "SELECT")
	defer func() { finish(err) }()

	var n3Settings N3Settings

	if err := db.reader().QueryRowContext(ctx, getN3SettingsSQL).Scan(&n3Settings.ExternalAddress); err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return &n3Settings, nil
}
