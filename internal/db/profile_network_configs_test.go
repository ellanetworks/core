// Copyright 2026 Ella Networks

package db_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ellanetworks/core/internal/db"
)

func TestProfileNetworkConfigsEndToEnd(t *testing.T) {
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

	// Get the default profile and slice created during initialization.
	defaultProfile, err := database.GetProfile(context.Background(), "default")
	if err != nil {
		t.Fatalf("Couldn't get default profile: %s", err)
	}

	defaultSlice, err := database.GetNetworkSliceByName(context.Background(), "default")
	if err != nil {
		t.Fatalf("Couldn't get default slice: %s", err)
	}

	// DB initialization creates a default config linking the default profile,
	// slice, and data network.
	configs, err := database.ListProfileNetworkConfigs(context.Background(), defaultProfile.ID)
	if err != nil {
		t.Fatalf("Couldn't list profile network configs: %s", err)
	}

	if len(configs) != 1 {
		t.Fatalf("Expected 1 default config, got %d", len(configs))
	}

	defaultConfig := configs[0]
	if defaultConfig.ProfileID != defaultProfile.ID {
		t.Fatalf("Expected profileID=%d, got %d", defaultProfile.ID, defaultConfig.ProfileID)
	}

	if defaultConfig.SliceID != defaultSlice.ID {
		t.Fatalf("Expected sliceID=%d, got %d", defaultSlice.ID, defaultConfig.SliceID)
	}

	// Create a new data network and profile for testing.
	newDN := &db.DataNetwork{
		Name:   "test-dn",
		IPPool: "10.99.0.0/24",
		DNS:    "1.1.1.1",
		MTU:    1500,
	}

	err = database.CreateDataNetwork(context.Background(), newDN)
	if err != nil {
		t.Fatalf("Couldn't create data network: %s", err)
	}

	createdDN, err := database.GetDataNetwork(context.Background(), "test-dn")
	if err != nil {
		t.Fatalf("Couldn't get created data network: %s", err)
	}

	newProfile := &db.Profile{
		Name: "test-profile",
	}

	err = database.CreateProfile(context.Background(), newProfile)
	if err != nil {
		t.Fatalf("Couldn't create profile: %s", err)
	}

	createdProfile, err := database.GetProfile(context.Background(), "test-profile")
	if err != nil {
		t.Fatalf("Couldn't get created profile: %s", err)
	}

	// Create a new config.
	config := &db.ProfileNetworkConfig{
		ProfileID:           createdProfile.ID,
		SliceID:             defaultSlice.ID,
		DataNetworkID:       createdDN.ID,
		Var5qi:              9,
		Arp:                 1,
		SessionAmbrUplink:   "500 Mbps",
		SessionAmbrDownlink: "500 Mbps",
	}

	err = database.CreateProfileNetworkConfig(context.Background(), config)
	if err != nil {
		t.Fatalf("Couldn't create profile network config: %s", err)
	}

	// List configs for the new profile.
	configs, err = database.ListProfileNetworkConfigs(context.Background(), createdProfile.ID)
	if err != nil {
		t.Fatalf("Couldn't list configs for new profile: %s", err)
	}

	if len(configs) != 1 {
		t.Fatalf("Expected 1 config for new profile, got %d", len(configs))
	}

	if configs[0].Var5qi != 9 {
		t.Fatalf("Expected var5qi=9, got %d", configs[0].Var5qi)
	}

	if configs[0].Arp != 1 {
		t.Fatalf("Expected arp=1, got %d", configs[0].Arp)
	}

	if configs[0].SessionAmbrUplink != "500 Mbps" {
		t.Fatalf("Expected sessionAmbrUplink='500 Mbps', got %q", configs[0].SessionAmbrUplink)
	}

	// Get specific config.
	retrieved, err := database.GetProfileNetworkConfig(context.Background(), createdProfile.ID, defaultSlice.ID, createdDN.ID)
	if err != nil {
		t.Fatalf("Couldn't get specific config: %s", err)
	}

	if retrieved.ProfileID != createdProfile.ID {
		t.Fatalf("Expected profileID=%d, got %d", createdProfile.ID, retrieved.ProfileID)
	}

	// Update the config.
	retrieved.Var5qi = 8
	retrieved.Arp = 2
	retrieved.SessionAmbrUplink = "1 Gbps"
	retrieved.SessionAmbrDownlink = "1 Gbps"

	err = database.UpdateProfileNetworkConfig(context.Background(), retrieved)
	if err != nil {
		t.Fatalf("Couldn't update profile network config: %s", err)
	}

	updated, err := database.GetProfileNetworkConfig(context.Background(), createdProfile.ID, defaultSlice.ID, createdDN.ID)
	if err != nil {
		t.Fatalf("Couldn't get updated config: %s", err)
	}

	if updated.Var5qi != 8 {
		t.Fatalf("Expected var5qi=8 after update, got %d", updated.Var5qi)
	}

	if updated.Arp != 2 {
		t.Fatalf("Expected arp=2 after update, got %d", updated.Arp)
	}

	if updated.SessionAmbrUplink != "1 Gbps" {
		t.Fatalf("Expected sessionAmbrUplink='1 Gbps' after update, got %q", updated.SessionAmbrUplink)
	}

	// Delete the config.
	err = database.DeleteProfileNetworkConfig(context.Background(), createdProfile.ID, defaultSlice.ID, createdDN.ID)
	if err != nil {
		t.Fatalf("Couldn't delete profile network config: %s", err)
	}

	configs, err = database.ListProfileNetworkConfigs(context.Background(), createdProfile.ID)
	if err != nil {
		t.Fatalf("Couldn't list configs after delete: %s", err)
	}

	if len(configs) != 0 {
		t.Fatalf("Expected 0 configs after delete, got %d", len(configs))
	}
}

