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
// command sequence through two independent Database instances (each backed
// by its own SQLite file) and asserts that the resulting shared.db content
// is identical. This catches non-determinism such as time.Now() inside
// applyX, map iteration order influencing SQL output, or reads from local.db
// during a shared.db apply — all of which would silently desync replicas.
func TestFSMDeterminism_ReplayProducesBitIdenticalState(t *testing.T) {
	ctx := context.Background()

	dbA, err := db.NewDatabase(ctx, filepath.Join(t.TempDir(), "a"))
	if err != nil {
		t.Fatalf("create db A: %v", err)
	}

	defer func() { _ = dbA.Close() }()

	dbB, err := db.NewDatabase(ctx, filepath.Join(t.TempDir(), "b"))
	if err != nil {
		t.Fatalf("create db B: %v", err)
	}

	defer func() { _ = dbB.Close() }()

	commands := buildDeterminismCommandStream(t)

	for i, cmd := range commands {
		if _, err := dbA.ApplyCommand(ctx, cmd); err != nil {
			t.Fatalf("apply command %d (%s) to A: %v", i, cmd.Type, err)
		}

		if _, err := dbB.ApplyCommand(ctx, cmd); err != nil {
			t.Fatalf("apply command %d (%s) to B: %v", i, cmd.Type, err)
		}
	}

	// Hash only the tables that the command stream touched and that have
	// fully deterministic content from the replay alone. The operator table
	// is excluded because NewDatabase seeds it with a random operator code;
	// the NAT/flow-accounting settings tables are upserts over a default
	// whose initial value is deterministic, so they are safe to compare.
	touchedTables := []string{"data_networks", "profiles", "nat_settings", "flow_accounting_settings"}

	hashA := canonicalHash(t, dbA.SharedPlainDB(), touchedTables)
	hashB := canonicalHash(t, dbB.SharedPlainDB(), touchedTables)

	if hashA != hashB {
		t.Fatalf("shared.db diverged after replaying %d commands\n  A: %s\n  B: %s", len(commands), hashA, hashB)
	}

	t.Logf("replayed %d commands — databases are identical (sha256: %s)", len(commands), hashA)
}

// buildDeterminismCommandStream returns a fixed sequence of commands covering
// multiple entity types. Payloads are fully deterministic (no timestamps or
// random values). Commands that depend on FK relationships are ordered so
// parents are created first.
func buildDeterminismCommandStream(t *testing.T) []*ellaraft.Command {
	t.Helper()

	type entry struct {
		cmdType ellaraft.CommandType
		payload any
	}

	entries := []entry{
		// Data networks (using names that don't conflict with defaults)
		{ellaraft.CmdCreateDataNetwork, db.DataNetwork{
			Name: "testnet-a", IPPool: "10.99.0.0/24", DNS: "9.9.9.9", MTU: 1300,
		}},
		{ellaraft.CmdCreateDataNetwork, db.DataNetwork{
			Name: "testnet-b", IPPool: "10.98.0.0/24", DNS: "1.0.0.1", MTU: 1500,
		}},
		// Profiles
		{ellaraft.CmdCreateProfile, db.Profile{
			Name:           "det-basic",
			UeAmbrUplink:   "500000 bps",
			UeAmbrDownlink: "2000000 bps",
		}},
		{ellaraft.CmdCreateProfile, db.Profile{
			Name:           "det-premium",
			UeAmbrUplink:   "1000000 bps",
			UeAmbrDownlink: "5000000 bps",
		}},
		// Singleton settings
		{ellaraft.CmdUpdateNATSettings, struct {
			Value bool `json:"value"`
		}{Value: true}},
		{ellaraft.CmdUpdateFlowAccountingSettings, struct {
			Value bool `json:"value"`
		}{Value: false}},
		// Update operator (already initialized by NewDatabase)
		{ellaraft.CmdUpdateOperatorSPN, db.Operator{
			ID:           1,
			SpnFullName:  "UpdatedNet",
			SpnShortName: "UN",
		}},
		{ellaraft.CmdUpdateOperatorTracking, db.Operator{
			ID:            1,
			SupportedTACs: `["0001","0002"]`,
		}},
	}

	cmds := make([]*ellaraft.Command, 0, len(entries))
	for _, e := range entries {
		cmd, err := ellaraft.NewCommand(e.cmdType, e.payload)
		if err != nil {
			t.Fatalf("build command %s: %v", e.cmdType, err)
		}

		cmds = append(cmds, cmd)
	}

	return cmds
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
