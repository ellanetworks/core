// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"path/filepath"
	"testing"

	ellaraft "github.com/ellanetworks/core/internal/raft"
	"github.com/google/uuid"
)

func newAtomicTestDB(t testing.TB) *Database {
	t.Helper()

	database, err := NewDatabase(t.Context(), filepath.Join(t.TempDir(), "db.sqlite3"), ellaraft.FastTestConfig())
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}

	if err := database.WaitUntilReady(t.Context()); err != nil {
		t.Fatalf("database never became ready: %v", err)
	}

	t.Cleanup(func() { _ = database.Close() })

	return database
}

func newTestProfile(t *testing.T, name string) *Profile {
	t.Helper()

	id, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("uuid: %v", err)
	}

	return &Profile{ID: id.String(), Name: name, UeAmbrUplink: "100 Mbps", UeAmbrDownlink: "100 Mbps", Allow4G: true, Allow5G: true}
}

func createTestSubscriber(t *testing.T, database *Database, imsi string) {
	t.Helper()

	profile := newTestProfile(t, "profile-"+imsi)
	if err := database.CreateProfile(t.Context(), profile); err != nil {
		t.Fatalf("CreateProfile: %v", err)
	}

	if err := database.CreateSubscriber(t.Context(), &Subscriber{
		Imsi:           imsi,
		PermanentKey:   "465b5ce8b199b49faa5f0a2ee238a6bc",
		Opc:            "cd63cb71954a9f4e48a5994e37a02baf",
		SequenceNumber: "000000000000",
		ProfileID:      profile.ID,
	}); err != nil {
		t.Fatalf("CreateSubscriber: %v", err)
	}
}

func newDatabaseAtVersion(t *testing.T, version int) *Database {
	t.Helper()

	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "db.sqlite3")

	conn, err := openSQLiteConnection(ctx, dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	if err := runMigrations(ctx, conn, version); err != nil {
		t.Fatalf("runMigrations(%d): %v", version, err)
	}

	if err := ensureFsmStateTable(ctx, conn); err != nil {
		t.Fatalf("ensure fsm_state table: %v", err)
	}

	d := new(Database)
	d.connection = conn
	d.readConnection = conn
	d.dbPath = dbPath
	d.dataDir = filepath.Dir(dbPath)
	d.changefeed = NewChangefeed()

	t.Cleanup(func() {
		if err := d.Close(); err != nil {
			t.Fatalf("close: %v", err)
		}
	})

	if err := d.refreshAppliedSchema(ctx); err != nil {
		t.Fatalf("refresh applied schema: %v", err)
	}

	RegisterMetrics(d)

	return d
}
