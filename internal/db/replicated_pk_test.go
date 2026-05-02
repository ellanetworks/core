// Copyright 2026 Ella Networks

package db

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

// autoIncrementExempt lists replicated tables that still use INTEGER
// PRIMARY KEY AUTOINCREMENT pending migration to UUID PKs. The list
// must only shrink — adding to it requires a code review nudge that
// the spec_uuid.md plan is being deferred for that table.
//
// Empty target: every replicated table generates its PK at the request
// handler and stores it as TEXT.
// All replicated tables have been migrated to TEXT UUID PKs (spec_uuid.md
// + migration v11). The exempt list is empty: every replicated table now
// generates its PK at the request handler. Adding a table here in the
// future requires a code review nudge that the structural fix is being
// deferred for that table.
var autoIncrementExempt = map[string]struct{}{}

// TestReplicatedTables_NoUnexpectedAUTOINCREMENT enforces the spec_uuid.md
// invariant: PKs of replicated tables are decided at the request handler,
// never derived from rollback-able local state. AUTOINCREMENT on a table
// that goes through changeset capture lets two captures pick the same id
// when the new leader's first capture races with a previous-term entry's
// pending FSM apply (see int_fail8.txt for the observed crash).
//
// Failures here mean either: a new replicated table was added with
// AUTOINCREMENT (don't), or an exempted table was migrated and the
// exemption stayed in the list (delete it).
func TestReplicatedTables_NoUnexpectedAUTOINCREMENT(t *testing.T) {
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
			if _, ok := autoIncrementExempt[table]; ok {
				continue
			}

			t.Errorf("replicated table %q uses AUTOINCREMENT but is not in autoIncrementExempt; either migrate it to a TEXT UUID PK (spec_uuid.md) or add it to the exempt list", table)
		} else if _, ok := autoIncrementExempt[table]; ok {
			t.Errorf("table %q is in autoIncrementExempt but no longer uses AUTOINCREMENT; delete it from the list", table)
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
