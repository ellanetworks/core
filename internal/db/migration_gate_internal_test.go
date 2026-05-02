// Copyright 2026 Ella Networks

package db

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	ellaraft "github.com/ellanetworks/core/internal/raft"
)

// Whitebox tests for the schema-migration proposal gate.
// minVoterSchemaSupport is unexported and the full
// CheckPendingMigrations path requires a leader; here we seed
// cluster_members directly via the public upsert API and inject a
// probe stub so the gate can be exercised without spinning up real
// peers.

func newStandaloneDB(t *testing.T) *Database {
	t.Helper()

	tmp := t.TempDir()

	database, err := NewDatabase(context.Background(), filepath.Join(tmp, "db.sqlite3"), ellaraft.ClusterConfig{})
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}

	t.Cleanup(func() { _ = database.Close() })

	return database
}

// stubProbe builds a probe function that returns the version recorded
// for each nodeID, or the configured error when the entry is missing
// (used to simulate "voter unreachable").
type stubProbe struct {
	versions    map[int]int
	unreachable map[int]bool
}

func (s stubProbe) probe(_ context.Context, nodeID int, _ string) (int, error) {
	if s.unreachable[nodeID] {
		return 0, errors.New("simulated peer unreachable")
	}

	v, ok := s.versions[nodeID]
	if !ok {
		return 0, errors.New("no stub for nodeID")
	}

	return v, nil
}

func TestMinVoterSchemaSupport_FloorIsLaggard(t *testing.T) {
	database := newStandaloneDB(t)
	ctx := context.Background()

	for _, m := range []*ClusterMember{
		{NodeID: 1, RaftAddress: "a:1", APIAddress: "a:2", Suffrage: "voter"},
		{NodeID: 2, RaftAddress: "b:1", APIAddress: "b:2", Suffrage: "voter"},
		{NodeID: 3, RaftAddress: "c:1", APIAddress: "c:2", Suffrage: "voter"},
	} {
		if err := database.UpsertClusterMember(ctx, m); err != nil {
			t.Fatalf("seed member %d: %v", m.NodeID, err)
		}
	}

	// Standalone DBs report nodeID 0, so no member matches "self" and
	// every voter is probed.
	database.probeVoterSchema = stubProbe{versions: map[int]int{1: 10, 2: 9, 3: 11}}.probe

	floor, laggard, err := database.minVoterSchemaSupport(ctx)
	if err != nil {
		t.Fatalf("minVoterSchemaSupport: %v", err)
	}

	if floor != 9 {
		t.Fatalf("floor: want 9, got %d", floor)
	}

	if laggard != 2 {
		t.Fatalf("laggard: want 2, got %d", laggard)
	}
}

func TestMinVoterSchemaSupport_UnreachableBlocks(t *testing.T) {
	database := newStandaloneDB(t)
	ctx := context.Background()

	for _, m := range []*ClusterMember{
		{NodeID: 1, RaftAddress: "a:1", APIAddress: "a:2", Suffrage: "voter"},
		{NodeID: 2, RaftAddress: "b:1", APIAddress: "b:2", Suffrage: "voter"},
	} {
		if err := database.UpsertClusterMember(ctx, m); err != nil {
			t.Fatalf("seed member %d: %v", m.NodeID, err)
		}
	}

	database.probeVoterSchema = stubProbe{
		versions:    map[int]int{1: 10},
		unreachable: map[int]bool{2: true},
	}.probe

	floor, laggard, err := database.minVoterSchemaSupport(ctx)
	if err != nil {
		t.Fatalf("minVoterSchemaSupport: %v", err)
	}

	if floor != 0 {
		t.Fatalf("floor: want 0 (unreachable blocks), got %d", floor)
	}

	if laggard != 2 {
		t.Fatalf("laggard: want 2, got %d", laggard)
	}
}

func TestMinVoterSchemaSupport_IgnoresLearners(t *testing.T) {
	database := newStandaloneDB(t)
	ctx := context.Background()

	for _, m := range []*ClusterMember{
		{NodeID: 1, RaftAddress: "a:1", APIAddress: "a:2", Suffrage: "voter"},
		{NodeID: 2, RaftAddress: "b:1", APIAddress: "b:2", Suffrage: "nonvoter"},
	} {
		if err := database.UpsertClusterMember(ctx, m); err != nil {
			t.Fatalf("seed member %d: %v", m.NodeID, err)
		}
	}

	probed := map[int]bool{}

	database.probeVoterSchema = func(_ context.Context, nodeID int, _ string) (int, error) {
		probed[nodeID] = true
		return 10, nil
	}

	floor, laggard, err := database.minVoterSchemaSupport(ctx)
	if err != nil {
		t.Fatalf("minVoterSchemaSupport: %v", err)
	}

	if floor != 10 {
		t.Fatalf("floor: want 10 (learner ignored), got %d", floor)
	}

	if laggard != 1 {
		t.Fatalf("laggard: want 1, got %d", laggard)
	}

	if probed[2] {
		t.Fatalf("learner node 2 must not be probed")
	}
}

func TestRequireSchema(t *testing.T) {
	database := newStandaloneDB(t)
	ctx := context.Background()

	applied, err := database.CurrentSchemaVersion(ctx)
	if err != nil {
		t.Fatalf("CurrentSchemaVersion: %v", err)
	}

	if err := database.RequireSchema(ctx, applied); err != nil {
		t.Fatalf("RequireSchema(current) unexpected error: %v", err)
	}

	if err := database.RequireSchema(ctx, applied+1); err != ErrMigrationPending {
		t.Fatalf("RequireSchema(current+1): want ErrMigrationPending, got %v", err)
	}
}

func TestClusterMember_MaxSchemaVersionRoundtrip(t *testing.T) {
	database := newStandaloneDB(t)
	ctx := context.Background()

	m := &ClusterMember{
		NodeID:           7,
		RaftAddress:      "10.0.0.7:8300",
		APIAddress:       "10.0.0.7:8443",
		Suffrage:         "voter",
		MaxSchemaVersion: 42,
	}

	if err := database.UpsertClusterMember(ctx, m); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	got, err := database.GetClusterMember(ctx, 7)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if got.MaxSchemaVersion != 42 {
		t.Fatalf("MaxSchemaVersion: want 42, got %d", got.MaxSchemaVersion)
	}
}