func TestNetworkConfigsInDataNetwork(t *testing.T) {
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

	// The default "internet" data network has the default config pointing to it.
	hasConfigs, err := database.NetworkConfigsInDataNetwork(context.Background(), "internet")
	if err != nil {
		t.Fatalf("Couldn't check NetworkConfigsInDataNetwork: %s", err)
	}

	if !hasConfigs {
		t.Fatalf("Expected default data network to have configs")
	}

	// Create a fresh data network with no configs.
	newDN := &db.DataNetwork{
		Name:   "empty-dn",
		IPPool: "10.88.0.0/24",
		DNS:    "1.1.1.1",
		MTU:    1500,
	}

	err = database.CreateDataNetwork(context.Background(), newDN)
	if err != nil {
		t.Fatalf("Couldn't create data network: %s", err)
	}

	hasConfigs, err = database.NetworkConfigsInDataNetwork(context.Background(), "empty-dn")
	if err != nil {
		t.Fatalf("Couldn't check NetworkConfigsInDataNetwork: %s", err)
	}

	if hasConfigs {
		t.Fatalf("Expected empty data network to have no configs")
	}
}

func TestGetSessionPolicy(t *testing.T) {
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

	// Create a subscriber using the default profile.
	defaultProfile, err := database.GetProfile(context.Background(), "default")
	if err != nil {
		t.Fatalf("Couldn't get default profile: %s", err)
	}

	subscriber := &db.Subscriber{
		Imsi:           "001010100007487",
		SequenceNumber: "000000000001",
		PermanentKey:   "6f30087629feb0b089783c81d0ae09b5",
		Opc:            "21a7e1897dfb481d62439142cdf1b6ee",
		ProfileID:      defaultProfile.ID,
	}

	err = database.CreateSubscriber(context.Background(), subscriber)
	if err != nil {
		t.Fatalf("Couldn't create subscriber: %s", err)
	}

	// The default slice has sst=1, sd="102030".
	sd := "102030"

	config, err := database.GetSessionPolicy(context.Background(), "001010100007487", 1, &sd, "internet")
	if err != nil {
		t.Fatalf("Couldn't get session policy: %s", err)
	}

	if config.Var5qi != 9 {
		t.Fatalf("Expected var5qi=9, got %d", config.Var5qi)
	}

	if config.Arp != 1 {
		t.Fatalf("Expected arp=1, got %d", config.Arp)
	}

	// Requesting a non-existent DNN should fail.
	_, err = database.GetSessionPolicy(context.Background(), "001010100007487", 1, &sd, "nonexistent")
	if err == nil {
		t.Fatalf("Expected error for non-existent DNN")
	}

	// Requesting a non-existent slice should fail.
	otherSD := "ffffff"

	_, err = database.GetSessionPolicy(context.Background(), "001010100007487", 99, &otherSD, "internet")
	if err == nil {
		t.Fatalf("Expected error for non-existent slice")
	}
}
