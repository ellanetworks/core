package db_test

import (
	"path/filepath"
	"testing"

	"github.com/yeastengine/ella/internal/db"
)

func TestProfilesEndToEnd(t *testing.T) {
	tempDir := t.TempDir()
	database, err := db.NewDatabase(filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}
	defer database.Close()

	res, err := database.ListProfiles()
	if err != nil {
		t.Fatalf("Couldn't complete RetrieveAll: %s", err)
	}

	if len(res) != 0 {
		t.Fatalf("One or more profiles were found in DB")
	}

	profile := &db.Profile{
		Name:            "my-profile",
		UeIpPool:        "0.0.0.0/24",
		DnsPrimary:      "8.8.8.8",
		DnsSecondary:    "9.9.9.9",
		Mtu:             1500,
		BitrateUplink:   1000000,
		BitrateDownlink: 2000000,
		BitrateUnit:     "bps",
		Qci:             9,
		Arp:             1,
		Pdb:             1,
		Pelr:            1,
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
	if retrievedProfile.UeIpPool != profile.UeIpPool {
		t.Fatalf("The ue ip pool from the database doesn't match the ue ip pool that was given")
	}
	if retrievedProfile.DnsPrimary != profile.DnsPrimary {
		t.Fatalf("The dns primary from the database doesn't match the dns primary that was given")
	}
	if retrievedProfile.DnsSecondary != profile.DnsSecondary {
		t.Fatalf("The dns secondary from the database doesn't match the dns secondary that was given")
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
	if retrievedProfile.BitrateUnit != profile.BitrateUnit {
		t.Fatalf("The bitrate unit from the database doesn't match the bitrate unit that was given")
	}
	if retrievedProfile.Qci != profile.Qci {
		t.Fatalf("The qci from the database doesn't match the qci that was given")
	}
	if retrievedProfile.Arp != profile.Arp {
		t.Fatalf("The arp from the database doesn't match the arp that was given")
	}
	if retrievedProfile.Pdb != profile.Pdb {
		t.Fatalf("The pdb from the database doesn't match the pdb that was given")
	}
	if retrievedProfile.Pelr != profile.Pelr {
		t.Fatalf("The pelr from the database doesn't match the pelr that was given")
	}

	// Edit the profile
	profile.UeIpPool = "1.1.1.0/24"
	profile.DnsPrimary = "2.2.2.2"

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
	if retrievedProfile.UeIpPool != profile.UeIpPool {
		t.Fatalf("The ue ip pool from the database doesn't match the ue ip pool that was given")
	}
	if retrievedProfile.DnsPrimary != profile.DnsPrimary {
		t.Fatalf("The dns primary from the database doesn't match the dns primary that was given")
	}

	if err = database.DeleteProfile(profile.Name); err != nil {
		t.Fatalf("Couldn't complete Delete: %s", err)
	}
	res, _ = database.ListProfiles()
	if len(res) != 0 {
		t.Fatalf("Profiles weren't deleted from the DB properly")
	}
}
