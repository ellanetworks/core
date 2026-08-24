// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/db"
	ellaraft "github.com/ellanetworks/core/internal/raft"
)

func TestNewDatabaseDoesNotWaitForElection(t *testing.T) {
	t.Parallel()

	cfg := ellaraft.FastTestConfig()
	cfg.ElectionTimeout = 10 * time.Second
	cfg.HeartbeatTimeout = 10 * time.Second
	cfg.LeaderLeaseTimeout = 5 * time.Second

	start := time.Now()

	database, err := db.NewDatabase(context.Background(), filepath.Join(t.TempDir(), "db.sqlite3"), cfg)
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}

	defer func() { _ = database.Close() }()

	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("NewDatabase blocked for %s waiting on leadership", elapsed)
	}

	if database.HasLeader() {
		t.Fatal("no leader should exist this early")
	}

	if database.IsOperatorInitialized(context.Background()) {
		t.Fatal("replicated settings cannot be seeded before an election")
	}
}

func TestStandaloneSeedsInitialSettingsAfterElection(t *testing.T) {
	t.Parallel()

	database, err := db.NewDatabase(context.Background(), filepath.Join(t.TempDir(), "db.sqlite3"), ellaraft.FastTestConfig())
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}

	defer func() { _ = database.Close() }()

	if err := database.WaitUntilReady(t.Context()); err != nil {
		t.Fatalf("database never became ready: %v", err)
	}

	if !database.IsOperatorInitialized(context.Background()) {
		t.Fatal("expected operator row to be seeded once leadership was acquired")
	}

	op, err := database.GetOperator(context.Background())
	if err != nil {
		t.Fatalf("GetOperator: %v", err)
	}

	if op.ClusterID == "" {
		t.Error("expected a cluster ID to be generated during seeding")
	}
}

func TestWriteIssuedBeforeElectionHoldsForLeader(t *testing.T) {
	t.Parallel()

	database, err := db.NewDatabase(context.Background(), filepath.Join(t.TempDir(), "db.sqlite3"), ellaraft.FastTestConfig())
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}

	defer func() { _ = database.Close() }()

	if database.HasLeader() {
		t.Skip("election already completed; nothing to hold for")
	}

	if err := database.InitializeJWTSecret(context.Background()); err != nil {
		t.Fatalf("write before election should hold for the leader, got: %v", err)
	}
}
