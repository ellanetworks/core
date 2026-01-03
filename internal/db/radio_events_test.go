// Copyright 2024 Ella Networks

package db_test

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/dbwriter"
)

func TestRadioEventsEndToEnd(t *testing.T) {
	tempDir := t.TempDir()

	database, err := db.NewDatabase(context.Background(), filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	}()

	res, total, err := database.ListRadioEvents(context.Background(), 1, 10, nil)
	if err != nil {
		t.Fatalf("couldn't list setwork logs: %s", err)
	}

	if total != 0 {
		t.Fatalf("Expected total count to be 0, but got %d", total)
	}

	if len(res) != 0 {
		t.Fatalf("Expected no setwork logs, but found %d", len(res))
	}

	err = database.InsertRadioEvent(context.Background(), &dbwriter.RadioEvent{
		Timestamp:     "2024-10-01T12:00:00Z",
		Protocol:      "ngap",
		MessageType:   "test_event",
		Direction:     "inbound",
		LocalAddress:  "127.0.0.1",
		RemoteAddress: "192.168.1.1",
		Raw:           []byte("SGVsbG8gd29ybGQh"),
	})
	if err != nil {
		t.Fatalf("couldn't insert setwork log: %s", err)
	}

	err = database.InsertRadioEvent(context.Background(), &dbwriter.RadioEvent{
		Timestamp:     "2024-10-01T13:00:00Z",
		Protocol:      "another_protocol",
		MessageType:   "another_event",
		Direction:     "outbound",
		LocalAddress:  "127.0.0.1",
		RemoteAddress: "192.168.1.1",
		Raw:           []byte("QW5vdGhlciBsb2cgZW50cnk="),
	})
	if err != nil {
		t.Fatalf("couldn't insert setwork log: %s", err)
	}

	res, total, err = database.ListRadioEvents(context.Background(), 1, 10, nil)
	if err != nil {
		t.Fatalf("couldn't list setwork logs: %s", err)
	}

	if total != 2 {
		t.Fatalf("Expected total count to be 2, but got %d", total)
	}

	if len(res) != 2 {
		t.Fatalf("Expected 2 setwork logs, but found %d", len(res))
	}

	if res[0].MessageType != "another_event" || res[1].MessageType != "test_event" {
		t.Fatalf("Radio events are not in the expected order or have incorrect data")
	}

	err = database.DeleteOldRadioEvents(context.Background(), 1)
	if err != nil {
		t.Fatalf("couldn't delete old setwork logs: %s", err)
	}

	res, total, err = database.ListRadioEvents(context.Background(), 1, 10, nil)
	if err != nil {
		t.Fatalf("couldn't list setwork logs after deletion: %s", err)
	}

	if total != 0 {
		t.Fatalf("Expected total count to be 0 after deletion, but got %d", total)
	}

	if len(res) != 0 {
		t.Fatalf("Expected no setwork logs after deletion, but found %d", len(res))
	}
}

func TestGetRadioEventByID(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()

	database, err := db.NewDatabase(context.Background(), filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %v", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %v", err)
		}
	}()

	ctx := context.Background()

	if err := database.InsertRadioEvent(ctx, &dbwriter.RadioEvent{
		Timestamp:     "2024-10-01T12:00:00Z",
		Protocol:      "ngap",
		MessageType:   "test_event",
		Direction:     "inbound",
		LocalAddress:  "127.0.0.1",
		RemoteAddress: "192.168.1.1",
		Raw:           []byte("SGVsbG8gd29ybGQh"),
	}); err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	logs, total, err := database.ListRadioEvents(ctx, 1, 10, nil)
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}

	if total != 1 {
		t.Fatalf("expected total 1 log, got %d", total)
	}

	if got := len(logs); got != 1 {
		t.Fatalf("expected 1 log, got %d", got)
	}

	logID := logs[0].ID

	log, err := database.GetRadioEventByID(ctx, logID)
	if err != nil {
		t.Fatalf("GetRadioEventByID failed: %v", err)
	}

	if !reflect.DeepEqual(logs[0], *log) {
		t.Fatalf("fetched log does not match listed log")
	}
}

