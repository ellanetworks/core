// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"path/filepath"
	"testing"
)

func TestMigrationV26AllowsThePrivacyTerminationReason(t *testing.T) {
	ctx := context.Background()

	conn, err := openSQLiteConnection(ctx, filepath.Join(t.TempDir(), "db.sqlite3"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	t.Cleanup(func() { _ = conn.Close() })

	if err := runMigrations(ctx, conn, 25); err != nil {
		t.Fatalf("migrate to v25: %v", err)
	}

	mustExec(t, conn, `INSERT INTO deferred_location_requests (id, supi, ldr_type, method, reporting_interval_s, reporting_amount, status, created_at)
		VALUES ('r1', 'imsi-001010000000001', 'PERIODIC', 'cell_id', 60, 10, 'ACTIVE', 1000)`)
	mustExec(t, conn, `INSERT INTO location_reports (request_id, seq, reported_at, location_data) VALUES ('r1', 1, 1001, '{}')`)

	if _, err := conn.ExecContext(ctx, `UPDATE deferred_location_requests SET status = 'TERMINATED', termination_cause = 'TERMINATION_BY_NETWORK', termination_reason = 'privacy', terminated_at = 2000 WHERE id = 'r1'`); err == nil {
		t.Fatal("v25 accepted the privacy termination reason")
	}

	if err := runMigrations(ctx, conn, 26); err != nil {
		t.Fatalf("migrate to v26: %v", err)
	}

	mustExec(t, conn, `UPDATE deferred_location_requests SET status = 'TERMINATED', termination_cause = 'TERMINATION_BY_NETWORK', termination_reason = 'privacy', terminated_at = 2000 WHERE id = 'r1'`)

	if _, err := conn.ExecContext(ctx, `UPDATE deferred_location_requests SET termination_reason = 'unknown' WHERE id = 'r1'`); err == nil {
		t.Fatal("v26 accepted an unknown termination reason")
	}

	var reports int
	if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM location_reports WHERE request_id = 'r1'").Scan(&reports); err != nil || reports != 1 {
		t.Fatalf("reports kept = %d, %v", reports, err)
	}

	for _, idx := range []string{"idx_deferred_location_requests_supi", "idx_deferred_location_requests_active_supi", "idx_deferred_location_requests_terminated_at"} {
		var n int
		if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = ?", idx).Scan(&n); err != nil || n != 1 {
			t.Fatalf("index %s: %d, %v", idx, n, err)
		}
	}

	mustExec(t, conn, "PRAGMA foreign_keys = ON")
	mustExec(t, conn, "DELETE FROM deferred_location_requests WHERE id = 'r1'")

	if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM location_reports").Scan(&reports); err != nil || reports != 0 {
		t.Fatalf("reports after the request was deleted = %d, %v", reports, err)
	}
}
