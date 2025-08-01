// Copyright 2024 Ella Networks

package db_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ellanetworks/core/internal/db"
)

func TestDatabaseBackup(t *testing.T) {
	tempDir := t.TempDir()

	dbPath := filepath.Join(tempDir, "db.sqlite3")

	database, err := db.NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("Couldn't initialize NewDatabase: %s", err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't close database: %s", err)
		}
	}()

	err = database.UpdateOperatorID(context.Background(), "123", "456")
	if err != nil {
		t.Fatalf("Couldn't update operator id: %s", err)
	}

	tmpFile, err := os.CreateTemp("", "backup_*.db")
	if err != nil {
		t.Fatalf("Couldn't create temp file for backup: %s", err)
	}
	defer func() {
		tmpFile.Close()
		os.Remove(tmpFile.Name()) // Ensure cleanup
	}()

	err = database.Backup(tmpFile)
	if err != nil {
		t.Fatalf("Couldn't create backup: %s", err)
	}

	tmpFileInfo, err := tmpFile.Stat()
	if err != nil {
		t.Fatalf("Couldn't stat backup file: %s", err)
	}

	if tmpFileInfo.Size() == 0 {
		t.Fatalf("Backup file is empty")
	}

	originalFileInfo, err := os.Stat(dbPath)
	if err != nil {
		t.Fatalf("Couldn't stat original database file: %s", err)
	}

	if originalFileInfo.Size() != tmpFileInfo.Size() {
		t.Fatalf("Backup file size mismatch: expected %d, got %d", originalFileInfo.Size(), tmpFileInfo.Size())
	}
}
