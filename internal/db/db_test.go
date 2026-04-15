// Copyright 2024 Ella Networks

package db_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ellanetworks/core/internal/db"
	ellaraft "github.com/ellanetworks/core/internal/raft"
)

func TestConnect(t *testing.T) {
	tempDir := t.TempDir()

	db, err := db.NewDatabase(context.Background(), filepath.Join(tempDir, "db.sqlite3"), ellaraft.ClusterConfig{})
	if err != nil {
		t.Fatalf("Can't connect to SQLite: %s", err)
	}

	err = db.Close()
	if err != nil {
		t.Fatalf("Can't close connection: %s", err)
	}
}