func TestRadioEventsRetentionPurgeKeepsNewerAndBoundary(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()

	database, err := db.NewDatabase(context.Background(), filepath.Join(tempDir, "db.sqlite3"))
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
		if err := database.InsertRadioEvent(ctx, &dbwriter.RadioEvent{
			Timestamp:   ts.UTC().Format(time.RFC3339),
			MessageType: event,
			Direction:   "inbound",
			Protocol:    "tester",
			Raw:         []byte("dGVzdA=="),
		}); err != nil {
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

	logs, total, err := database.ListRadioEvents(ctx, 1, 10, nil)
	if err != nil {
		t.Fatalf("list before purge failed: %v", err)
	}

	if total != 3 {
		t.Fatalf("expected total 3 logs before purge, got %d", total)
	}

	if got := len(logs); got != 3 {
		t.Fatalf("expected 3 logs before purge, got %d", got)
	}

	if err := database.DeleteOldRadioEvents(ctx, policyDays); err != nil {
		t.Fatalf("could not delete old setwork logs: %v", err)
	}

	// Verify only newer + boundary remain.
	logs, total, err = database.ListRadioEvents(ctx, 1, 10, nil)
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
		remaining[l.MessageType] = true
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

func TestListRadioEventsProtocolFilter(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()

	database, err := db.NewDatabase(context.Background(), filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %v", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %v", err)
		}
	}()

	ctx := context.Background()

	insert := func(protocol, event string) {
		if err := database.InsertRadioEvent(ctx, &dbwriter.RadioEvent{
			Protocol:    protocol,
			MessageType: event,
			Direction:   "inbound",
			Timestamp:   time.Now().UTC().Format(time.RFC3339),
			Raw:         []byte("dGVzdA=="),
		}); err != nil {
			t.Fatalf("insert failed (%s): %v", event, err)
		}
	}

	insert("protocol-001", "event-001")
	insert("protocol-002", "event-002")
	insert("protocol-001", "event-003")

	logs, total, err := database.ListRadioEvents(ctx, 1, 10, &db.RadioEventFilters{Protocol: ptr("protocol-001")})
	if err != nil {
		t.Fatalf("list with protocol filter failed: %v", err)
	}

	if total != 2 {
		t.Fatalf("expected total 2 logs with Protocol filter, got %d", total)
	}

	if got := len(logs); got != 2 {
		t.Fatalf("expected 2 logs with Protocol filter, got %d", got)
	}

	for _, l := range logs {
		if l.Protocol != "protocol-001" {
			t.Fatalf("expected protocol-001 with Protocol filter, got %q", l.Protocol)
		}
	}
}

func TestListRadioEventsTimestampFilter(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()

	database, err := db.NewDatabase(context.Background(), filepath.Join(tempDir, "db.sqlite3"))
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
		if err := database.InsertRadioEvent(ctx, &dbwriter.RadioEvent{
			Timestamp:   ts.UTC().Format(time.RFC3339),
			MessageType: event,
			Protocol:    "tester",
			Direction:   "inbound",
			Raw:         []byte("dGVzdA=="),
		}); err != nil {
			t.Fatalf("insert failed (%s): %v", event, err)
		}
	}

	now := time.Now().UTC().Truncate(time.Second)
	past1 := now.Add(-48 * time.Hour)
	past2 := now.Add(-24 * time.Hour)
	veryNearFuture := now.Add(5 * time.Minute)
	future := now.Add(24 * time.Hour)

	insert(past1, "event-001")
	insert(past2, "event-002")
	insert(now, "event-003")
	insert(future, "event-004")

	from := past2.Format(time.RFC3339)
	to := veryNearFuture.Format(time.RFC3339)

	logs, total, err := database.ListRadioEvents(ctx, 1, 10, &db.RadioEventFilters{
		TimestampFrom: &from,
		TimestampTo:   &to,
	})
	if err != nil {
		t.Fatalf("list with timestamp filter failed: %v", err)
	}

	if total != 2 {
		t.Fatalf("expected total 2 logs with timestamp filter, got %d", total)
	}

	if got := len(logs); got != 2 {
		t.Fatalf("expected 2 logs with timestamp filter, got %d", got)
	}

	expectedEvents := map[string]bool{
		"event-002": true,
		"event-003": true,
	}

	for _, l := range logs {
		if !expectedEvents[l.MessageType] {
			t.Fatalf("unexpected event %q with timestamp filter", l.MessageType)
		}
	}
}

func TestListRadioEventsTimestampAndProtocolFilters(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()

	database, err := db.NewDatabase(context.Background(), filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %v", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %v", err)
		}
	}()

	ctx := context.Background()

	insert := func(ts time.Time, protocol, event string) {
		if err := database.InsertRadioEvent(ctx, &dbwriter.RadioEvent{
			Timestamp:   ts.UTC().Format(time.RFC3339),
			MessageType: event,
			Protocol:    protocol,
			Direction:   "inbound",
			Raw:         []byte("dGVzdA=="),
		}); err != nil {
			t.Fatalf("insert failed (%s): %v", event, err)
		}
	}

	now := time.Now().UTC().Truncate(time.Second)
	past1 := now.Add(-48 * time.Hour)
	past2 := now.Add(-24 * time.Hour)
	future := now.Add(24 * time.Hour)

	insert(past1, "protocol-001", "event-001")
	insert(past2, "protocol-002", "event-002")
	insert(now, "protocol-001", "event-003")
	insert(future, "protocol-002", "event-004")

	from := past2.Format(time.RFC3339)
	to := future.Format(time.RFC3339)
	protocol := "protocol-001"

	logs, total, err := database.ListRadioEvents(ctx, 1, 10, &db.RadioEventFilters{
		TimestampFrom: &from,
		TimestampTo:   &to,
		Protocol:      &protocol,
	})
	if err != nil {
		t.Fatalf("list with timestamp+Protocol filter failed: %v", err)
	}

	if total != 1 {
		t.Fatalf("expected total 1 log with timestamp+Protocol filter, got %d", total)
	}

	if got := len(logs); got != 1 {
		t.Fatalf("expected 1 log with timestamp+Protocol filter, got %d", got)
	}

	if logs[0].MessageType != "event-003" {
		t.Fatalf("expected event-003 with timestamp+Protocol filter, got %q", logs[0].MessageType)
	}
}

func ptr[T any](v T) *T {
	return &v
}
