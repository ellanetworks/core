// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/canonical/sqlair"
)

// newSubscriberDatabaseAtV17 builds the cluster-mode local state of a node
// running this binary before the v18 proposal has committed: schema at the
// baseline, statements prepared, applied-schema cache seeded.
func newSubscriberDatabaseAtV17(t *testing.T) *Database {
	t.Helper()

	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "db.sqlite3")

	conn, err := openSQLiteConnection(ctx, dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	if err := runMigrations(ctx, conn, 17); err != nil {
		t.Fatalf("runMigrations(17): %v", err)
	}

	if err := ensureFsmStateTable(ctx, conn); err != nil {
		t.Fatalf("ensure fsm_state table: %v", err)
	}

	d := new(Database)
	d.connPtr.Store(sqlair.NewDB(conn))
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

	if err := d.PrepareStatements(); err != nil {
		t.Fatalf("prepare statements: %v", err)
	}

	return d
}

// TestSubscriberReadsWorkAtBaselineSchema covers the window between a node
// starting on this binary and the v18 migration reaching it through Raft: the
// description column does not exist yet, and the AMF and MME read subscribers
// on every registration.
func TestSubscriberReadsWorkAtBaselineSchema(t *testing.T) {
	ctx := context.Background()
	d := newSubscriberDatabaseAtV17(t)

	// The profile FK is irrelevant to the column set under test.
	if _, err := d.conn().PlainDB().ExecContext(ctx, "PRAGMA foreign_keys = OFF"); err != nil {
		t.Fatalf("disable foreign keys: %v", err)
	}

	_, err := d.conn().PlainDB().ExecContext(ctx, fmt.Sprintf(
		"INSERT INTO %s (id, imsi, sequenceNumber, permanentKey, opc, profileID) VALUES ('01890000-0000-7000-8000-000000000001', '001010000000001', '000000000001', '00112233445566778899aabbccddeeff', '00112233445566778899aabbccddeeff', '01890000-0000-7000-8000-0000000000ff')",
		SubscribersTableName))
	if err != nil {
		t.Fatalf("insert subscriber: %v", err)
	}

	sub, err := d.GetSubscriber(ctx, "001010000000001")
	if err != nil {
		t.Fatalf("GetSubscriber at schema 17: %v", err)
	}

	if sub.Description != "" {
		t.Fatalf("description = %q, want %q", sub.Description, "")
	}

	search := "0000001"

	subs, total, err := d.ListSubscribersPage(ctx, &SubscriberFilters{Search: &search}, 1, 25)
	if err != nil {
		t.Fatalf("ListSubscribersPage(search) at schema 17: %v", err)
	}

	if total != 1 || len(subs) != 1 || subs[0].Imsi != "001010000000001" {
		t.Fatalf("search by imsi at schema 17: total=%d subs=%v", total, subs)
	}
}

// TestSubscriberWritesAreGatedAtBaselineSchema pins the other half: writes
// that would capture a changeset shaped by the v18 column are refused with a
// retryable error until the migration applies.
func TestSubscriberWritesAreGatedAtBaselineSchema(t *testing.T) {
	ctx := context.Background()
	d := newSubscriberDatabaseAtV17(t)

	sub := &Subscriber{
		ID:             "01890000-0000-7000-8000-000000000001",
		Imsi:           "001010000000001",
		SequenceNumber: "000000000001",
		PermanentKey:   "00112233445566778899aabbccddeeff",
		Opc:            "00112233445566778899aabbccddeeff",
		ProfileID:      "01890000-0000-7000-8000-0000000000ff",
	}

	if err := d.CreateSubscriber(ctx, sub); !errors.Is(err, ErrMigrationPending) {
		t.Fatalf("CreateSubscriber at schema 17: want ErrMigrationPending, got %v", err)
	}

	if err := d.UpdateSubscriberProfile(ctx, sub); !errors.Is(err, ErrMigrationPending) {
		t.Fatalf("UpdateSubscriberProfile at schema 17: want ErrMigrationPending, got %v", err)
	}
}
