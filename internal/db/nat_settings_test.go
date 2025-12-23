// Copyright 2024 Ella Networks

package db_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ellanetworks/core/internal/db"
)

func TestGetNATSettings_Default(t *testing.T) {
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

	enabled, err := database.IsNATEnabled(context.Background())
	if err != nil {
		t.Fatalf("Couldn't complete RetrieveAll: %s", err)
	}

	if !enabled {
		t.Fatalf("NAT should be enabled by default")
	}
}

func TestUpdateAndGetNATSettings(t *testing.T) {
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

	err = database.UpdateNATSettings(context.Background(), false)
	if err != nil {
		t.Fatalf("Couldn't complete Update: %s", err)
	}

	enabled, err := database.IsNATEnabled(context.Background())
	if err != nil {
		t.Fatalf("Couldn't complete RetrieveAll: %s", err)
	}

	if enabled {
		t.Fatalf("NAT should be disabled")
	}

	err = database.UpdateNATSettings(context.Background(), true)
	if err != nil {
		t.Fatalf("Couldn't complete Update: %s", err)
	}

	enabled, err = database.IsNATEnabled(context.Background())
	if err != nil {
		t.Fatalf("Couldn't complete RetrieveAll: %s", err)
	}

	if !enabled {
		t.Fatalf("NAT should be enabled")
	}
}

func TestUpdateNATSettings_RestartDatabase(t *testing.T) {
	tempDir := t.TempDir()

	database, err := db.NewDatabase(context.Background(), filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}

	err = database.UpdateNATSettings(context.Background(), false)
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

	enabled, err := database.IsNATEnabled(context.Background())
	if err != nil {
		t.Fatalf("Couldn't complete RetrieveAll: %s", err)
	}

	if enabled {
		t.Fatalf("NAT should be disabled")
	}
}
