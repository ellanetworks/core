// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// SPDX-FileCopyrightText: Ella Networks Inc.

package db

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	ellaraft "github.com/ellanetworks/core/internal/raft"
)

// Whitebox tests for the migration gate. Seeds cluster_members via the
// public upsert API and stubs probeMemberSchema to avoid real peers.

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

func TestMinMemberSchemaSupport_FloorIsLaggard(t *testing.T) {
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

	database.raftMemberIDs = func() []int { return []int{1, 2, 3} }
	database.probeMemberSchema = stubProbe{versions: map[int]int{1: 10, 2: 9, 3: 11}}.probe

	floor, laggard, err := database.minMemberSchemaSupport(ctx)
	if err != nil {
		t.Fatalf("minMemberSchemaSupport: %v", err)
	}

	if floor != 9 {
		t.Fatalf("floor: want 9, got %d", floor)
	}

	if laggard != 2 {
		t.Fatalf("laggard: want 2, got %d", laggard)
	}
}

func TestMinMemberSchemaSupport_UnreachableBlocks(t *testing.T) {
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

	database.raftMemberIDs = func() []int { return []int{1, 2} }
	database.probeMemberSchema = stubProbe{
		versions:    map[int]int{1: 10},
		unreachable: map[int]bool{2: true},
	}.probe

	floor, laggard, err := database.minMemberSchemaSupport(ctx)
	if err != nil {
		t.Fatalf("minMemberSchemaSupport: %v", err)
	}

	if floor != 0 {
		t.Fatalf("floor: want 0 (unreachable blocks), got %d", floor)
	}

	if laggard != 2 {
		t.Fatalf("laggard: want 2, got %d", laggard)
	}
}

// Learners apply committed entries like voters, so an old-binary learner
// holds the floor down.
func TestMinMemberSchemaSupport_LearnerHoldsFloor(t *testing.T) {
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

	database.raftMemberIDs = func() []int { return []int{1, 2} }

	probed := map[int]bool{}

	database.probeMemberSchema = func(_ context.Context, nodeID int, _ string) (int, error) {
		probed[nodeID] = true
		return 3, nil
	}

	floor, laggard, err := database.minMemberSchemaSupport(ctx)
	if err != nil {
		t.Fatalf("minMemberSchemaSupport: %v", err)
	}

	if !probed[2] {
		t.Fatalf("learner node 2 was not probed")
	}

	if floor != 3 {
		t.Fatalf("floor: want 3 (learner), got %d", floor)
	}

	if laggard != 2 {
		t.Fatalf("laggard: want 2, got %d", laggard)
	}
}

// A cluster_members row for a node absent from the Raft configuration
// must not block the gate: it receives no entries.
func TestMinMemberSchemaSupport_SkipsRowsOutsideConfiguration(t *testing.T) {
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

	database.raftMemberIDs = func() []int { return []int{1} }

	database.probeMemberSchema = stubProbe{unreachable: map[int]bool{2: true}}.probe

	floor, _, err := database.minMemberSchemaSupport(ctx)
	if err != nil {
		t.Fatalf("minMemberSchemaSupport: %v", err)
	}

	// node 1 matches the standalone selfID, so its contribution is the
	// in-process SchemaVersion(). Tracking the literal would require a
	// test update on every migration bump.
	if floor != SchemaVersion() {
		t.Fatalf("floor: want %d (phantom row skipped), got %d", SchemaVersion(), floor)
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
