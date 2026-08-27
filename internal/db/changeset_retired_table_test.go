// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"testing"

	"github.com/mattn/go-sqlite3"
)

// captureChangesetOverTables records a changeset over an explicit table list,
// standing in for a leader whose binary still held those tables in its
// replicated set. The mutations are rolled back, matching captureChangeset.
func captureChangesetOverTables(t *testing.T, database *Database, tables []string, stmts []string) []byte {
	t.Helper()

	ctx := context.Background()

	conn, err := database.PlainDB().Conn(ctx)
	if err != nil {
		t.Fatalf("acquire conn: %v", err)
	}

	defer func() { _ = conn.Close() }()

	var changeset []byte

	err = conn.Raw(func(raw any) error {
		sqliteConn, ok := raw.(*sqlite3.SQLiteConn)
		if !ok {
			return fmt.Errorf("unexpected sqlite driver conn type %T", raw)
		}

		if _, err := sqliteConn.ExecContext(ctx, "BEGIN IMMEDIATE", nil); err != nil {
			return fmt.Errorf("begin capture: %w", err)
		}

		defer func() {
			_, _ = sqliteConn.ExecContext(context.Background(), "ROLLBACK", nil)
		}()

		changeset, err = sqliteConn.CaptureChangeset(ctx, func() error {
			for _, stmt := range stmts {
				if _, err := sqliteConn.ExecContext(ctx, stmt, []driver.NamedValue{}); err != nil {
					return fmt.Errorf("exec %q: %w", stmt, err)
				}
			}

			return nil
		}, tables)

		return err
	})
	if err != nil {
		t.Fatalf("capture changeset over %v: %v", tables, err)
	}

	if len(changeset) == 0 {
		t.Fatalf("capture over %v returned zero bytes", tables)
	}

	return changeset
}

func countAuditLogs(t *testing.T, database *Database, id string) int {
	t.Helper()

	var n int

	if err := database.PlainDB().QueryRowContext(context.Background(),
		"SELECT COUNT(*) FROM audit_logs WHERE id = ?", id).Scan(&n); err != nil {
		t.Fatalf("count audit_logs: %v", err)
	}

	return n
}

func auditLogActor(t *testing.T, database *Database, id string) string {
	t.Helper()

	var actor string

	if err := database.PlainDB().QueryRowContext(context.Background(),
		"SELECT actor FROM audit_logs WHERE id = ?", id).Scan(&actor); err != nil {
		t.Fatalf("read audit_logs actor: %v", err)
	}

	return actor
}

// TestApplyChangeset_SkipsRetiredReplicatedTable replays an entry captured
// while audit_logs was replicated against a node that already owns the same
// row, which is what a snapshot restore leaves behind now that
// restoreLocalOnlyTables carries audit_logs across the file swap. The retired
// table must be filtered out, and the replicated table in the same entry must
// still apply.
func TestApplyChangeset_SkipsRetiredReplicatedTable(t *testing.T) {
	database := newAtomicTestDB(t)
	ctx := context.Background()

	const auditID = "01900000-0000-7000-8000-0000000000a1"

	changeset := captureChangesetOverTables(t, database,
		[]string{AuditLogsTableName, NetworkSlicesTableName},
		[]string{
			fmt.Sprintf(`INSERT INTO audit_logs (id, timestamp, level, actor, action, ip, details)
			 VALUES ('%s', '2026-08-27T09:00:00Z', 'INFO', 'other-node', 'login', '10.0.0.2', 'details')`, auditID),
			`INSERT INTO network_slices (id, sst, sd, name)
			 VALUES ('01900000-0000-7000-8000-00000000aaaa', 1, '000001', 'changeset-regression')`,
		})

	// The row this node owns after restoreLocalOnlyTables carried its
	// audit_logs across the snapshot restore.
	if _, err := database.PlainDB().ExecContext(ctx,
		`INSERT INTO audit_logs (id, timestamp, level, actor, action, ip, details)
		 VALUES (?, '2026-08-27T10:00:00Z', 'INFO', 'local-node', 'login', '10.0.0.1', 'details')`,
		auditID); err != nil {
		t.Fatalf("seed local audit_logs row: %v", err)
	}

	setLastApplied(t, database, 41)

	if _, err := database.applyChangeset(ctx, &bytesPayload{Value: changeset}, 42); err != nil {
		t.Fatalf("apply changeset carrying a retired table: %v", err)
	}

	if got := countAuditLogs(t, database, auditID); got != 1 {
		t.Fatalf("audit_logs rows for %s: got %d, want 1", auditID, got)
	}

	if got := auditLogActor(t, database, auditID); got != "local-node" {
		t.Fatalf("audit_logs actor for %s: got %q, want %q", auditID, got, "local-node")
	}

	if got := countSlices(t, database); got != 1 {
		t.Fatalf("network_slices rows named changeset-regression: got %d, want 1", got)
	}

	if got := readLastApplied(t, database); got != 42 {
		t.Fatalf("fsm_state.lastApplied: got %d, want 42", got)
	}
}

