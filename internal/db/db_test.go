package db_test

import (
	"path/filepath"
	"testing"

	"github.com/yeastengine/ella/internal/db"
)

func TestConnect(t *testing.T) {
	tempDir := t.TempDir()
	db, err := db.NewDatabase(filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("Can't connect to SQLite: %s", err)
	}
	db.Close()
}
