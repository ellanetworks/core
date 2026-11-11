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
	BGPDefaultEnabled       = false
	BGPDefaultLocalAS       = 64512
	BGPDefaultRouterID      = ""
	BGPDefaultListenAddress = ":179"
)

// defaultBGPSettings is the single source of truth for the documented
// BGP defaults. InitializeBGPSettings seeds the singleton row with these
// values when the table is empty.
func defaultBGPSettings() *BGPSettings {
	return &BGPSettings{
		Enabled:       BGPDefaultEnabled,
		LocalAS:       BGPDefaultLocalAS,
		RouterID:      BGPDefaultRouterID,
		ListenAddress: BGPDefaultListenAddress,
	}
}

const BGPSettingsTableName = "bgp_settings"

const upsertBGPSettingsStmt = `
INSERT INTO ` + BGPSettingsTableName + ` (singleton, enabled, localAS, routerID, listenAddress) VALUES (TRUE, ?, ?, ?, ?)
ON CONFLICT(singleton) DO UPDATE SET enabled=excluded.enabled, localAS=excluded.localAS, routerID=excluded.routerID, listenAddress=excluded.listenAddress;
`

const getBGPSettingsStmt = `SELECT enabled, localAS, routerID, listenAddress FROM ` + BGPSettingsTableName + ` WHERE singleton=TRUE;`

type BGPSettings struct {
	Enabled       bool
	LocalAS       int
	RouterID      string
	ListenAddress string
}

// InitializeBGPSettings inserts the default BGP settings row if the
// singleton row does not yet exist. Idempotent: an existing row (whether
// holding the default or an operator-set value) is left untouched.
func (db *Database) InitializeBGPSettings(ctx context.Context) error {
	_, err := db.GetBGPSettings(ctx)
	if err == nil {
		return nil
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("failed to check BGP settings: %w", err)
	}

	return db.UpdateBGPSettings(ctx, defaultBGPSettings())
}

func (db *Database) GetBGPSettings(ctx context.Context) (*BGPSettings, error) {
	ctx, span, timer := startDBQuery(ctx, BGPSettingsTableName, "SELECT", "select")
	defer span.End()
	defer timer.ObserveDuration()

	var bgpSettings BGPSettings

	err := db.reader().QueryRowContext(ctx, getBGPSettingsStmt).Scan(&bgpSettings.Enabled, &bgpSettings.LocalAS, &bgpSettings.RouterID, &bgpSettings.ListenAddress)
	if err != nil {
		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return &bgpSettings, nil
}

func (db *Database) IsBGPEnabled(ctx context.Context) (bool, error) {
	settings, err := db.GetBGPSettings(ctx)
	if err != nil {
		return false, err
	}

	return settings.Enabled, nil
}

func (db *Database) UpdateBGPSettings(ctx context.Context, settings *BGPSettings) error {
	ctx, span, timer := startDBQuery(ctx, BGPSettingsTableName, "UPSERT", "update")
	defer span.End()
	defer timer.ObserveDuration()

	_, err := db.applyUpdateBGPSettings(ctx, settings)
	if err != nil {
		recordSpanError(span, err)

		return err
	}

	db.publishOpTopics([]Topic{TopicBGPSettings})

	return nil
}