// TestApplyChangeset_UnfilteredRetiredTableWouldConflict pins the reason the
// filter exists: the same entry aborts the whole apply when every table is
// accepted, which is what halts the FSM.
func TestApplyChangeset_UnfilteredRetiredTableWouldConflict(t *testing.T) {
	database := newAtomicTestDB(t)
	ctx := context.Background()

	const auditID = "01900000-0000-7000-8000-0000000000a2"

	changeset := captureChangesetOverTables(t, database,
		[]string{AuditLogsTableName},
		[]string{
			fmt.Sprintf(`INSERT INTO audit_logs (id, timestamp, level, actor, action, ip, details)
			 VALUES ('%s', '2026-08-27T09:00:00Z', 'INFO', 'other-node', 'login', '10.0.0.2', 'details')`, auditID),
		})

	if _, err := database.PlainDB().ExecContext(ctx,
		`INSERT INTO audit_logs (id, timestamp, level, actor, action, ip, details)
		 VALUES (?, '2026-08-27T10:00:00Z', 'INFO', 'local-node', 'login', '10.0.0.1', 'details')`,
		auditID); err != nil {
		t.Fatalf("seed local audit_logs row: %v", err)
	}

	conn, err := database.PlainDB().Conn(ctx)
	if err != nil {
		t.Fatalf("acquire conn: %v", err)
	}

	defer func() { _ = conn.Close() }()

	err = conn.Raw(func(raw any) error {
		sqliteConn, ok := raw.(*sqlite3.SQLiteConn)
		if !ok {
			return fmt.Errorf("unexpected sqlite driver conn type %T", raw)
		}

		return sqliteConn.ApplyChangeset(ctx, changeset)
	})
	if err == nil {
		t.Fatal("unfiltered apply of a retired-table changeset succeeded, expected a conflict")
	}
}

// TestApplyForwardedOperation_RejectsRetiredOps covers a follower on an older
// binary forwarding an operation a leader cannot honor. Capturing
// InsertAuditLog on the leader yields an empty changeset, since audit_logs is
// unattached, and the propose path reads an empty changeset as a successful
// no-op — so without an explicit rejection the follower is told its audit
// record was written.
func TestApplyForwardedOperation_RejectsRetiredOps(t *testing.T) {
	database := newAtomicTestDB(t)

	for _, opName := range []string{"InsertAuditLog", "DeleteOldAuditLogs"} {
		t.Run(opName, func(t *testing.T) {
			_, err := database.ApplyForwardedOperation(opName, []byte(`{}`))
			if err == nil {
				t.Fatalf("%s: forwarded retired operation returned success", opName)
			}

			if !errors.Is(err, ErrRetiredOperation) {
				t.Fatalf("%s: got %v, want ErrRetiredOperation", opName, err)
			}
		})
	}
}

// TestApplyForwardedOperation_AcceptsLiveOps pins that the rejection is scoped
// to retired names.
func TestApplyForwardedOperation_AcceptsLiveOps(t *testing.T) {
	database := newAtomicTestDB(t)

	_, err := database.ApplyForwardedOperation("CreateNetworkSlice", []byte(
		`{"id":"01900000-0000-7000-8000-00000000abcd","sst":1,"sd":"000001","name":"forwarded-live"}`))
	if err != nil && errors.Is(err, ErrRetiredOperation) {
		t.Fatalf("live operation rejected as retired: %v", err)
	}

	if err != nil && errors.Is(err, ErrUnknownOperation) {
		t.Fatalf("live operation reported as unknown: %v", err)
	}
}
