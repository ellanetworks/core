// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ellanetworks/core/internal/db"
)

type toggleSetting struct {
	name           string
	defaultEnabled bool
	isEnabled      func(*db.Database, context.Context) (bool, error)
	update         func(*db.Database, context.Context, bool) error
}

var toggleSettings = []toggleSetting{
	{
		name:           "NAT",
		defaultEnabled: true,
		isEnabled:      (*db.Database).IsNATEnabled,
		update:         (*db.Database).UpdateNATSettings,
	},
	{
		name:           "flow accounting",
		defaultEnabled: true,
		isEnabled:      (*db.Database).IsFlowAccountingEnabled,
		update:         (*db.Database).UpdateFlowAccountingSettings,
	},
	{
		name:           "local switch",
		defaultEnabled: false,
		isEnabled:      (*db.Database).IsLocalSwitchEnabled,
		update:         (*db.Database).UpdateLocalSwitchSettings,
	},
}

func assertToggle(t *testing.T, s toggleSetting, database *db.Database, want bool) {
	t.Helper()

	enabled, err := s.isEnabled(database, context.Background())
	if err != nil {
		t.Fatalf("%s: read setting: %s", s.name, err)
	}

	if enabled != want {
		t.Fatalf("%s enabled = %v, want %v", s.name, enabled, want)
	}
}

func TestToggleSettings_Default(t *testing.T) {
	for _, s := range toggleSettings {
		t.Run(s.name, func(t *testing.T) {
			assertToggle(t, s, setupTestDB(t), s.defaultEnabled)
		})
	}
}

func TestToggleSettings_UpdateAndGet(t *testing.T) {
	for _, s := range toggleSettings {
		t.Run(s.name, func(t *testing.T) {
			database := setupTestDB(t)

			for _, want := range []bool{!s.defaultEnabled, s.defaultEnabled} {
				if err := s.update(database, context.Background(), want); err != nil {
					t.Fatalf("%s: update to %v: %s", s.name, want, err)
				}

				assertToggle(t, s, database, want)
			}
		})
	}
}

func TestToggleSettings_SurviveRestart(t *testing.T) {
	for _, s := range toggleSettings {
		t.Run(s.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "db.sqlite3")
			want := !s.defaultEnabled

			database, err := db.NewDatabaseWithoutRaft(context.Background(), path)
			if err != nil {
				t.Fatalf("Couldn't complete NewDatabase: %s", err)
			}

			if err := s.update(database, context.Background(), want); err != nil {
				t.Fatalf("%s: update to %v: %s", s.name, want, err)
			}

			if err := database.Close(); err != nil {
				t.Fatalf("Couldn't complete Close: %s", err)
			}

			reopened, err := db.NewDatabaseWithoutRaft(context.Background(), path)
			if err != nil {
				t.Fatalf("Couldn't reopen database: %s", err)
			}

			t.Cleanup(func() {
				if err := reopened.Close(); err != nil {
					t.Fatalf("Couldn't complete Close: %s", err)
				}
			})

			assertToggle(t, s, reopened, want)
		})
	}
}
