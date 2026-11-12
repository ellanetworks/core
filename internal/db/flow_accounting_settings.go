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
	FlowAccountingDefaultEnabled = true
)

const FlowAccountingSettingsTableName = "flow_accounting_settings"

const upsertFlowAccountingSettingsSQL = `
INSERT INTO ` + FlowAccountingSettingsTableName + ` (singleton, enabled) VALUES (TRUE, ?)
ON CONFLICT(singleton) DO UPDATE SET enabled=excluded.enabled;
`

const getFlowAccountingSettingsSQL = `SELECT enabled FROM ` + FlowAccountingSettingsTableName + ` WHERE singleton=TRUE;`

// InitializeFlowAccountingSettings inserts the default flow accounting
// settings row if the singleton row does not yet exist. Idempotent.
func (db *Database) InitializeFlowAccountingSettings(ctx context.Context) error {
	_, err := db.IsFlowAccountingEnabled(ctx)
	if err == nil {
		return nil
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("failed to check flow accounting settings: %w", err)
	}

	return db.UpdateFlowAccountingSettings(ctx, FlowAccountingDefaultEnabled)
}

func (db *Database) IsFlowAccountingEnabled(ctx context.Context) (bool, error) {
	ctx, span, timer := startDBQuery(ctx, FlowAccountingSettingsTableName, "SELECT", "select")
	defer span.End()
	defer timer.ObserveDuration()

	var enabled bool

	err := db.reader().QueryRowContext(ctx, getFlowAccountingSettingsSQL).Scan(&enabled)
	if err != nil {
		recordSpanError(span, err)

		return false, fmt.Errorf("query failed: %w", err)
	}

	return enabled, nil
}

func (db *Database) UpdateFlowAccountingSettings(ctx context.Context, enabled bool) error {
	ctx, span, timer := startDBQuery(ctx, FlowAccountingSettingsTableName, "UPSERT", "update")
	defer span.End()
	defer timer.ObserveDuration()

	_, err := db.applyUpdateFlowAccountingSettings(ctx, &boolPayload{Value: enabled})
	if err != nil {
		recordSpanError(span, err)

		return err
	}

	db.publishOpTopics([]Topic{TopicFlowAccountingSettings})

	return nil
}
