// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ellanetworks/core/internal/db"
)

func TestGetLocalSwitchSettings_Default(t *testing.T) {
	tempDir := t.TempDir()

	database, err := db.NewDatabaseWithoutRaft(context.Background(), filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	}()

	enabled, err := database.IsLocalSwitchEnabled(context.Background())
	if err != nil {
		t.Fatalf("Couldn't complete IsLocalSwitchEnabled: %s", err)
	}

	if enabled {
		t.Fatalf("Local switch should be disabled by default")
	}
}

func TestUpdateAndGetLocalSwitchSettings(t *testing.T) {
	tempDir := t.TempDir()

	database, err := db.NewDatabaseWithoutRaft(context.Background(), filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	}()

	err = database.UpdateLocalSwitchSettings(context.Background(), true)
	if err != nil {
		t.Fatalf("Couldn't complete Update: %s", err)
	}

	enabled, err := database.IsLocalSwitchEnabled(context.Background())
	if err != nil {
		t.Fatalf("Couldn't complete IsLocalSwitchEnabled: %s", err)
	}

	if !enabled {
		t.Fatalf("Local switch should be enabled")
	}

	err = database.UpdateLocalSwitchSettings(context.Background(), false)
	if err != nil {
		t.Fatalf("Couldn't complete Update: %s", err)
	}

	enabled, err = database.IsLocalSwitchEnabled(context.Background())
	if err != nil {
		t.Fatalf("Couldn't complete IsLocalSwitchEnabled: %s", err)
	}

	if enabled {
		t.Fatalf("Local switch should be disabled")
	}
}

func TestUpdateLocalSwitchSettings_RestartDatabase(t *testing.T) {
	tempDir := t.TempDir()

	database, err := db.NewDatabaseWithoutRaft(context.Background(), filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}

	err = database.UpdateLocalSwitchSettings(context.Background(), true)
	if err != nil {
		t.Fatalf("Couldn't complete Update: %s", err)
	}

	err = database.Close()
	if err != nil {
		t.Fatalf("Couldn't complete Close: %s", err)
	}

	database, err = db.NewDatabaseWithoutRaft(context.Background(), filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	}()

	enabled, err := database.IsLocalSwitchEnabled(context.Background())
	if err != nil {
		t.Fatalf("Couldn't complete IsLocalSwitchEnabled: %s", err)
	}

	if !enabled {
		t.Fatalf("Local switch should be enabled after restart")
	}
}
