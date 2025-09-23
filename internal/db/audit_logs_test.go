// Copyright 2024 Ella Networks

package db_test

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

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

	ctx := context.Background()

	res, total, err := database.ListAuditLogsPage(ctx, 1, 10)
	if err != nil {
		t.Fatalf("couldn't list audit logs: %s", err)
	}

	if total != 0 {
		t.Fatalf("Expected total count to be 0, but got %d", total)
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

	res, total, err = database.ListAuditLogsPage(ctx, 1, 10)
	if err != nil {
		t.Fatalf("couldn't list audit logs: %s", err)
	}

	if total != 2 {
		t.Fatalf("Expected total count to be 2, but got %d", total)
	}

	if len(res) != 2 {
		t.Fatalf("Expected 2 audit logs, but found %d", len(res))
	}

	if res[0].Action != "another_action" || res[1].Action != "test_action" {
		t.Fatalf("Audit logs are not in the expected order or have incorrect data")
	}

	err = database.DeleteOldAuditLogs(context.Background(), 1)
	if err != nil {
		t.Fatalf("couldn't delete old audit logs: %s", err)
	}

	res, total, err = database.ListAuditLogsPage(ctx, 1, 10)
	if err != nil {
		t.Fatalf("couldn't list audit logs after deletion: %s", err)
	}

	if total != 0 {
		t.Fatalf("Expected total count to be 0 after deletion, but got %d", total)
	}

	if len(res) != 0 {
		t.Fatalf("Expected no audit logs after deletion, but found %d", len(res))
	}
}

func TestAuditLogsRetentionPurgeKeepsNewerAndBoundary(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	database, err := db.NewDatabase(filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %v", err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %v", err)
		}
	}()

	ctx := context.Background()

	insert := func(ts time.Time, action string) {
		raw := fmt.Sprintf(`{
			"timestamp":"%s",
			"level":"info",
			"component":"Audit",
			"action":"%s",
			"actor":"tester",
			"details":"test",
			"ip":"127.0.0.1"
		}`, ts.UTC().Format(time.RFC3339), action)
		if err := database.InsertAuditLogJSON(ctx, []byte(raw)); err != nil {
			t.Fatalf("insert failed (%s): %v", action, err)
		}
	}

	now := time.Now().UTC().Truncate(time.Second)

	const policyDays = 7
	cutoff := now.AddDate(0, 0, -policyDays)

	veryOld := cutoff.Add(-48 * time.Hour)
	boundary := cutoff
	fresh := now.Add(-24 * time.Hour)

	insert(veryOld, "very_old")
	insert(boundary, "boundary_exact")
	insert(fresh, "fresh")

	logs, total, err := database.ListAuditLogsPage(ctx, 1, 10)
	if err != nil {
		t.Fatalf("list before purge failed: %v", err)
	}

	if total != 3 {
		t.Fatalf("Expected total count to be 3, but got %d", total)
	}

	if got := len(logs); got != 3 {
		t.Fatalf("expected 3 logs before purge, got %d", got)
	}

	if err := database.DeleteOldAuditLogs(ctx, policyDays); err != nil {
		t.Fatalf("could not delete old audit logs: %v", err)
	}

	// Verify only newer + boundary remain.
	logs, total, err = database.ListAuditLogsPage(ctx, 1, 10)
	if err != nil {
		t.Fatalf("list after purge failed: %v", err)
	}

	if total != 2 {
		t.Fatalf("Expected total count to be 2 after purge, but got %d", total)
	}

	if got := len(logs); got != 2 {
		t.Fatalf("expected 2 logs after purge, got %d", got)
	}

	remaining := map[string]bool{}
	for _, l := range logs {
		remaining[l.Action] = true
	}

	if remaining["very_old"] {
		t.Fatalf("unexpected: very_old log should have been deleted")
	}

	if !remaining["boundary_exact"] {
		t.Fatalf("expected boundary_exact log to remain (cutoff is inclusive)")
	}

	if !remaining["fresh"] {
		t.Fatalf("expected fresh log to remain")
	}
}
