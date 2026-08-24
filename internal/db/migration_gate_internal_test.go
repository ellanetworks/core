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

func newStandaloneDB(t *testing.T) *Database {
	t.Helper()

	tmp := t.TempDir()

	database, err := NewDatabase(context.Background(), filepath.Join(tmp, "db.sqlite3"), ellaraft.FastTestConfig())
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}

	if err := database.WaitUntilReady(t.Context()); err != nil {
		t.Fatalf("database never became ready: %v", err)
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

	if floor != SchemaVersion() {
		t.Fatalf("floor: want %d (phantom row skipped), got %d", SchemaVersion(), floor)
	}
}

func TestMinMemberSchemaSupport_ConfigurationMemberWithoutRowBlocks(t *testing.T) {
	database := newStandaloneDB(t)
	ctx := context.Background()

	if err := database.UpsertClusterMember(ctx, &ClusterMember{
		NodeID: 1, RaftAddress: "a:1", APIAddress: "a:2", Suffrage: "voter",
	}); err != nil {
		t.Fatalf("seed member 1: %v", err)
	}

	database.raftMemberIDs = func() []int { return []int{1, 2} }
	database.probeMemberSchema = stubProbe{versions: map[int]int{1: 10}}.probe

	floor, laggard, err := database.minMemberSchemaSupport(ctx)
	if err != nil {
		t.Fatalf("minMemberSchemaSupport: %v", err)
	}

	if floor != 0 {
		t.Fatalf("floor: want 0 (member with no row blocks), got %d", floor)
	}

	if laggard != 2 {
		t.Fatalf("laggard: want 2, got %d", laggard)
	}
}

func TestMinMemberSchemaSupport_UnavailableConfigurationBlocks(t *testing.T) {
	database := newStandaloneDB(t)
	ctx := context.Background()

	if err := database.UpsertClusterMember(ctx, &ClusterMember{
		NodeID: 2, RaftAddress: "b:1", APIAddress: "b:2", Suffrage: "voter",
	}); err != nil {
		t.Fatalf("seed member 2: %v", err)
	}

	database.raftMemberIDs = func() []int { return nil }
	database.probeMemberSchema = stubProbe{unreachable: map[int]bool{2: true}}.probe

	floor, _, err := database.minMemberSchemaSupport(ctx)
	if err != nil {
		t.Fatalf("minMemberSchemaSupport: %v", err)
	}

	if floor != 0 {
		t.Fatalf("floor: want 0 (configuration unavailable blocks), got %d", floor)
	}
}

func TestPendingMigrationInfo_UnreachableMemberReportsLaggard(t *testing.T) {
	database := newStandaloneDB(t)
	ctx := context.Background()

	if err := database.UpsertClusterMember(ctx, &ClusterMember{
		NodeID: 2, RaftAddress: "b:1", APIAddress: "b:2", Suffrage: "nonvoter",
	}); err != nil {
		t.Fatalf("seed member 2: %v", err)
	}

	current, err := database.CurrentSchemaVersion(ctx)
	if err != nil {
		t.Fatalf("CurrentSchemaVersion: %v", err)
	}

	if _, err := database.PlainDB().ExecContext(ctx,
		"UPDATE schema_version SET version = ? WHERE id = 1", current-1); err != nil {
		t.Fatalf("lower schema version: %v", err)
	}

	database.raftMemberIDs = func() []int { return []int{2} }
	database.probeMemberSchema = stubProbe{unreachable: map[int]bool{2: true}}.probe

	status, err := database.PendingMigrationInfo(ctx)
	if err != nil {
		t.Fatalf("PendingMigrationInfo: %v", err)
	}

	if status.TargetSchema != current-1 {
		t.Fatalf("TargetSchema: want %d (current), got %d", current-1, status.TargetSchema)
	}

	if status.LaggardNodeID != 2 {
		t.Fatalf("LaggardNodeID: want 2, got %d", status.LaggardNodeID)
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
