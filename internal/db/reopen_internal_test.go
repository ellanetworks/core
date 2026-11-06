// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func readProbe(t *testing.T, ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
},
) string {
	t.Helper()

	var v string
	if err := q.QueryRowContext(ctx, "SELECT v FROM probe").Scan(&v); err != nil {
		t.Fatalf("read probe: %v", err)
	}

	return v
}

func TestReopenKeepsHandlesHeldAcrossRestoreUsable(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), DBFilename)

	database, err := NewDatabaseWithoutRaft(ctx, dbPath)
	if err != nil {
		t.Fatalf("NewDatabaseWithoutRaft: %v", err)
	}

	t.Cleanup(func() { _ = database.Close() })

	handle := database.PlainDB()

	if _, err := handle.ExecContext(ctx, "CREATE TABLE probe (v TEXT NOT NULL)"); err != nil {
		t.Fatalf("create probe: %v", err)
	}

	if _, err := handle.ExecContext(ctx, "INSERT INTO probe(v) VALUES ('before')"); err != nil {
		t.Fatalf("insert probe: %v", err)
	}

	staged := dbPath + ".staged"

	if _, err := handle.ExecContext(ctx, "VACUUM INTO ?", staged); err != nil {
		t.Fatalf("stage snapshot: %v", err)
	}

	stagedDB, err := openSQLiteConnection(ctx, staged)
	if err != nil {
		t.Fatalf("open staged: %v", err)
	}

	if _, err := stagedDB.ExecContext(ctx, "UPDATE probe SET v = 'after'"); err != nil {
		t.Fatalf("update staged: %v", err)
	}

	if err := stagedDB.Close(); err != nil {
		t.Fatalf("close staged: %v", err)
	}

	inFlight, err := handle.Conn(ctx)
	if err != nil {
		t.Fatalf("acquire in-flight conn: %v", err)
	}

	if got := readProbe(t, ctx, inFlight); got != "before" {
		t.Fatalf("in-flight read = %q, want %q", got, "before")
	}

	for _, suffix := range []string{"-wal", "-shm"} {
		_ = os.Remove(dbPath + suffix)
	}

	if err := os.Rename(staged, dbPath); err != nil {
		t.Fatalf("swap in snapshot: %v", err)
	}

	reopened := make(chan error, 1)

	go func() { reopened <- database.Reopen(ctx) }()

	if got := readProbe(t, ctx, inFlight); got != "before" {
		t.Fatalf("in-flight read during reopen = %q, want %q", got, "before")
	}

	if err := inFlight.Close(); err != nil {
		t.Fatalf("release in-flight conn: %v", err)
	}

	select {
	case err := <-reopened:
		if err != nil {
			t.Fatalf("Reopen: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Reopen did not return after the in-flight conn was released")
	}

	if got := readProbe(t, ctx, handle); got != "after" {
		t.Fatalf("read through handle taken before Reopen = %q, want %q", got, "after")
	}
}
