// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/canonical/sqlair"
)

// TestBootstrapSeedWorksAtBaselineSchema reproduces the HA leader's bootstrap
// condition: in cluster mode NewDatabase migrates the local schema only to
// baselineVersion, then runs Initialize() synchronously on becoming leader —
// before CheckPendingMigrations proposes any post-baseline migration over Raft.
// The seed must therefore succeed against the baseline-only schema. This guards
// against a seed that depends on a post-baseline migration (e.g. migration 14's
// allow4G/isDefault columns), which manifested as "table profiles has no column
// named allow4G" and left HA clusters with no default profile.
func TestBootstrapSeedWorksAtBaselineSchema(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "db.sqlite3")

	conn, err := openSQLiteConnection(ctx, dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	// The cluster-mode local schema: baseline only (cf. NewDatabase).
	if err := runMigrations(ctx, conn, baselineVersion); err != nil {
		t.Fatalf("runMigrations(baselineVersion=%d): %v", baselineVersion, err)
	}

	if err := ensureFsmStateTable(ctx, conn); err != nil {
		t.Fatalf("ensure fsm_state table: %v", err)
	}

	d := new(Database)
	d.connPtr.Store(sqlair.NewDB(conn))
	d.dbPath = dbPath
	d.dataDir = filepath.Dir(dbPath)
	d.changefeed = NewChangefeed()

	defer func() {
		if err := d.Close(); err != nil {
			t.Fatalf("close: %v", err)
		}
	}()

	if err := d.refreshAppliedSchema(ctx); err != nil {
		t.Fatalf("refresh applied schema: %v", err)
	}

	if err := d.PrepareStatements(); err != nil {
		t.Fatalf("prepare statements: %v", err)
	}

	RegisterMetrics(d)

	if err := d.InitializeLocalSettings(ctx); err != nil {
		t.Fatalf("initialize local settings: %v", err)
	}

	if err := d.Initialize(ctx); err != nil {
		t.Fatalf("Initialize against baseline schema: %v", err)
	}

	if _, err := d.GetProfile(ctx, InitialProfileName); err != nil {
		t.Fatalf("default profile not seeded after Initialize: %v", err)
	}
}
