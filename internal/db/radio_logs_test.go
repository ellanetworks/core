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

func TestRadioLogsEndToEnd(t *testing.T) {
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

	res, total, err := database.ListRadioLogsPage(context.Background(), 1, 10)
	if err != nil {
		t.Fatalf("couldn't list radio logs: %s", err)
	}

	if total != 0 {
		t.Fatalf("Expected total count to be 0, but got %d", total)
	}

	if len(res) != 0 {
		t.Fatalf("Expected no radio logs, but found %d", len(res))
	}

	rawEntry1 := `{"timestamp":"2024-10-01T12:00:00Z","component":"Radio","event":"test_event","ran_id":"test_ran_id","details":"This is a test radio log entry"}`
	rawEntry2 := `{"timestamp":"2024-10-01T13:00:00Z","component":"Radio","event":"another_event","ran_id":"another_ran_id","details":"This is another test radio log entry"}`

	err = database.InsertRadioLogJSON(context.Background(), []byte(rawEntry1))
	if err != nil {
		t.Fatalf("couldn't insert radio log: %s", err)
	}

	err = database.InsertRadioLogJSON(context.Background(), []byte(rawEntry2))
	if err != nil {
		t.Fatalf("couldn't insert radio log: %s", err)
	}

	res, total, err = database.ListRadioLogsPage(context.Background(), 1, 10)
	if err != nil {
		t.Fatalf("couldn't list radio logs: %s", err)
	}

	if total != 2 {
		t.Fatalf("Expected total count to be 2, but got %d", total)
	}

	if len(res) != 2 {
		t.Fatalf("Expected 2 radio logs, but found %d", len(res))
	}

	if res[0].Event != "another_event" || res[1].Event != "test_event" {
		t.Fatalf("Radio logs are not in the expected order or have incorrect data")
	}

	err = database.DeleteOldRadioLogs(context.Background(), 1)
	if err != nil {
		t.Fatalf("couldn't delete old radio logs: %s", err)
	}

	res, total, err = database.ListRadioLogsPage(context.Background(), 1, 10)
	if err != nil {
		t.Fatalf("couldn't list radio logs after deletion: %s", err)
	}

	if total != 0 {
		t.Fatalf("Expected total count to be 0 after deletion, but got %d", total)
	}

	if len(res) != 0 {
		t.Fatalf("Expected no radio logs after deletion, but found %d", len(res))
	}
}

func TestRadioLogsRetentionPurgeKeepsNewerAndBoundary(t *testing.T) {
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

	insert := func(ts time.Time, event string) {
		raw := fmt.Sprintf(`{
			"timestamp":"%s",
			"level":"info",
			"component":"Radio",
			"event":"%s",
			"ran_id":"001:01:000008",
			"details":"test"
		}`, ts.UTC().Format(time.RFC3339), event)
		if err := database.InsertRadioLogJSON(ctx, []byte(raw)); err != nil {
			t.Fatalf("insert failed (%s): %v", event, err)
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

	logs, total, err := database.ListRadioLogsPage(ctx, 1, 10)
	if err != nil {
		t.Fatalf("list before purge failed: %v", err)
	}

	if total != 3 {
		t.Fatalf("expected total 3 logs before purge, got %d", total)
	}

	if got := len(logs); got != 3 {
		t.Fatalf("expected 3 logs before purge, got %d", got)
	}

	if err := database.DeleteOldRadioLogs(ctx, policyDays); err != nil {
		t.Fatalf("could not delete old radio logs: %v", err)
	}

	// Verify only newer + boundary remain.
	logs, total, err = database.ListRadioLogsPage(ctx, 1, 10)
	if err != nil {
		t.Fatalf("list after purge failed: %v", err)
	}

	if total != 2 {
		t.Fatalf("expected total 2 logs after purge, got %d", total)
	}

	if got := len(logs); got != 2 {
		t.Fatalf("expected 2 logs after purge, got %d", got)
	}

	remaining := map[string]bool{}
	for _, l := range logs {
		remaining[l.Event] = true
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
