package db_test

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/ellanetworks/core/internal/db"
	ellaraft "github.com/ellanetworks/core/internal/raft"
	_ "github.com/mattn/go-sqlite3"
)

// TestFSMDeterminism_ReplayProducesBitIdenticalState replays the same
// deterministic write sequence through two independent Database instances
// and asserts that the resulting shared.db content is identical.
func TestFSMDeterminism_ReplayProducesBitIdenticalState(t *testing.T) {
	ctx := context.Background()

	dbA, err := db.NewDatabase(ctx, filepath.Join(t.TempDir(), "a"), ellaraft.ClusterConfig{})
	if err != nil {
		t.Fatalf("create db A: %v", err)
	}

	defer func() { _ = dbA.Close() }()

	dbB, err := db.NewDatabase(ctx, filepath.Join(t.TempDir(), "b"), ellaraft.ClusterConfig{})
	if err != nil {
		t.Fatalf("create db B: %v", err)
	}

	defer func() { _ = dbB.Close() }()

	operations := buildDeterminismOperations()

	for i, op := range operations {
		if err := op(ctx, dbA); err != nil {
			t.Fatalf("apply operation %d to A: %v", i, err)
		}

		if err := op(ctx, dbB); err != nil {
			t.Fatalf("apply operation %d to B: %v", i, err)
		}
	}

	// Hash only the tables that the command stream touched and that have
	// fully deterministic content from the replay alone. The operator table
	// is excluded because NewDatabase seeds it with a random operator code;
	// the NAT/flow-accounting settings tables are upserts over a default
	// whose initial value is deterministic, so they are safe to compare.
	touchedTables := []string{"data_networks", "profiles", "nat_settings", "flow_accounting_settings"}

	hashA := canonicalHash(t, dbA.PlainDB(), touchedTables)
	hashB := canonicalHash(t, dbB.PlainDB(), touchedTables)

	if hashA != hashB {
		t.Fatalf("database diverged after replaying %d operations\n  A: %s\n  B: %s", len(operations), hashA, hashB)
	}

	t.Logf("replayed %d operations - databases are identical (sha256: %s)", len(operations), hashA)
}

func buildDeterminismOperations() []func(context.Context, *db.Database) error {
	return []func(context.Context, *db.Database) error{
		func(ctx context.Context, d *db.Database) error {
			return d.CreateDataNetwork(ctx, &db.DataNetwork{Name: "testnet-a", IPPool: "10.99.0.0/24", DNS: "9.9.9.9", MTU: 1300})
		},
		func(ctx context.Context, d *db.Database) error {
			return d.CreateDataNetwork(ctx, &db.DataNetwork{Name: "testnet-b", IPPool: "10.98.0.0/24", DNS: "1.0.0.1", MTU: 1500})
		},
		func(ctx context.Context, d *db.Database) error {
			return d.CreateProfile(ctx, &db.Profile{Name: "det-basic", UeAmbrUplink: "500000 bps", UeAmbrDownlink: "2000000 bps"})
		},
		func(ctx context.Context, d *db.Database) error {
			return d.CreateProfile(ctx, &db.Profile{Name: "det-premium", UeAmbrUplink: "1000000 bps", UeAmbrDownlink: "5000000 bps"})
		},
		func(ctx context.Context, d *db.Database) error {
			return d.UpdateNATSettings(ctx, true)
		},
		func(ctx context.Context, d *db.Database) error {
			return d.UpdateFlowAccountingSettings(ctx, false)
		},
		func(ctx context.Context, d *db.Database) error {
			return d.UpdateOperatorSPN(ctx, "UpdatedNet", "UN")
		},
		func(ctx context.Context, d *db.Database) error {
			return d.UpdateOperatorTracking(ctx, []string{"0001", "0002"})
		},
	}
}

// canonicalHash produces a SHA-256 digest of the specified tables' content in
// a deterministic order. Each table's rows are sorted lexicographically so
// internal SQLite page layout differences don't cause false negatives.
func canonicalHash(t *testing.T, plainDB *sql.DB, tables []string) string {
	t.Helper()

	sort.Strings(tables)

	h := sha256.New()

	for _, table := range tables {
		_, _ = fmt.Fprintf(h, "TABLE:%s\n", table)
		dumpTable(t, plainDB, table, h)
	}

	return fmt.Sprintf("%x", h.Sum(nil))
}

func dumpTable(t *testing.T, plainDB *sql.DB, table string, h io.Writer) {
	t.Helper()

	query := fmt.Sprintf("SELECT * FROM %q ORDER BY rowid", table)

	rows, err := plainDB.QueryContext(context.Background(), query)
	if err != nil {
		t.Fatalf("query %s: %v", table, err)
	}

	defer func() { _ = rows.Close() }()

	cols, err := rows.Columns()
	if err != nil {
		t.Fatalf("columns %s: %v", table, err)
	}

	var sortedRows []string

	for rows.Next() {
		vals := make([]any, len(cols))

		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}

		if err := rows.Scan(ptrs...); err != nil {
			t.Fatalf("scan %s: %v", table, err)
		}

		var parts []string
		for i, col := range cols {
			parts = append(parts, fmt.Sprintf("%s=%v", col, vals[i]))
		}

		sortedRows = append(sortedRows, strings.Join(parts, "|"))
	}

	if err := rows.Err(); err != nil {
		t.Fatalf("iterate %s: %v", table, err)
	}

	sort.Strings(sortedRows)

	for _, row := range sortedRows {
		_, _ = fmt.Fprintf(h, "%s\n", row)
	}
}
