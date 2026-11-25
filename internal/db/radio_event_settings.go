// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

const RadioEventSettingsTableName = "radio_event_settings"

const RadioEventsDefaultEnabled = true

const (
	getRadioEventSettingsSQL    = "SELECT enabled FROM " + RadioEventSettingsTableName + " WHERE singleton=TRUE"
	upsertRadioEventSettingsSQL = "INSERT INTO " + RadioEventSettingsTableName + " (singleton, enabled) VALUES (TRUE, ?) ON CONFLICT(singleton) DO UPDATE SET enabled=excluded.enabled"
)

func (db *Database) IsRadioEventsEnabled(ctx context.Context) (_ bool, err error) {
	ctx, finish := startDBQuery(ctx, RadioEventSettingsTableName, "SELECT")
	defer func() { finish(err) }()

	var enabled bool

	err = db.reader().QueryRowContext(ctx, getRadioEventSettingsSQL).Scan(&enabled)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return RadioEventsDefaultEnabled, nil
		}

		return false, fmt.Errorf("query failed: %w", err)
	}

	return enabled, nil
}

func (db *Database) UpdateRadioEventSettings(ctx context.Context, enabled bool) (err error) {
	ctx, finish := startDBQuery(ctx, RadioEventSettingsTableName, "UPSERT")
	defer func() { finish(err) }()

	if _, err := opUpdateRadioEventSettings.Invoke(ctx, db, &boolPayload{Value: enabled}); err != nil {
		return err
	}

	return nil
}

func (db *Database) applyUpdateRadioEventSettings(ctx context.Context, p *boolPayload) (any, error) {
	if _, err := db.runner(ctx).ExecContext(ctx, upsertRadioEventSettingsSQL, p.Value); err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}
