// SPDX-FileCopyrightText: 2026-present Ella Networks
// SPDX-License-Identifier: Apache-2.0

package db_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ellanetworks/core/internal/db"
)

func TestGetFlowAccountingSettings_Default(t *testing.T) {
	tempDir := t.TempDir()

	database, err := db.NewDatabase(context.Background(), filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	}()

	enabled, err := database.IsFlowAccountingEnabled(context.Background())
	if err != nil {
		t.Fatalf("Couldn't complete IsFlowAccountingEnabled: %s", err)
	}

	if !enabled {
		t.Fatalf("Flow accounting should be enabled by default")
	}
}

func TestUpdateAndGetFlowAccountingSettings(t *testing.T) {
	tempDir := t.TempDir()

	database, err := db.NewDatabase(context.Background(), filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	}()

	err = database.UpdateFlowAccountingSettings(context.Background(), false)
	if err != nil {
		t.Fatalf("Couldn't complete Update: %s", err)
	}

	enabled, err := database.IsFlowAccountingEnabled(context.Background())
	if err != nil {
		t.Fatalf("Couldn't complete IsFlowAccountingEnabled: %s", err)
	}

	if enabled {
		t.Fatalf("Flow accounting should be disabled")
	}

	err = database.UpdateFlowAccountingSettings(context.Background(), true)
	if err != nil {
		t.Fatalf("Couldn't complete Update: %s", err)
	}

	enabled, err = database.IsFlowAccountingEnabled(context.Background())
	if err != nil {
		t.Fatalf("Couldn't complete IsFlowAccountingEnabled: %s", err)
	}

	if !enabled {
		t.Fatalf("Flow accounting should be enabled")
	}
}

func TestUpdateFlowAccountingSettings_RestartDatabase(t *testing.T) {
	tempDir := t.TempDir()

	database, err := db.NewDatabase(context.Background(), filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}

	err = database.UpdateFlowAccountingSettings(context.Background(), false)
	if err != nil {
		t.Fatalf("Couldn't complete Update: %s", err)
	}

	err = database.Close()
	if err != nil {
		t.Fatalf("Couldn't complete Close: %s", err)
	}

	database, err = db.NewDatabase(context.Background(), filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	}()

	enabled, err := database.IsFlowAccountingEnabled(context.Background())
	if err != nil {
		t.Fatalf("Couldn't complete IsFlowAccountingEnabled: %s", err)
	}

	if enabled {
		t.Fatalf("Flow accounting should be disabled")
	}
}
