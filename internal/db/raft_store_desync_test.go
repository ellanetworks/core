// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ellanetworks/core/internal/db"
	ellaraft "github.com/ellanetworks/core/internal/raft"
	_ "github.com/mattn/go-sqlite3"
)

func seedSubscriber(t *testing.T, database *db.Database, imsi string) {
	t.Helper()

	profileID, err := createDataNetworkAndPolicy(database)
	if err != nil {
		t.Fatalf("create prerequisites: %s", err)
	}

	if err := database.CreateSubscriber(context.Background(), &db.Subscriber{
		Imsi:           imsi,
		SequenceNumber: "000000000001",
		PermanentKey:   "6f30087629feb0b089783c81d0ae09b5",
		Opc:            "21a7e1897dfb481d62439142cdf1b6ee",
		ProfileID:      profileID,
	}); err != nil {
		t.Fatalf("create subscriber: %s", err)
	}
}

func resetLastApplied(t *testing.T, dbPath string) {
	t.Helper()

	conn, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("open database: %s", err)
	}

	defer func() { _ = conn.Close() }()

	if _, err := conn.ExecContext(context.Background(), "UPDATE fsm_state SET lastApplied = 0 WHERE id = 1"); err != nil {
		t.Fatalf("reset lastApplied: %s", err)
	}
}

func TestStartupRejectsDatabaseAheadOfRaftStore(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "ella.db")

	database, err := db.NewDatabase(ctx, dbPath, ellaraft.FastTestConfig())
	if err != nil {
		t.Fatalf("new database: %s", err)
	}

	if err := database.WaitUntilReady(t.Context()); err != nil {
		t.Fatalf("database never became ready: %s", err)
	}

	seedSubscriber(t, database, "999010071245191")

	if err := database.Close(); err != nil {
		t.Fatalf("close database: %s", err)
	}

	if err := os.RemoveAll(filepath.Join(tempDir, "raft")); err != nil {
		t.Fatalf("remove raft store: %s", err)
	}

	_, err = db.NewDatabase(ctx, dbPath, ellaraft.FastTestConfig())
	if err == nil {
		t.Fatal("expected startup to fail when the database is ahead of the raft store")
	}

	if !strings.Contains(err.Error(), "database is ahead of the raft store") {
		t.Fatalf("unexpected error: %s", err)
	}
}

func TestStartupAcceptsResetLastAppliedAfterRaftStoreWipe(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "ella.db")

	database, err := db.NewDatabase(ctx, dbPath, ellaraft.FastTestConfig())
	if err != nil {
		t.Fatalf("new database: %s", err)
	}

	if err := database.WaitUntilReady(t.Context()); err != nil {
		t.Fatalf("database never became ready: %s", err)
	}

	seedSubscriber(t, database, "999010071245191")

	if err := database.Close(); err != nil {
		t.Fatalf("close database: %s", err)
	}

	if err := os.RemoveAll(filepath.Join(tempDir, "raft")); err != nil {
		t.Fatalf("remove raft store: %s", err)
	}

	resetLastApplied(t, dbPath)

	recovered, err := db.NewDatabase(ctx, dbPath, ellaraft.FastTestConfig())
	if err != nil {
		t.Fatalf("reopen after reset: %s", err)
	}

	if err := recovered.WaitUntilReady(t.Context()); err != nil {
		t.Fatalf("database never became ready after reset: %s", err)
	}

	defer func() {
		if err := recovered.Close(); err != nil {
			t.Fatalf("close database: %s", err)
		}
	}()

	if err := recovered.CreateSubscriber(ctx, &db.Subscriber{
		Imsi:           "999010071245192",
		SequenceNumber: "000000000001",
		PermanentKey:   "6f30087629feb0b089783c81d0ae09b5",
		Opc:            "21a7e1897dfb481d62439142cdf1b6ee",
		ProfileID:      mustProfileID(t, recovered),
	}); err != nil {
		t.Fatalf("create subscriber after reset: %s", err)
	}

	_, total, err := recovered.ListSubscribersPage(ctx, nil, 1, 10)
	if err != nil {
		t.Fatalf("list subscribers: %s", err)
	}

	if total != 2 {
		t.Fatalf("expected 2 subscribers after reset, got %d", total)
	}
}

func mustProfileID(t *testing.T, database *db.Database) string {
	t.Helper()

	profile, err := database.GetProfile(context.Background(), "test-profile")
	if err != nil {
		t.Fatalf("get profile: %s", err)
	}

	return profile.ID
}
