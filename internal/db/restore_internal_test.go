// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
)

func TestRestoreRefusedWhileAnotherRestoreHoldsTheLock(t *testing.T) {
	database := &Database{}

	database.restoreMu.Lock()
	defer database.restoreMu.Unlock()

	if err := database.Restore(t.Context(), nil); !errors.Is(err, ErrRestoreInProgress) {
		t.Fatalf("Restore err = %v, want ErrRestoreInProgress", err)
	}

	if err := database.SelfRestore(t.Context()); !errors.Is(err, ErrRestoreInProgress) {
		t.Fatalf("SelfRestore err = %v, want ErrRestoreInProgress", err)
	}
}

func openMigratedFileDB(t *testing.T, path string, version int) *sql.DB {
	t.Helper()

	conn, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}

	t.Cleanup(func() { _ = conn.Close() })

	conn.SetMaxOpenConns(1)

	if _, err := conn.ExecContext(t.Context(), "PRAGMA journal_mode = WAL;"); err != nil {
		t.Fatalf("enable WAL: %v", err)
	}

	runMigrationsUpToTest(t, conn, version)

	return conn
}

func TestRestoreLocalTablesFromOlderSchemaReplacesSnapshotRows(t *testing.T) {
	ctx := t.Context()
	dir := t.TempDir()
	livePath := filepath.Join(dir, "live.db")
	stagedPath := filepath.Join(dir, "staged.db")

	live := openMigratedFileDB(t, livePath, baselineVersion)
	staged := openMigratedFileDB(t, stagedPath, len(migrations))

	insertAuditLog := func(conn *sql.DB, id string) {
		if _, err := conn.ExecContext(ctx,
			"INSERT INTO audit_logs (id, timestamp, level, action) VALUES (?, 1000, 'info', 'test')", id); err != nil {
			t.Fatalf("insert audit log %s: %v", id, err)
		}
	}

	insertAuditLog(live, "joiner")
	insertAuditLog(staged, "leader")

	if _, err := staged.ExecContext(ctx,
		"INSERT INTO deferred_location_requests (id, supi, ldr_type, method, status, created_at) VALUES ('leader', 'imsi-001010000000001', 'UE_AVAILABLE', 'cell_id', 'ACTIVE', 1000)"); err != nil {
		t.Fatalf("insert deferred location request: %v", err)
	}

	if err := staged.Close(); err != nil {
		t.Fatalf("close staged: %v", err)
	}

	if err := restoreLocalOnlyTables(ctx, livePath, stagedPath); err != nil {
		t.Fatalf("restoreLocalOnlyTables: %v", err)
	}

	merged := openMigratedFileDB(t, stagedPath, len(migrations))

	var ids []string

	rows, err := merged.QueryContext(ctx, "SELECT id FROM audit_logs ORDER BY id")
	if err != nil {
		t.Fatalf("list audit logs: %v", err)
	}

	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan audit log: %v", err)
		}

		ids = append(ids, id)
	}

	_ = rows.Close()

	if len(ids) != 1 || ids[0] != "joiner" {
		t.Fatalf("audit logs after merge = %v, want only this node's [joiner]", ids)
	}

	var deferred int
	if err := merged.QueryRowContext(ctx, "SELECT COUNT(*) FROM deferred_location_requests").Scan(&deferred); err != nil {
		t.Fatalf("count deferred location requests: %v", err)
	}

	if deferred != 0 {
		t.Fatalf("deferred location requests after merge = %d, want 0: the leader's local rows must not survive", deferred)
	}
}
