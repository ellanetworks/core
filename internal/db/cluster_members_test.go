package db_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ellanetworks/core/internal/db"
)

func TestDBClusterMembersEndToEnd(t *testing.T) {
	tempDir := t.TempDir()

	database, err := db.NewDatabaseWithoutRaft(context.Background(), filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	}()

	ctx := context.Background()

	// List should be empty initially
	members, err := database.ListClusterMembers(ctx)
	if err != nil {
		t.Fatalf("Couldn't list cluster members: %s", err)
	}

	if len(members) != 0 {
		t.Fatalf("Expected no members, got %d", len(members))
	}

	// Count should be 0
	count, err := database.CountClusterMembers(ctx)
	if err != nil {
		t.Fatalf("Couldn't count cluster members: %s", err)
	}

	if count != 0 {
		t.Fatalf("Expected count to be 0, got %d", count)
	}

	// Upsert a member
	member1 := &db.ClusterMember{
		NodeID:      1,
		RaftAddress: "10.0.0.1:8300",
		APIAddress:  "10.0.0.1:8443",
	}

	err = database.UpsertClusterMember(ctx, member1)
	if err != nil {
		t.Fatalf("Couldn't upsert cluster member: %s", err)
	}

	// List should have 1 member
	members, err = database.ListClusterMembers(ctx)
	if err != nil {
		t.Fatalf("Couldn't list cluster members: %s", err)
	}

	if len(members) != 1 {
		t.Fatalf("Expected 1 member, got %d", len(members))
	}

	if members[0].NodeID != 1 {
		t.Fatalf("Expected nodeID 1, got %d", members[0].NodeID)
	}

	if members[0].RaftAddress != "10.0.0.1:8300" {
		t.Fatalf("Expected raftAddress 10.0.0.1:8300, got %s", members[0].RaftAddress)
	}

	if members[0].APIAddress != "10.0.0.1:8443" {
		t.Fatalf("Expected apiAddress 10.0.0.1:8443, got %s", members[0].APIAddress)
	}

	// Get member by ID
	retrieved, err := database.GetClusterMember(ctx, 1)
	if err != nil {
		t.Fatalf("Couldn't get cluster member: %s", err)
	}

	if retrieved.RaftAddress != "10.0.0.1:8300" {
		t.Fatalf("Expected raftAddress 10.0.0.1:8300, got %s", retrieved.RaftAddress)
	}

	// Upsert same member with updated address (should update, not duplicate)
	member1Updated := &db.ClusterMember{
		NodeID:      1,
		RaftAddress: "10.0.0.1:9300",
		APIAddress:  "10.0.0.1:9443",
	}

	err = database.UpsertClusterMember(ctx, member1Updated)
	if err != nil {
		t.Fatalf("Couldn't upsert updated cluster member: %s", err)
	}

	// Count should still be 1
	count, err = database.CountClusterMembers(ctx)
	if err != nil {
		t.Fatalf("Couldn't count cluster members: %s", err)
	}

	if count != 1 {
		t.Fatalf("Expected count to be 1 after upsert, got %d", count)
	}

	// Verify updated address
	retrieved, err = database.GetClusterMember(ctx, 1)
	if err != nil {
		t.Fatalf("Couldn't get updated cluster member: %s", err)
	}

	if retrieved.RaftAddress != "10.0.0.1:9300" {
		t.Fatalf("Expected updated raftAddress 10.0.0.1:9300, got %s", retrieved.RaftAddress)
	}

	if retrieved.APIAddress != "10.0.0.1:9443" {
		t.Fatalf("Expected updated apiAddress 10.0.0.1:9443, got %s", retrieved.APIAddress)
	}

	// Add a second member
	member2 := &db.ClusterMember{
		NodeID:      2,
		RaftAddress: "10.0.0.2:8300",
		APIAddress:  "10.0.0.2:8443",
	}

	err = database.UpsertClusterMember(ctx, member2)
	if err != nil {
		t.Fatalf("Couldn't upsert second cluster member: %s", err)
	}

	// Count should be 2
	count, err = database.CountClusterMembers(ctx)
	if err != nil {
		t.Fatalf("Couldn't count cluster members: %s", err)
	}

	if count != 2 {
		t.Fatalf("Expected count to be 2, got %d", count)
	}

	// List should return members ordered by nodeID
	members, err = database.ListClusterMembers(ctx)
	if err != nil {
		t.Fatalf("Couldn't list cluster members: %s", err)
	}

	if len(members) != 2 {
		t.Fatalf("Expected 2 members, got %d", len(members))
	}

	if members[0].NodeID != 1 {
		t.Fatalf("Expected first member nodeID 1, got %d", members[0].NodeID)
	}

	if members[1].NodeID != 2 {
		t.Fatalf("Expected second member nodeID 2, got %d", members[1].NodeID)
	}

	// Get non-existent member should return ErrNotFound
	_, err = database.GetClusterMember(ctx, 999)
	if err != db.ErrNotFound {
		t.Fatalf("Expected ErrNotFound, got: %v", err)
	}

	// Delete first member
	err = database.DeleteClusterMember(ctx, 1)
	if err != nil {
		t.Fatalf("Couldn't delete cluster member: %s", err)
	}

	// Count should be 1
	count, err = database.CountClusterMembers(ctx)
	if err != nil {
		t.Fatalf("Couldn't count cluster members: %s", err)
	}

	if count != 1 {
		t.Fatalf("Expected count to be 1 after delete, got %d", count)
	}

	// Delete non-existent member should return ErrNotFound
	err = database.DeleteClusterMember(ctx, 999)
	if err != db.ErrNotFound {
		t.Fatalf("Expected ErrNotFound for delete of non-existent, got: %v", err)
	}

	// Clean up second member
	err = database.DeleteClusterMember(ctx, 2)
	if err != nil {
		t.Fatalf("Couldn't delete second cluster member: %s", err)
	}

	// List should be empty again
	members, err = database.ListClusterMembers(ctx)
	if err != nil {
		t.Fatalf("Couldn't list cluster members: %s", err)
	}

	if len(members) != 0 {
		t.Fatalf("Expected no members after cleanup, got %d", len(members))
	}
}
