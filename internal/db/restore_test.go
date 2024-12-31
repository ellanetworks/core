package db_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ellanetworks/core/internal/db"
)

func TestRestore(t *testing.T) {
	tempDir := t.TempDir()

	databasePath := filepath.Join(tempDir, "db.sqlite3")
	backupFilePath := filepath.Join(tempDir, "backup.db")
	database, err := db.NewDatabase(databasePath)
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("failed to close database: %v", err)
		}
	}()

	if err := os.WriteFile(databasePath, []byte("dummy data"), 0644); err != nil {
		t.Fatalf("failed to write dummy data to database: %v", err)
	}

	if err := os.WriteFile(backupFilePath, []byte("backup data"), 0644); err != nil {
		t.Fatalf("failed to create backup file: %v", err)
	}

	err = database.Restore(backupFilePath)
	if err != nil {
		t.Fatalf("Restore failed: %v", err)
	}

	restoredData, err := os.ReadFile(databasePath)
	if err != nil {
		t.Fatalf("failed to read restored database: %v", err)
	}

	backupData, err := os.ReadFile(backupFilePath)
	if err != nil {
		t.Fatalf("failed to read backup file: %v", err)
	}

	if string(restoredData) != string(backupData) {
		t.Fatalf("restored data does not match backup data")
	}
}
