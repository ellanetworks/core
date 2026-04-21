// Copyright 2024 Ella Networks

package db_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ellanetworks/core/internal/db"
)

func TestProfilesEndToEnd(t *testing.T) {
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

	// Default profile is created during initialization
	res, total, err := database.ListProfilesPage(context.Background(), 1, 10)
	if err != nil {
		t.Fatalf("Couldn't complete ListProfilesPage: %s", err)
	}

	if total != 1 {
		t.Fatalf("Expected 1 default profile, got %d", total)
	}

	if len(res) != 1 {
		t.Fatalf("Expected 1 profile in results, got %d", len(res))
	}

	if res[0].Name != "default" {
		t.Fatalf("Expected default profile name 'default', got %q", res[0].Name)
	}

	if res[0].UeAmbrUplink != "200 Mbps" {
		t.Fatalf("Expected default UeAmbrUplink '200 Mbps', got %q", res[0].UeAmbrUplink)
	}

	if res[0].UeAmbrDownlink != "200 Mbps" {
		t.Fatalf("Expected default UeAmbrDownlink '200 Mbps', got %q", res[0].UeAmbrDownlink)
	}

	// Create a new profile
	newProfile := &db.Profile{
		Name:           "test-profile",
		UeAmbrUplink:   "500 Mbps",
		UeAmbrDownlink: "1 Gbps",
	}

	err = database.CreateProfile(context.Background(), newProfile)
	if err != nil {
		t.Fatalf("Couldn't complete CreateProfile: %s", err)
	}

	// List again — should have 2
	res, total, err = database.ListProfilesPage(context.Background(), 1, 10)
	if err != nil {
		t.Fatalf("Couldn't complete ListProfilesPage: %s", err)
	}

	if total != 2 {
		t.Fatalf("Expected 2 profiles, got %d", total)
	}

	if len(res) != 2 {
		t.Fatalf("Expected 2 profiles in results, got %d", len(res))
	}

	// Get by name
	retrieved, err := database.GetProfile(context.Background(), "test-profile")
	if err != nil {
		t.Fatalf("Couldn't complete GetProfile: %s", err)
	}

	if retrieved.Name != "test-profile" {
		t.Fatalf("Expected name 'test-profile', got %q", retrieved.Name)
	}

	if retrieved.UeAmbrUplink != "500 Mbps" {
		t.Fatalf("Expected UeAmbrUplink '500 Mbps', got %q", retrieved.UeAmbrUplink)
	}

	if retrieved.UeAmbrDownlink != "1 Gbps" {
		t.Fatalf("Expected UeAmbrDownlink '1 Gbps', got %q", retrieved.UeAmbrDownlink)
	}

	// Get by ID
	retrievedByID, err := database.GetProfileByID(context.Background(), retrieved.ID)
	if err != nil {
		t.Fatalf("Couldn't complete GetProfileByID: %s", err)
	}

	if retrievedByID.Name != "test-profile" {
		t.Fatalf("Expected name 'test-profile', got %q", retrievedByID.Name)
	}

	// Update
	updatedProfile := &db.Profile{
		Name:           "test-profile",
		UeAmbrUplink:   "1 Gbps",
		UeAmbrDownlink: "2 Gbps",
	}

	err = database.UpdateProfile(context.Background(), updatedProfile)
	if err != nil {
		t.Fatalf("Couldn't complete UpdateProfile: %s", err)
	}

	retrieved, err = database.GetProfile(context.Background(), "test-profile")
	if err != nil {
		t.Fatalf("Couldn't complete GetProfile after update: %s", err)
	}

	if retrieved.UeAmbrUplink != "1 Gbps" {
		t.Fatalf("Expected UeAmbrUplink '1 Gbps' after update, got %q", retrieved.UeAmbrUplink)
	}

	if retrieved.UeAmbrDownlink != "2 Gbps" {
		t.Fatalf("Expected UeAmbrDownlink '2 Gbps' after update, got %q", retrieved.UeAmbrDownlink)
	}

	// Count
	count, err := database.CountProfiles(context.Background())
	if err != nil {
		t.Fatalf("Couldn't complete CountProfiles: %s", err)
	}

	if count != 2 {
		t.Fatalf("Expected count 2, got %d", count)
	}

	// Delete
	err = database.DeleteProfile(context.Background(), "test-profile")
	if err != nil {
		t.Fatalf("Couldn't complete DeleteProfile: %s", err)
	}

	count, err = database.CountProfiles(context.Background())
	if err != nil {
		t.Fatalf("Couldn't complete CountProfiles after delete: %s", err)
	}

	if count != 1 {
		t.Fatalf("Expected count 1 after delete, got %d", count)
	}
}

func TestProfileGetNotFound(t *testing.T) {
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

	_, err = database.GetProfile(context.Background(), "nonexistent")
	if err != db.ErrNotFound {
		t.Fatalf("Expected ErrNotFound, got %v", err)
	}
}

func TestProfileDuplicateName(t *testing.T) {
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

	profile := &db.Profile{
		Name:           "duplicate-profile",
		UeAmbrUplink:   "100 Mbps",
		UeAmbrDownlink: "100 Mbps",
	}

	err = database.CreateProfile(context.Background(), profile)
	if err != nil {
		t.Fatalf("Couldn't complete first CreateProfile: %s", err)
	}

	err = database.CreateProfile(context.Background(), profile)
	if err != db.ErrAlreadyExists {
		t.Fatalf("Expected ErrAlreadyExists, got %v", err)
	}
}

func TestProfileSubscriberCount(t *testing.T) {
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

	profile := &db.Profile{
		Name:           "count-profile",
		UeAmbrUplink:   "100 Mbps",
		UeAmbrDownlink: "100 Mbps",
	}

	err = database.CreateProfile(context.Background(), profile)
	if err != nil {
		t.Fatalf("Couldn't complete CreateProfile: %s", err)
	}

	created, err := database.GetProfile(context.Background(), "count-profile")
	if err != nil {
		t.Fatalf("Couldn't complete GetProfile: %s", err)
	}

	count, err := database.CountSubscribersInProfile(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("Couldn't complete CountSubscribersInProfile: %s", err)
	}

	if count != 0 {
		t.Fatalf("Expected 0 subscribers, got %d", count)
	}

	hasSubscribers, err := database.SubscribersInProfile(context.Background(), "count-profile")
	if err != nil {
		t.Fatalf("Couldn't complete SubscribersInProfile: %s", err)
	}

	if hasSubscribers {
		t.Fatal("Expected no subscribers in profile")
	}

	// Add a subscriber
	subscriber := &db.Subscriber{
		Imsi:           "001010100007487",
		SequenceNumber: "000000000001",
		PermanentKey:   "6f30087629feb0b089783c81d0ae09b5",
		Opc:            "21a7e1897dfb481d62439142cdf1b6ee",
		ProfileID:      created.ID,
	}

	err = database.CreateSubscriber(context.Background(), subscriber)
	if err != nil {
		t.Fatalf("Couldn't complete CreateSubscriber: %s", err)
	}

	count, err = database.CountSubscribersInProfile(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("Couldn't complete CountSubscribersInProfile: %s", err)
	}

	if count != 1 {
		t.Fatalf("Expected 1 subscriber, got %d", count)
	}

	hasSubscribers, err = database.SubscribersInProfile(context.Background(), "count-profile")
	if err != nil {
		t.Fatalf("Couldn't complete SubscribersInProfile: %s", err)
	}

	if !hasSubscribers {
		t.Fatal("Expected subscribers in profile")
	}
}
