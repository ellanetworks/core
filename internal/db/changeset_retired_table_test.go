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

func TestApplyForwardedOperation_AcceptsLiveOps(t *testing.T) {
	database := newAtomicTestDB(t)

	const sliceID = "01900000-0000-7000-8000-00000000abcd"

	result, err := database.ApplyForwardedOperation("CreateNetworkSlice", []byte(
		`{"id":"`+sliceID+`","sst":1,"sd":"000001","name":"forwarded-live"}`))
	if err != nil {
		t.Fatalf("live operation rejected: %v", err)
	}

	// Tolerating any non-retired, non-unknown error would let this pass while
	// the write silently failed, so assert the operation actually committed
	// and took effect.
	if result.Index == 0 {
		t.Error("forwarded changeset op returned index 0; a follower would report an uncommitted write as durable")
	}

	slice, err := database.GetNetworkSlice(t.Context(), "forwarded-live")
	if err != nil {
		t.Fatalf("network slice absent after forwarded op: %v", err)
	}

	if slice.ID != sliceID {
		t.Errorf("network slice ID = %q, want %q", slice.ID, sliceID)
	}
}
