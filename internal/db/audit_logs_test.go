// Copyright 2024 Ella Networks

package db_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ellanetworks/core/internal/db"
)

func TestAuditLogsEndToEnd(t *testing.T) {
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

	res, err := database.ListAuditLogs(context.Background())
	if err != nil {
		t.Fatalf("couldn't list audit logs: %s", err)
	}

	if len(res) != 0 {
		t.Fatalf("Expected no audit logs, but found %d", len(res))
	}

	rawEntry1 := `{"timestamp":"2024-10-01T12:00:00Z","component":"Audit","action":"test_action","actor":"test_actor","details":"This is a test audit log entry","ip":"1.2.3.4"}`
	rawEntry2 := `{"timestamp":"2024-10-01T13:00:00Z","component":"Audit","action":"another_action","actor":"another_actor","details":"This is another test audit log entry","ip":"2.3.4.5"}`

	err = database.InsertAuditLogJSON(context.Background(), []byte(rawEntry1))
	if err != nil {
		t.Fatalf("couldn't insert audit log: %s", err)
	}

	err = database.InsertAuditLogJSON(context.Background(), []byte(rawEntry2))
	if err != nil {
		t.Fatalf("couldn't insert audit log: %s", err)
	}

	res, err = database.ListAuditLogs(context.Background())
	if err != nil {
		t.Fatalf("couldn't list audit logs: %s", err)
	}

	if len(res) != 2 {
		t.Fatalf("Expected 2 audit logs, but found %d", len(res))
	}

	if res[0].Action != "another_action" || res[1].Action != "test_action" {
		t.Fatalf("Audit logs are not in the expected order or have incorrect data")
	}
}
