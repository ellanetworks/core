// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db_test

import (
	"context"
	"errors"
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

func TestWaitForInitializationImpliesJWTSecret(t *testing.T) {
	t.Parallel()

	database, err := db.NewDatabase(context.Background(), filepath.Join(t.TempDir(), "db.sqlite3"), ellaraft.FastTestConfig())
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}

	defer func() { _ = database.Close() }()

	if err := database.WaitForInitialization(t.Context(), 30*time.Second); err != nil {
		t.Fatalf("initial settings never landed: %v", err)
	}

	secret, err := database.GetJWTSecret(t.Context())
	if err != nil {
		t.Fatalf("the API upgrade gate gets past WaitForInitialization, so the JWT secret must already be seeded: %v", err)
	}

	if len(secret) == 0 {
		t.Fatal("expected a non-empty JWT secret")
	}
}

func TestInitializationSignalMeansSeedingFinished(t *testing.T) {
	t.Parallel()

	database, err := db.NewDatabase(context.Background(), filepath.Join(t.TempDir(), "db.sqlite3"), ellaraft.FastTestConfig())
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}

	defer func() { _ = database.Close() }()

	deadline := time.Now().Add(30 * time.Second)
	for !database.IsOperatorInitialized(t.Context()) {
		if time.Now().After(deadline) {
			t.Fatal("initial settings never landed")
		}
	}

	checks := []struct {
		name string
		fn   func() error
	}{
		{"jwt secret", func() error { _, err := database.GetJWTSecret(t.Context()); return err }},
		{"operator", func() error { _, err := database.GetOperator(t.Context()); return err }},
		{"default data network", func() error { _, err := database.GetDataNetwork(t.Context(), db.InitialDataNetworkName); return err }},
		{"default profile", func() error { _, err := database.GetProfile(t.Context(), db.InitialProfileName); return err }},
		{"default policy", func() error { _, err := database.GetPolicy(t.Context(), db.InitialPolicyName); return err }},
		{"default network slice", func() error { _, err := database.GetNetworkSlice(t.Context(), db.InitialSliceName); return err }},
	}

	for _, c := range checks {
		if err := c.fn(); err != nil {
			t.Errorf("%s missing the moment the readiness signal flipped; the operator row must be the last thing Initialize writes: %v", c.name, err)
		}
	}

	numKeys, err := database.CountHomeNetworkKeys(t.Context())
	if err != nil {
		t.Fatalf("CountHomeNetworkKeys: %v", err)
	}

	if numKeys == 0 {
		t.Error("home network key missing the moment the readiness signal flipped")
	}
}

func TestWriteHonoursCallerCancellationWhileHoldingForLeader(t *testing.T) {
	t.Parallel()

	cfg := ellaraft.FastTestConfig()
	cfg.ElectionTimeout = 10 * time.Second
	cfg.HeartbeatTimeout = 10 * time.Second
	cfg.LeaderLeaseTimeout = 5 * time.Second

	database, err := db.NewDatabase(context.Background(), filepath.Join(t.TempDir(), "db.sqlite3"), cfg)
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}

	defer func() { _ = database.Close() }()

	if database.HasLeader() {
		t.Fatal("no leader should exist this early; the hold cannot be exercised")
	}

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	err = database.InitializeJWTSecret(ctx)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected the write to fail once the caller went away")
	}

	if !errors.Is(err, db.ErrProposeTimeout) {
		t.Errorf("want a transient ErrProposeTimeout the API maps to 503, got %v", err)
	}

	if elapsed > 2*time.Second {
		t.Errorf("write ignored caller cancellation and burned the full budget: took %s", elapsed)
	}
}
