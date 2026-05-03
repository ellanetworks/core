// Copyright 2026 Ella Networks

package db

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

// TestReplicatedTables_NoAUTOINCREMENT enforces a structural invariant:
// the primary key of every replicated table is decided at the request
// handler (UUIDv7) and never derived from rollback-able local state.
//
// Server-side AUTOINCREMENT in a replicated table is unsafe: the
// leader's capture path (BEGIN; INSERT; capture changeset; ROLLBACK)
// rolls back sqlite_sequence with the row, so two captures across the
// leader-takeover window can pick the same id. The follower's
// sqlite3changeset_apply then rejects the second INSERT with CONFLICT
// and the FSM panics. Generating the PK at the handler eliminates the
// race regardless of timing.
//
// Adding a replicated table with INTEGER PRIMARY KEY AUTOINCREMENT
// fails this test — migrate it to TEXT PRIMARY KEY (UUIDv7) first.
func TestReplicatedTables_NoAUTOINCREMENT(t *testing.T) {
	tmp := t.TempDir()

	conn, err := openSQLiteConnection(context.Background(), filepath.Join(tmp, "db.sqlite3"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	defer func() { _ = conn.Close() }()

	if err := runMigrations(context.Background(), conn, 0); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	for _, table := range replicatedChangesetTables {
		if hasAutoIncrement(t, conn, table) {
			t.Errorf("replicated table %q uses INTEGER PRIMARY KEY AUTOINCREMENT; "+
				"migrate it to TEXT PRIMARY KEY with a handler-generated UUID", table)
		}
	}
}

func hasAutoIncrement(t *testing.T, conn *sql.DB, table string) bool {
	t.Helper()

	var sqlText string

	err := conn.QueryRowContext(context.Background(),
		"SELECT sql FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&sqlText)
	if err != nil {
		if err == sql.ErrNoRows {
			return false
		}

		t.Fatalf("read schema for %q: %v", table, err)
	}

	return strings.Contains(strings.ToUpper(sqlText), "AUTOINCREMENT")
}
