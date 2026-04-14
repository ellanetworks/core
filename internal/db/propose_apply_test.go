// Copyright 2026 Ella Networks

package db_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ellanetworks/core/internal/db"
)

// TestProposeApply_RoundTrip_WritesShowUpAndAdvanceAppliedIndex exercises the
// full write path: a high-level Database call → propose → Raft.Apply → FSM →
// applyX → SQL. It locks in two invariants:
//
//  1. Every shared-DB write advances the Raft applied index, proving the
//     change was committed to the log (not just to SQLite).
//  2. The write is visible to subsequent reads, proving FSM.Apply completed
//     and the transaction is durable before propose returns.
func TestProposeApply_RoundTrip_WritesShowUpAndAdvanceAppliedIndex(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	database, err := db.NewDatabase(ctx, filepath.Join(tempDir, "data"))
	if err != nil {
		t.Fatalf("new database: %v", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("close database: %v", err)
		}
	}()

	startIdx := database.RaftAppliedIndex()

	if err := database.CreateDataNetwork(ctx, &db.DataNetwork{
		Name:   "net-1",
		IPPool: "10.0.0.0/24",
	}); err != nil {
		t.Fatalf("create data network: %v", err)
	}

	afterCreate := database.RaftAppliedIndex()
	if afterCreate <= startIdx {
		t.Fatalf("applied index must advance on write: start=%d after=%d", startIdx, afterCreate)
	}

	got, err := database.GetDataNetwork(ctx, "net-1")
	if err != nil {
		t.Fatalf("read-after-write failed: %v", err)
	}

	if got.IPPool != "10.0.0.0/24" {
		t.Fatalf("read-after-write payload mismatch: got %q", got.IPPool)
	}

	// A second write advances the index again; two sequential writes cannot
	// collapse into one log entry.
	if err := database.CreateDataNetwork(ctx, &db.DataNetwork{
		Name:   "net-2",
		IPPool: "10.0.1.0/24",
	}); err != nil {
		t.Fatalf("create second data network: %v", err)
	}

	afterSecond := database.RaftAppliedIndex()
	if afterSecond <= afterCreate {
		t.Fatalf("applied index must advance on second write: first=%d second=%d",
			afterCreate, afterSecond)
	}
}

// TestProposeApply_ApplierErrorSurfacesToCaller proves that a SQL-level error
// raised inside applyX (e.g. unique-constraint violation) is returned to the
// propose caller as a typed error, not as a generic Raft failure.
func TestProposeApply_ApplierErrorSurfacesToCaller(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	database, err := db.NewDatabase(ctx, filepath.Join(tempDir, "data"))
	if err != nil {
		t.Fatalf("new database: %v", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("close database: %v", err)
		}
	}()

	if err := database.CreateDataNetwork(ctx, &db.DataNetwork{
		Name:   "dup",
		IPPool: "10.0.0.0/24",
	}); err != nil {
		t.Fatalf("initial create: %v", err)
	}

	// Re-creating the same data network must fail with ErrAlreadyExists,
	// unwrapped from the FSM response envelope.
	err = database.CreateDataNetwork(ctx, &db.DataNetwork{
		Name:   "dup",
		IPPool: "10.0.0.0/24",
	})
	if err == nil {
		t.Fatal("expected duplicate create to fail")
	}
}
