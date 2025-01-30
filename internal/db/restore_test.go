// Copyright 2024 Ella Networks

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
	database, err := db.NewDatabase(databasePath, initialOperator)
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("failed to close database: %v", err)
		}
	}()

	dummyData := []byte("dummy data")
	if err := os.WriteFile(databasePath, dummyData, 0o644); err != nil {
		t.Fatalf("failed to write dummy data to database: %v", err)
	}

	backupFile, err := os.CreateTemp("", "backup_*.db")
	if err != nil {
		t.Fatalf("failed to create temporary backup file: %v", err)
	}
	defer func() {
		backupFile.Close()
		os.Remove(backupFile.Name()) // Ensure cleanup
	}()

	backupData := []byte("backup data")
	if _, err := backupFile.Write(backupData); err != nil {
		t.Fatalf("failed to write backup data: %v", err)
	}

	if _, err := backupFile.Seek(0, 0); err != nil {
		t.Fatalf("failed to reset backup file pointer: %v", err)
	}

	err = database.Restore(backupFile)
	if err != nil {
		t.Fatalf("Restore failed: %v", err)
	}

	restoredData, err := os.ReadFile(databasePath)
	if err != nil {
		t.Fatalf("failed to read restored database: %v", err)
	}

	if string(restoredData) != string(backupData) {
		t.Fatalf("restored data does not match backup data")
	}
}
