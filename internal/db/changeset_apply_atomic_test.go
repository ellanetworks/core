// Copyright 2026 Ella Networks

package db

import (
	"context"
	"path/filepath"
	"testing"

	ellaraft "github.com/ellanetworks/core/internal/raft"
)

// captureAuditLogChangeset returns the changeset bytes for a single
// InsertAuditLog op without committing it locally. captureChangeset
// rolls back its inner transaction, so the audit_logs table remains
// untouched after this returns.
func captureAuditLogChangeset(t *testing.T, database *Database) []byte {
	t.Helper()

	payload := &auditLogPayload{
		Timestamp: "2026-05-02T13:00:00Z",
		Level:     "info",
		Actor:     "test",
		Action:    "test_action",
		IP:        "1.2.3.4",
		Details:   "regression test",
	}

	bytes, _, err := database.captureChangeset(context.Background(),
		func(ctx context.Context) (any, error) {
			return database.applyInsertAuditLog(ctx, payload)
		}, "InsertAuditLog")
	if err != nil {
		t.Fatalf("capture changeset: %v", err)
	}

	if len(bytes) == 0 {
		t.Fatalf("capture changeset returned zero bytes")
	}

	return bytes
}

func setLastApplied(t *testing.T, database *Database, v uint64) {
	t.Helper()

	if _, err := database.PlainDB().ExecContext(context.Background(),
		"UPDATE fsm_state SET lastApplied = ? WHERE id = 1", int64(v)); err != nil {
		t.Fatalf("set fsm_state.lastApplied = %d: %v", v, err)
	}
}

func readLastApplied(t *testing.T, database *Database) uint64 {
	t.Helper()

	var v int64

	if err := database.PlainDB().QueryRowContext(context.Background(),
		"SELECT lastApplied FROM fsm_state WHERE id = 1").Scan(&v); err != nil {
		t.Fatalf("read fsm_state.lastApplied: %v", err)
	}

	return uint64(v)
}

func countAuditLogs(t *testing.T, database *Database) int {
	t.Helper()

	var n int

	if err := database.PlainDB().QueryRowContext(context.Background(),
		"SELECT COUNT(*) FROM audit_logs").Scan(&n); err != nil {
		t.Fatalf("count audit_logs: %v", err)
	}

	return n
}

func newAtomicTestDB(t *testing.T) *Database {
	t.Helper()

	tmp := t.TempDir()

	database, err := NewDatabase(context.Background(),
		filepath.Join(tmp, "db.sqlite3"), ellaraft.ClusterConfig{})
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}

	t.Cleanup(func() { _ = database.Close() })

	return database
}

// TestApplyChangeset_AdvancesLastAppliedOnSuccess pins the contract
// that a successful changeset apply advances fsm_state.lastApplied to
// the supplied logIndex in the same SQLite transaction as the apply.
func TestApplyChangeset_AdvancesLastAppliedOnSuccess(t *testing.T) {
	database := newAtomicTestDB(t)
	ctx := context.Background()

	bytes := captureAuditLogChangeset(t, database)

	setLastApplied(t, database, 7)

	if _, err := database.applyChangeset(ctx, &bytesPayload{
		Value:     bytes,
		Operation: "InsertAuditLog",
	}, 42); err != nil {
		t.Fatalf("applyChangeset: %v", err)
	}

	if got := countAuditLogs(t, database); got != 1 {
		t.Fatalf("audit_logs count: want 1, got %d", got)
	}

	if got := readLastApplied(t, database); got != 42 {
		t.Fatalf("lastApplied: want 42, got %d", got)
	}
}

// TestApplyChangeset_RollsBackBothOnConflict pins the failure-path
// half of the same contract: when sqlite3changeset_apply fails, the
// fsm_state.lastApplied write must roll back too. Re-applying the same
// changeset bytes a second time forces a duplicate-PK conflict on the
// audit_logs auto-increment id.
//
// Without the surrounding transaction, the lastApplied write would
// commit even though the apply failed, corrupting crash-recovery
// replay's skip logic.
func TestApplyChangeset_RollsBackBothOnConflict(t *testing.T) {
	database := newAtomicTestDB(t)
	ctx := context.Background()

	bytes := captureAuditLogChangeset(t, database)

	setLastApplied(t, database, 7)

	if _, err := database.applyChangeset(ctx, &bytesPayload{
		Value:     bytes,
		Operation: "InsertAuditLog",
	}, 42); err != nil {
		t.Fatalf("first applyChangeset: %v", err)
	}

	// Sanity-check: post-success state.
	if got := readLastApplied(t, database); got != 42 {
		t.Fatalf("after first apply: lastApplied want 42, got %d", got)
	}

	// Re-apply the same bytes. The captured INSERT carries a concrete
	// auto-increment id; SQLite reports a CONFLICT on the duplicate row.
	_, err := database.applyChangeset(ctx, &bytesPayload{
		Value:     bytes,
		Operation: "InsertAuditLog",
	}, 43)
	if err == nil {
		t.Fatalf("second applyChangeset: want conflict error, got nil")
	}

	if got := countAuditLogs(t, database); got != 1 {
		t.Fatalf("audit_logs count after conflict: want 1, got %d", got)
	}

	if got := readLastApplied(t, database); got != 42 {
		t.Fatalf("lastApplied after conflict: want 42 (rolled back), got %d", got)
	}
}
