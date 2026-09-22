// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db_test

import (
	"context"
	"errors"
	"testing"

	"github.com/ellanetworks/core/internal/db"
)

func drainStateOf(ctx context.Context, t *testing.T, database *db.Database, nodeID string) db.ClusterMember {
	t.Helper()

	member, err := database.GetClusterMember(ctx, nodeID)
	if err != nil {
		t.Fatalf("get cluster member: %s", err)
	}

	return *member
}

func TestSetDrainStateReturnsTheEffectiveState(t *testing.T) {
	database := setupTestDB(t)
	ctx := context.Background()

	const nodeID = "11111111-1111-1111-1111-111111111111"

	if err := database.UpsertClusterMember(ctx, &db.ClusterMember{
		NodeID:     nodeID,
		APIAddress: "10.0.0.1:5002",
	}); err != nil {
		t.Fatalf("upsert cluster member: %s", err)
	}

	state, err := database.SetDrainState(ctx, nodeID, db.DrainStateDraining)
	if err != nil {
		t.Fatalf("drain: %s", err)
	}

	if state != db.DrainStateDraining {
		t.Fatalf("drain returned %q, want %q", state, db.DrainStateDraining)
	}

	first := drainStateOf(ctx, t, database, nodeID)
	if first.DrainUpdatedAt == 0 {
		t.Fatal("drainUpdatedAt was not stamped on the transition")
	}

	state, err = database.SetDrainState(ctx, nodeID, db.DrainStateDrained)
	if err != nil {
		t.Fatalf("mark drained: %s", err)
	}

	if state != db.DrainStateDrained {
		t.Fatalf("mark drained returned %q, want %q", state, db.DrainStateDrained)
	}

	state, err = database.SetDrainState(ctx, nodeID, db.DrainStateDraining)
	if err != nil {
		t.Fatalf("re-drain: %s", err)
	}

	if state != db.DrainStateDrained {
		t.Fatalf("re-draining a drained node returned %q, want %q", state, db.DrainStateDrained)
	}

	if got := drainStateOf(ctx, t, database, nodeID); got.DrainState != db.DrainStateDrained {
		t.Fatalf("stored state = %q, want %q", got.DrainState, db.DrainStateDrained)
	}
}

func TestSetDrainStateRefusedTransitionWritesNothing(t *testing.T) {
	database := setupTestDB(t)
	ctx := context.Background()

	const nodeID = "22222222-2222-2222-2222-222222222222"

	if err := database.UpsertClusterMember(ctx, &db.ClusterMember{
		NodeID:     nodeID,
		APIAddress: "10.0.0.2:5002",
	}); err != nil {
		t.Fatalf("upsert cluster member: %s", err)
	}

	if _, err := database.SetDrainState(ctx, nodeID, db.DrainStateDraining); err != nil {
		t.Fatalf("drain: %s", err)
	}

	before := drainStateOf(ctx, t, database, nodeID)

	state, err := database.SetDrainState(ctx, nodeID, db.DrainStateDraining)
	if err != nil {
		t.Fatalf("second drain: %s", err)
	}

	if state != db.DrainStateDraining {
		t.Fatalf("second drain returned %q, want %q", state, db.DrainStateDraining)
	}

	after := drainStateOf(ctx, t, database, nodeID)
	if after.DrainUpdatedAt != before.DrainUpdatedAt {
		t.Fatalf("drainUpdatedAt moved from %d to %d on a refused transition; the drain deadline would never fire",
			before.DrainUpdatedAt, after.DrainUpdatedAt)
	}
}

func TestSetDrainStateDoesNotDrainAResumedNode(t *testing.T) {
	database := setupTestDB(t)
	ctx := context.Background()

	const nodeID = "33333333-3333-3333-3333-333333333333"

	if err := database.UpsertClusterMember(ctx, &db.ClusterMember{
		NodeID:     nodeID,
		APIAddress: "10.0.0.3:5002",
	}); err != nil {
		t.Fatalf("upsert cluster member: %s", err)
	}

	if _, err := database.SetDrainState(ctx, nodeID, db.DrainStateDraining); err != nil {
		t.Fatalf("drain: %s", err)
	}

	if _, err := database.SetDrainState(ctx, nodeID, db.DrainStateActive); err != nil {
		t.Fatalf("resume: %s", err)
	}

	state, err := database.SetDrainState(ctx, nodeID, db.DrainStateDrained)
	if err != nil {
		t.Fatalf("mark drained after resume: %s", err)
	}

	if state != db.DrainStateActive {
		t.Fatalf("marking a resumed node drained returned %q, want %q", state, db.DrainStateActive)
	}

	if got := drainStateOf(ctx, t, database, nodeID); got.DrainState != db.DrainStateActive {
		t.Fatalf("stored state = %q, want %q; a late sweep clobbered the resume", got.DrainState, db.DrainStateActive)
	}
}

func TestSetDrainStateUnknownNode(t *testing.T) {
	database := setupTestDB(t)
	ctx := context.Background()

	_, err := database.SetDrainState(context.WithoutCancel(ctx), "44444444-4444-4444-4444-444444444444", db.DrainStateDraining)
	if !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("err = %v, want db.ErrNotFound", err)
	}
}
