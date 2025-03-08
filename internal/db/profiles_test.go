// Copyright 2024 Ella Networks

package db_test

import (
	"path/filepath"
	"testing"

	"github.com/ellanetworks/core/internal/db"
)

func TestProfilesEndToEnd(t *testing.T) {
	tempDir := t.TempDir()
	database, err := db.NewDatabase(filepath.Join(tempDir, "db.sqlite3"), initialOperator)
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	}()

	res, err := database.ListProfiles()
	if err != nil {
		t.Fatalf("Couldn't complete RetrieveAll: %s", err)
	}

	if len(res) != 0 {
		t.Fatalf("One or more profiles were found in DB")
	}

	profile := &db.Profile{
		Name:            "my-profile",
		UeIPPool:        "0.0.0.0/24",
		DNS:             "8.8.8.8",
		Mtu:             1500,
		BitrateUplink:   "100 Mbps",
		BitrateDownlink: "200 Mbps",
		Var5qi:          9,
		PriorityLevel:   1,
	}
	err = database.CreateProfile(profile)
	if err != nil {
		t.Fatalf("Couldn't complete Create: %s", err)
	}

	res, err = database.ListProfiles()
	if err != nil {
		t.Fatalf("Couldn't complete RetrieveAll: %s", err)
	}
	if len(res) != 1 {
		t.Fatalf("One or more profiles weren't found in DB")
	}

	retrievedProfile, err := database.GetProfile(profile.Name)
	if err != nil {
		t.Fatalf("Couldn't complete Retrieve: %s", err)
	}
	if retrievedProfile.Name != profile.Name {
		t.Fatalf("The profile name from the database doesn't match the profile name that was given")
	}
	if retrievedProfile.UeIPPool != profile.UeIPPool {
		t.Fatalf("The ue ip pool from the database doesn't match the ue ip pool that was given")
	}
	if retrievedProfile.DNS != profile.DNS {
		t.Fatalf("The dns from the database doesn't match the dns that was given")
	}
	if retrievedProfile.Mtu != profile.Mtu {
		t.Fatalf("The mtu from the database doesn't match the mtu that was given")
	}
	if retrievedProfile.BitrateUplink != profile.BitrateUplink {
		t.Fatalf("The bitrate uplink from the database doesn't match the bitrate uplink that was given")
	}
	if retrievedProfile.BitrateDownlink != profile.BitrateDownlink {
		t.Fatalf("The bitrate downlink from the database doesn't match the bitrate downlink that was given")
	}
	if retrievedProfile.Var5qi != profile.Var5qi {
		t.Fatalf("The Var5qi from the database doesn't match the Var5qi that was given")
	}
	if retrievedProfile.PriorityLevel != profile.PriorityLevel {
		t.Fatalf("The priority level from the database doesn't match the priority level that was given")
	}

	// Edit the profile
	profile.UeIPPool = "1.1.1.0/24"
	profile.DNS = "2.2.2.2"

	if err = database.UpdateProfile(profile); err != nil {
		t.Fatalf("Couldn't complete Update: %s", err)
	}

	retrievedProfile, err = database.GetProfile(profile.Name)
	if err != nil {
		t.Fatalf("Couldn't complete Retrieve: %s", err)
	}
	if retrievedProfile.Name != profile.Name {
		t.Fatalf("The profile name from the database doesn't match the profile name that was given")
	}
	if retrievedProfile.Mtu != profile.Mtu {
		t.Fatalf("The mtu from the database doesn't match the mtu that was given")
	}
	if retrievedProfile.UeIPPool != profile.UeIPPool {
		t.Fatalf("The ue ip pool from the database doesn't match the ue ip pool that was given")
	}
	if retrievedProfile.DNS != profile.DNS {
		t.Fatalf("The dns from the database doesn't match the dns that was given")
	}

	if err = database.DeleteProfile(profile.Name); err != nil {
		t.Fatalf("Couldn't complete Delete: %s", err)
	}
	res, _ = database.ListProfiles()
	if len(res) != 0 {
		t.Fatalf("Profiles weren't deleted from the DB properly")
	}
}
