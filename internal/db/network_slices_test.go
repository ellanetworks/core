// Copyright 2024 Ella Networks

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

	// Default slice is created during initialization
	res, total, err := database.ListNetworkSlicesPage(context.Background(), 1, 10)
	if err != nil {
		t.Fatalf("Couldn't complete ListNetworkSlicesPage: %s", err)
	}

	if total != 1 {
		t.Fatalf("Expected 1 default slice, got %d", total)
	}

	if len(res) != 1 {
		t.Fatalf("Expected 1 slice in results, got %d", len(res))
	}

	if res[0].Name != "default" {
		t.Fatalf("Expected default slice name 'default', got %q", res[0].Name)
	}

	if res[0].Sst != 1 {
		t.Fatalf("Expected default slice SST 1, got %d", res[0].Sst)
	}

	// Create a new slice
	sd := "aabbcc"
	newSlice := &db.NetworkSlice{
		Name: "test-slice",
		Sst:  2,
		Sd:   &sd,
	}

	err = database.CreateNetworkSlice(context.Background(), newSlice)
	if err != nil {
		t.Fatalf("Couldn't complete CreateNetworkSlice: %s", err)
	}

	// List again — should have 2
	res, total, err = database.ListNetworkSlicesPage(context.Background(), 1, 10)
	if err != nil {
		t.Fatalf("Couldn't complete ListNetworkSlicesPage: %s", err)
	}

	if total != 2 {
		t.Fatalf("Expected 2 slices, got %d", total)
	}

	if len(res) != 2 {
		t.Fatalf("Expected 2 slices in results, got %d", len(res))
	}

	// Get by name
	retrieved, err := database.GetNetworkSlice(context.Background(), "test-slice")
	if err != nil {
		t.Fatalf("Couldn't complete GetNetworkSlice: %s", err)
	}

	if retrieved.Name != "test-slice" {
		t.Fatalf("Expected name 'test-slice', got %q", retrieved.Name)
	}

	if retrieved.Sst != 2 {
		t.Fatalf("Expected SST 2, got %d", retrieved.Sst)
	}

	if retrieved.Sd == nil {
		t.Fatal("Expected non-nil SD")
	}

	if *retrieved.Sd != "aabbcc" {
		t.Fatalf("Expected SD 'aabbcc', got %q", *retrieved.Sd)
	}

	// Get by ID
	retrievedByID, err := database.GetNetworkSliceByID(context.Background(), retrieved.ID)
	if err != nil {
		t.Fatalf("Couldn't complete GetNetworkSliceByID: %s", err)
	}

	if retrievedByID.Name != "test-slice" {
		t.Fatalf("Expected name 'test-slice', got %q", retrievedByID.Name)
	}

	// Update
	newSd := "112233"
	updatedSlice := &db.NetworkSlice{
		Name: "test-slice",
		Sst:  3,
		Sd:   &newSd,
	}

	err = database.UpdateNetworkSlice(context.Background(), updatedSlice)
	if err != nil {
		t.Fatalf("Couldn't complete UpdateNetworkSlice: %s", err)
	}

	retrieved, err = database.GetNetworkSlice(context.Background(), "test-slice")
	if err != nil {
		t.Fatalf("Couldn't complete GetNetworkSlice after update: %s", err)
	}

	if retrieved.Sst != 3 {
		t.Fatalf("Expected SST 3 after update, got %d", retrieved.Sst)
	}

	if *retrieved.Sd != "112233" {
		t.Fatalf("Expected SD '112233' after update, got %q", *retrieved.Sd)
	}

	// Count
	count, err := database.CountNetworkSlices(context.Background())
	if err != nil {
		t.Fatalf("Couldn't complete CountNetworkSlices: %s", err)
	}

	if count != 2 {
		t.Fatalf("Expected count 2, got %d", count)
	}

	// Delete
	err = database.DeleteNetworkSlice(context.Background(), "test-slice")
	if err != nil {
		t.Fatalf("Couldn't complete DeleteNetworkSlice: %s", err)
	}

	count, err = database.CountNetworkSlices(context.Background())
	if err != nil {
		t.Fatalf("Couldn't complete CountNetworkSlices after delete: %s", err)
	}

	if count != 1 {
		t.Fatalf("Expected count 1 after delete, got %d", count)
	}
}

func TestNetworkSliceGetNotFound(t *testing.T) {
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

	_, err = database.GetNetworkSlice(context.Background(), "nonexistent")
	if err != db.ErrNotFound {
		t.Fatalf("Expected ErrNotFound, got %v", err)
	}
}

func TestNetworkSliceDuplicateName(t *testing.T) {
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

	slice := &db.NetworkSlice{
		Name: "duplicate-slice",
		Sst:  1,
	}

	err = database.CreateNetworkSlice(context.Background(), slice)
	if err != nil {
		t.Fatalf("Couldn't complete first CreateNetworkSlice: %s", err)
	}

	err = database.CreateNetworkSlice(context.Background(), slice)
	if err != db.ErrAlreadyExists {
		t.Fatalf("Expected ErrAlreadyExists, got %v", err)
	}
}

