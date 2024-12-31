package db_test

import (
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

	network := &db.Network{
		Mcc: "123",
		Mnc: "456",
	}
	err = database.UpdateNetwork(network)
	if err != nil {
		t.Fatalf("Couldn't update network: %s", err)
	}

	backupFilePath, err := database.Backup()
	if err != nil {
		t.Fatalf("Couldn't create backup: %s", err)
	}

	if _, err := os.Stat(backupFilePath); os.IsNotExist(err) {
		t.Fatalf("Backup file does not exist: %s", backupFilePath)
	}

	originalFileInfo, err := os.Stat(dbPath)
	if err != nil {
		t.Fatalf("Couldn't stat original database file: %s", err)
	}
	backupFileInfo, err := os.Stat(backupFilePath)
	if err != nil {
		t.Fatalf("Couldn't stat backup file: %s", err)
	}
	if originalFileInfo.Size() != backupFileInfo.Size() {
		t.Fatalf("Backup file size mismatch: expected %d, got %d", originalFileInfo.Size(), backupFileInfo.Size())
	}

	if err := os.Remove(backupFilePath); err != nil {
		t.Fatalf("Couldn't delete backup file: %s", err)
	}

}
