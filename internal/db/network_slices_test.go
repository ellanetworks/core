// Copyright 2026 Ella Networks

package db_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ellanetworks/core/internal/db"
)

func TestNetworkSlicesEndToEnd(t *testing.T) {
	tempDir := t.TempDir()

	database, err := db.NewDatabase(context.Background(), filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	}()

	// DB initialization creates a default "default" network slice (sst=1, sd=nil).
	slices, err := database.ListNetworkSlices(context.Background())
	if err != nil {
		t.Fatalf("Couldn't list network slices: %s", err)
	}

	if len(slices) != 1 {
		t.Fatalf("Expected 1 default slice, got %d", len(slices))
	}

	defaultSlice := slices[0]
	if defaultSlice.Sst != 1 {
		t.Fatalf("Expected default slice sst=1, got %d", defaultSlice.Sst)
	}

	if defaultSlice.Sd == nil || *defaultSlice.Sd != "102030" {
		t.Fatalf("Expected default slice sd='102030', got %v", defaultSlice.Sd)
	}

	// Create a new slice with SD.
	sd := "010203"
	newSlice := &db.NetworkSlice{
		Sst:  2,
		Sd:   &sd,
		Name: "test-slice",
	}

	err = database.CreateNetworkSlice(context.Background(), newSlice)
	if err != nil {
		t.Fatalf("Couldn't create network slice: %s", err)
	}

	slices, err = database.ListNetworkSlices(context.Background())
	if err != nil {
		t.Fatalf("Couldn't list network slices: %s", err)
	}

	if len(slices) != 2 {
		t.Fatalf("Expected 2 slices, got %d", len(slices))
	}

	// Retrieve by SST/SD.
	retrieved, err := database.GetNetworkSliceBySstSd(context.Background(), 2, &sd)
	if err != nil {
		t.Fatalf("Couldn't get network slice by sst/sd: %s", err)
	}

	if retrieved.Sst != 2 {
		t.Fatalf("Expected sst=2, got %d", retrieved.Sst)
	}

	if retrieved.Sd == nil || *retrieved.Sd != sd {
		t.Fatalf("Expected sd=%q, got %v", sd, retrieved.Sd)
	}

	if retrieved.Name != "test-slice" {
		t.Fatalf("Expected name 'test-slice', got %q", retrieved.Name)
	}

	// Retrieve the default slice by SST/SD.
	defaultSD := "102030"

	defaultRetrieved, err := database.GetNetworkSliceBySstSd(context.Background(), 1, &defaultSD)
	if err != nil {
		t.Fatalf("Couldn't get default slice by sst/sd: %s", err)
	}

	if defaultRetrieved.Sst != 1 {
		t.Fatalf("Expected sst=1, got %d", defaultRetrieved.Sst)
	}

	// Retrieve by SST with null SD should return ErrNotFound for the default.
	_, err = database.GetNetworkSliceBySstSd(context.Background(), 1, nil)
	if err == nil {
		t.Fatalf("Expected error when looking up sst=1 with nil sd (default has sd='102030')")
	}

	// Retrieve by ID.
	byID, err := database.GetNetworkSliceByID(context.Background(), retrieved.ID)
	if err != nil {
		t.Fatalf("Couldn't get network slice by ID: %s", err)
	}

	if byID.Name != "test-slice" {
		t.Fatalf("Expected name 'test-slice', got %q", byID.Name)
	}

	// Retrieve by name.
	byName, err := database.GetNetworkSliceByName(context.Background(), "test-slice")
	if err != nil {
		t.Fatalf("Couldn't get network slice by name: %s", err)
	}

	if byName.ID != retrieved.ID {
		t.Fatalf("Expected ID %d, got %d", retrieved.ID, byName.ID)
	}

	// Update the slice.
	newSD := "aabbcc"
	retrieved.Sst = 3
	retrieved.Sd = &newSD
	retrieved.Name = "updated-slice"

	err = database.UpdateNetworkSlice(context.Background(), retrieved)
	if err != nil {
		t.Fatalf("Couldn't update network slice: %s", err)
	}

	updated, err := database.GetNetworkSliceByID(context.Background(), retrieved.ID)
	if err != nil {
		t.Fatalf("Couldn't get updated network slice: %s", err)
	}

	if updated.Sst != 3 {
		t.Fatalf("Expected sst=3 after update, got %d", updated.Sst)
	}

	if updated.Sd == nil || *updated.Sd != newSD {
		t.Fatalf("Expected sd=%q after update, got %v", newSD, updated.Sd)
	}

	if updated.Name != "updated-slice" {
		t.Fatalf("Expected name 'updated-slice', got %q", updated.Name)
	}

	// Count.
	count, err := database.CountNetworkSlices(context.Background())
	if err != nil {
		t.Fatalf("Couldn't count network slices: %s", err)
	}

	if count != 2 {
		t.Fatalf("Expected count=2, got %d", count)
	}

	// Delete the created slice.
	err = database.DeleteNetworkSlice(context.Background(), retrieved.ID)
	if err != nil {
		t.Fatalf("Couldn't delete network slice: %s", err)
	}

	count, err = database.CountNetworkSlices(context.Background())
	if err != nil {
		t.Fatalf("Couldn't count network slices after delete: %s", err)
	}

	if count != 1 {
		t.Fatalf("Expected count=1 after delete, got %d", count)
	}

	// Delete non-existent slice returns ErrNotFound.
	err = database.DeleteNetworkSlice(context.Background(), 9999)
	if err == nil {
		t.Fatalf("Expected error when deleting non-existent slice")
	}
}