func TestNetworkSliceNilSD(t *testing.T) {
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

	slice := &db.NetworkSlice{
		Name: "no-sd-slice",
		Sst:  1,
		Sd:   nil,
	}

	err = database.CreateNetworkSlice(context.Background(), slice)
	if err != nil {
		t.Fatalf("Couldn't complete CreateNetworkSlice: %s", err)
	}

	retrieved, err := database.GetNetworkSlice(context.Background(), "no-sd-slice")
	if err != nil {
		t.Fatalf("Couldn't complete GetNetworkSlice: %s", err)
	}

	if retrieved.Sd != nil {
		t.Fatalf("Expected nil SD, got %q", *retrieved.Sd)
	}
}

func TestListNetworkSlicesByIDs(t *testing.T) {
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

	// Get the default slice ID
	defaultSlice, err := database.GetNetworkSlice(context.Background(), "default")
	if err != nil {
		t.Fatalf("Couldn't get default slice: %s", err)
	}

	// Create two more slices
	sd1 := "aabbcc"

	err = database.CreateNetworkSlice(context.Background(), &db.NetworkSlice{Name: "slice-a", Sst: 2, Sd: &sd1})
	if err != nil {
		t.Fatalf("Couldn't create slice-a: %s", err)
	}

	sliceA, err := database.GetNetworkSlice(context.Background(), "slice-a")
	if err != nil {
		t.Fatalf("Couldn't get slice-a: %s", err)
	}

	sd2 := "112233"

	err = database.CreateNetworkSlice(context.Background(), &db.NetworkSlice{Name: "slice-b", Sst: 3, Sd: &sd2})
	if err != nil {
		t.Fatalf("Couldn't create slice-b: %s", err)
	}

	sliceB, err := database.GetNetworkSlice(context.Background(), "slice-b")
	if err != nil {
		t.Fatalf("Couldn't get slice-b: %s", err)
	}

	// Fetch subset of IDs
	slices, err := database.ListNetworkSlicesByIDs(context.Background(), []int{defaultSlice.ID, sliceB.ID})
	if err != nil {
		t.Fatalf("Couldn't complete ListNetworkSlicesByIDs: %s", err)
	}

	if len(slices) != 2 {
		t.Fatalf("Expected 2 slices, got %d", len(slices))
	}

	foundIDs := map[int]bool{}
	for _, s := range slices {
		foundIDs[s.ID] = true
	}

	if !foundIDs[defaultSlice.ID] || !foundIDs[sliceB.ID] {
		t.Fatalf("Expected IDs %d and %d, got %v", defaultSlice.ID, sliceB.ID, foundIDs)
	}

	// Fetch all three
	slices, err = database.ListNetworkSlicesByIDs(context.Background(), []int{defaultSlice.ID, sliceA.ID, sliceB.ID})
	if err != nil {
		t.Fatalf("Couldn't complete ListNetworkSlicesByIDs: %s", err)
	}

	if len(slices) != 3 {
		t.Fatalf("Expected 3 slices, got %d", len(slices))
	}

	// Empty IDs returns nil
	slices, err = database.ListNetworkSlicesByIDs(context.Background(), []int{})
	if err != nil {
		t.Fatalf("Couldn't complete ListNetworkSlicesByIDs with empty IDs: %s", err)
	}

	if slices != nil {
		t.Fatalf("Expected nil for empty IDs, got %d slices", len(slices))
	}

	// Non-existent IDs return empty
	slices, err = database.ListNetworkSlicesByIDs(context.Background(), []int{9999})
	if err != nil {
		t.Fatalf("Couldn't complete ListNetworkSlicesByIDs with non-existent ID: %s", err)
	}

	if len(slices) != 0 {
		t.Fatalf("Expected 0 slices for non-existent ID, got %d", len(slices))
	}
}

func TestListAllNetworkSlices(t *testing.T) {
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

	// Default slice exists
	slices, err := database.ListAllNetworkSlices(context.Background())
	if err != nil {
		t.Fatalf("Couldn't complete ListAllNetworkSlices: %s", err)
	}

	if len(slices) != 1 {
		t.Fatalf("Expected 1 slice, got %d", len(slices))
	}

	// Add another
	err = database.CreateNetworkSlice(context.Background(), &db.NetworkSlice{
		Name: "extra-slice",
		Sst:  2,
	})
	if err != nil {
		t.Fatalf("Couldn't complete CreateNetworkSlice: %s", err)
	}

	slices, err = database.ListAllNetworkSlices(context.Background())
	if err != nil {
		t.Fatalf("Couldn't complete ListAllNetworkSlices: %s", err)
	}

	if len(slices) != 2 {
		t.Fatalf("Expected 2 slices, got %d", len(slices))
	}
}
