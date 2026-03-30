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

	database, err := db.NewDatabase(context.Background(), filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}

	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	}()

	res, total, err := database.ListProfilesPage(context.Background(), 1, 10)
	if err != nil {
		t.Fatalf("Couldn't complete RetrieveAll: %s", err)
	}

	if total != 1 {
		t.Fatalf("Default profile wasn't found in DB")
	}

	if len(res) != 1 {
		t.Fatalf("More than one profiles were found in DB")
	}

	profile := &db.Profile{
		Name:           "my-profile",
		UeAmbrUplink:   "100 Mbps",
		UeAmbrDownlink: "200 Mbps",
	}

	err = database.CreateProfile(context.Background(), profile)
	if err != nil {
		t.Fatalf("Couldn't complete Create: %s", err)
	}

	res, total, err = database.ListProfilesPage(context.Background(), 1, 10)
	if err != nil {
		t.Fatalf("Couldn't complete RetrieveAll: %s", err)
	}

	if total != 2 {
		t.Fatalf("Not all profiles were found in DB")
	}

	if len(res) != 2 {
		t.Fatalf("One or more profiles weren't found in DB")
	}

	retrievedProfile, err := database.GetProfile(context.Background(), profile.Name)
	if err != nil {
		t.Fatalf("Couldn't complete Retrieve: %s", err)
	}

	if retrievedProfile.Name != profile.Name {
		t.Fatalf("The profile name from the database doesn't match the profile name that was given")
	}

	if retrievedProfile.UeAmbrUplink != profile.UeAmbrUplink {
		t.Fatalf("The UeAmbrUplink from the database doesn't match the value that was given")
	}

	if retrievedProfile.UeAmbrDownlink != profile.UeAmbrDownlink {
		t.Fatalf("The UeAmbrDownlink from the database doesn't match the value that was given")
	}

	// Edit the profile
	profile.UeAmbrUplink = "150 Mbps"
	profile.UeAmbrDownlink = "300 Mbps"

	if err = database.UpdateProfile(context.Background(), profile); err != nil {
		t.Fatalf("Couldn't complete Update: %s", err)
	}

	retrievedProfile, err = database.GetProfile(context.Background(), profile.Name)
	if err != nil {
		t.Fatalf("Couldn't complete Retrieve: %s", err)
	}

	if retrievedProfile.Name != profile.Name {
		t.Fatalf("The profile name from the database doesn't match the profile name that was given")
	}

	if retrievedProfile.UeAmbrUplink != profile.UeAmbrUplink {
		t.Fatalf("The UeAmbrUplink from the database doesn't match the value that was given")
	}

	if retrievedProfile.UeAmbrDownlink != profile.UeAmbrDownlink {
		t.Fatalf("The UeAmbrDownlink from the database doesn't match the value that was given")
	}

	if err = database.DeleteProfile(context.Background(), profile.Name); err != nil {
		t.Fatalf("Couldn't complete Delete: %s", err)
	}

	res, total, _ = database.ListProfilesPage(context.Background(), 1, 10)
	if total != 1 {
		t.Fatalf("Profile wasn't deleted from the DB properly")
	}

	if len(res) != 1 {
		t.Fatalf("Profile wasn't deleted from the DB properly")
	}
}
