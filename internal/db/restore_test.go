// Copyright 2024 Ella Networks

package db_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ellanetworks/core/internal/db"
)

func TestRestore(t *testing.T) {
	tempDir := t.TempDir()

	databasePath := filepath.Join(tempDir, "db.sqlite3")

	database, err := db.NewDatabase(context.Background(), databasePath)
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("failed to close database: %v", err)
		}
	}()

	// Create a real SQLite backup using the Backup method
	backupFile, err := os.CreateTemp("", "backup_*.db")
	if err != nil {
		t.Fatalf("failed to create temporary backup file: %v", err)
	}

	defer func() {
		err := backupFile.Close()
		if err != nil {
			t.Fatalf("failed to close backup file: %v", err)
		}

		err = os.Remove(backupFile.Name()) // Ensure cleanup
		if err != nil {
			t.Fatalf("failed to remove backup file: %v", err)
		}
	}()

	if err := database.Backup(context.Background(), backupFile); err != nil {
		t.Fatalf("failed to create backup: %v", err)
	}

	if _, err := backupFile.Seek(0, 0); err != nil {
		t.Fatalf("failed to reset backup file pointer: %v", err)
	}

	err = database.Restore(context.Background(), backupFile)
	if err != nil {
		t.Fatalf("Restore failed: %v", err)
	}

	// Verify the restored database is functional by running a query
	_, total, err := database.ListSubscribersPage(context.Background(), 1, 10)
	if err != nil {
		t.Fatalf("failed to query restored database: %v", err)
	}

	if total != 0 {
		t.Fatalf("expected 0 subscribers, got %d", total)
	}
}
