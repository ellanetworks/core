// Copyright 2024 Ella Networks

package db_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ellanetworks/core/internal/db"
)

func TestLogRetentionPolicyEndToEnd(t *testing.T) {
	tempDir := t.TempDir()

	database, err := db.NewDatabase(filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	}()

	res, err := database.GetLogRetentionPolicy(context.Background(), db.CategoryAuditLogs)
	if err != nil {
		t.Fatalf("couldn't get audit log retention policy: %s", err)
	}

	if res != 7 {
		t.Fatalf("Expected default audit log retention policy to be 7 days, but got %d", res)
	}

	policy := &db.LogRetentionPolicy{
		Category: db.CategoryAuditLogs,
		Days:     60,
	}

	err = database.SetLogRetentionPolicy(context.Background(), policy)
	if err != nil {
		t.Fatalf("couldn't set audit log retention policy: %s", err)
	}

	res, err = database.GetLogRetentionPolicy(context.Background(), db.CategoryAuditLogs)
	if err != nil {
		t.Fatalf("couldn't get audit log retention policy: %s", err)
	}

	if res != 60 {
		t.Fatalf("Expected audit log retention policy to be 60 days, but got %d", res)
	}
}
