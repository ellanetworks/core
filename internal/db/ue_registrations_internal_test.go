// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func TestResetUERegistrationsPurgedInRestoredDB(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "restored.db")

	conn, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatalf("open: %s", err)
	}

	defer func() { _ = conn.Close() }()

	stmts := []string{
		"CREATE TABLE ue_registrations (imsi TEXT NOT NULL, type TEXT NOT NULL, nodeID TEXT NOT NULL, purged INTEGER NOT NULL DEFAULT 0, registrationTime INTEGER NOT NULL, version INTEGER NOT NULL, PRIMARY KEY (imsi, type))",
		"INSERT INTO ue_registrations VALUES ('001010000000001', 'mme', '1', 1, 1, 1)",
		"INSERT INTO ue_registrations VALUES ('001010000000001', 'amf-3gpp-access', '2', 0, 1, 2)",
		"INSERT INTO ue_registrations VALUES ('001010000000002', 'amf-3gpp-access', '1', 1, 1, 3)",
	}

	for _, stmt := range stmts {
		if _, err := conn.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("exec %q: %s", stmt, err)
		}
	}

	if err := resetUERegistrationsPurgedInRestoredDB(ctx, path); err != nil {
		t.Fatalf("resetUERegistrationsPurgedInRestoredDB: %s", err)
	}

	var purged int
	if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM ue_registrations WHERE purged!=0").Scan(&purged); err != nil {
		t.Fatalf("count purged: %s", err)
	}

	if purged != 0 {
		t.Fatalf("expected no purged registrations, got %d", purged)
	}
}

func TestResetUERegistrationsPurgedInRestoredDB_PreV21Backup(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "restored.db")

	conn, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatalf("open: %s", err)
	}

	defer func() { _ = conn.Close() }()

	if _, err := conn.ExecContext(ctx, "CREATE TABLE subscribers (imsi TEXT PRIMARY KEY)"); err != nil {
		t.Fatalf("create: %s", err)
	}

	if err := resetUERegistrationsPurgedInRestoredDB(ctx, path); err != nil {
		t.Fatalf("resetUERegistrationsPurgedInRestoredDB: %s", err)
	}
}
