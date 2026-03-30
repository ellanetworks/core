// Copyright 2024 Ella Networks

package db_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ellanetworks/core/internal/db"
)

func createDataNetworkAndProfile(database *db.Database) (int, error) {
	newDataNetwork := &db.DataNetwork{
		Name:   "not-internet",
		IPPool: "1.2.3.0/24",
	}

	err := database.CreateDataNetwork(context.Background(), newDataNetwork)
	if err != nil {
		return 0, err
	}

	profile := &db.Profile{
		Name:           "my-profile",
		UeAmbrUplink:   "100 Mbps",
		UeAmbrDownlink: "200 Mbps",
	}

	err = database.CreateProfile(context.Background(), profile)
	if err != nil {
		return 0, err
	}

	profileCreated, err := database.GetProfile(context.Background(), profile.Name)
	if err != nil {
		return 0, err
	}

	return profileCreated.ID, nil
}

func TestSubscribersDbEndToEnd(t *testing.T) {
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

	res, total, err := database.ListSubscribersPage(context.Background(), 1, 10)
	if err != nil {
		t.Fatalf("Couldn't complete RetrieveAll: %s", err)
	}

	if total != 0 {
		t.Fatalf("Expected total count to be 0, but got %d", total)
	}

	if len(res) != 0 {
		t.Fatalf("One or more subscribers were found in DB")
	}

	profileID, err := createDataNetworkAndProfile(database)
	if err != nil {
		t.Fatalf("Couldn't create data network and profile: %s", err)
	}

	subscriber := &db.Subscriber{
		Imsi:           "001010100007487",
		SequenceNumber: "000000000001",
		PermanentKey:   "6f30087629feb0b089783c81d0ae09b5",
		Opc:            "21a7e1897dfb481d62439142cdf1b6ee",
		ProfileID:      profileID,
	}

	err = database.CreateSubscriber(context.Background(), subscriber)
	if err != nil {
		t.Fatalf("Couldn't complete Create: %s", err)
	}

	res, total, err = database.ListSubscribersPage(context.Background(), 1, 10)
	if err != nil {
		t.Fatalf("Couldn't complete RetrieveAll: %s", err)
	}

	if total != 1 {
		t.Fatalf("Expected total count to be 1, but got %d", total)
	}

	if len(res) != 1 {
		t.Fatalf("One or more subscribers weren't found in DB")
	}

	retrievedSubscriber, err := database.GetSubscriber(context.Background(), subscriber.Imsi)
	if err != nil {
		t.Fatalf("Couldn't complete Retrieve: %s", err)
	}

	if retrievedSubscriber.Imsi != subscriber.Imsi {
		t.Fatalf("The subscriber from the database doesn't match the subscriber that was given")
	}

	if retrievedSubscriber.SequenceNumber != subscriber.SequenceNumber {
		t.Fatalf("The sequence number from the database doesn't match the sequence number that was given")
	}

	if retrievedSubscriber.PermanentKey != subscriber.PermanentKey {
		t.Fatalf("The permanent key value from the database doesn't match the permanent key value that was given")
	}

	if retrievedSubscriber.Opc != subscriber.Opc {
		t.Fatalf("The OPC value from the database doesn't match the OPC value that was given")
	}

	newProfile := db.Profile{
		Name:           "another-profile",
		UeAmbrUplink:   "50 Mbps",
		UeAmbrDownlink: "50 Mbps",
	}

	err = database.CreateProfile(context.Background(), &newProfile)
	if err != nil {
		t.Fatalf("Couldn't complete Create: %s", err)
	}

	newProfileCreated, err := database.GetProfile(context.Background(), newProfile.Name)
	if err != nil {
		t.Fatalf("Couldn't complete Retrieve: %s", err)
	}

	subscriber.ProfileID = newProfileCreated.ID
	if err = database.UpdateSubscriberProfile(context.Background(), subscriber); err != nil {
		t.Fatalf("Couldn't complete Update: %s", err)
	}

	retrievedSubscriber, err = database.GetSubscriber(context.Background(), subscriber.Imsi)
	if err != nil {
		t.Fatalf("Couldn't complete Retrieve: %s", err)
	}

	if retrievedSubscriber.ProfileID != newProfileCreated.ID {
		t.Fatalf("Profile IDs don't match: %d", retrievedSubscriber.ProfileID)
	}

	if err = database.DeleteSubscriber(context.Background(), subscriber.Imsi); err != nil {
		t.Fatalf("Couldn't complete Delete: %s", err)
	}

	res, total, _ = database.ListSubscribersPage(context.Background(), 1, 10)

	if total != 0 {
		t.Fatalf("Expected total count to be 0, but got %d", total)
	}

	if len(res) != 0 {
		t.Fatalf("Subscribers weren't deleted from the DB properly")
	}
}

func TestCountSubscribersInProfile(t *testing.T) {
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

	profileID, err := createDataNetworkAndProfile(database)
	if err != nil {
		t.Fatalf("Couldn't create data network and profile: %s", err)
	}

	count, err := database.CountSubscribersInProfile(context.Background(), profileID)
	if err != nil {
		t.Fatalf("Couldn't complete CountSubscribersInProfile: %s", err)
	}

	if count != 0 {
		t.Fatalf("Expected 0 subscribers in profile, but got %d", count)
	}

	subscriber1 := &db.Subscriber{
		Imsi:           "001010100007487",
		SequenceNumber: "000000000001",
		PermanentKey:   "e08f6711b5319a21d550787cd263ee0a",
		Opc:            "21a7e1897dfb481d62439142cdf1b6ee",
		ProfileID:      profileID,
	}

	err = database.CreateSubscriber(context.Background(), subscriber1)
	if err != nil {
		t.Fatalf("Couldn't complete CreateSubscriber: %s", err)
	}

	newProfile := &db.Profile{
		Name:           "another-profile",
		UeAmbrUplink:   "50 Mbps",
		UeAmbrDownlink: "50 Mbps",
	}

	err = database.CreateProfile(context.Background(), newProfile)
	if err != nil {
		t.Fatalf("Couldn't Create Profile: %s", err)
	}

	newProfileCreated, err := database.GetProfile(context.Background(), newProfile.Name)
	if err != nil {
		t.Fatalf("Couldn't Retrieve Profile: %s", err)
	}

	subscriber2 := &db.Subscriber{
		Imsi:           "001010100007488",
		SequenceNumber: "000000000001",
		PermanentKey:   "6f30087629feb0b089783c81d0ae09b5",
		Opc:            "21a7e1897dfb481d62439142cdf1b6ee",
		ProfileID:      newProfileCreated.ID,
	}

	err = database.CreateSubscriber(context.Background(), subscriber2)
	if err != nil {
		t.Fatalf("Couldn't Create Subscriber: %s", err)
	}

	count, err = database.CountSubscribersInProfile(context.Background(), profileID)
	if err != nil {
		t.Fatalf("Couldn't complete CountSubscribersInProfile: %s", err)
	}

	if count != 1 {
		t.Fatalf("Expected 1 subscriber in profile, but got %d", count)
	}

	subscriber3 := &db.Subscriber{
		Imsi:           "001010100007489",
		SequenceNumber: "000000000001",
		PermanentKey:   "6f30087629feb0b089783c81d0ae09b5",
		Opc:            "21a7e1897dfb481d62439142cdf1b6ee",
		ProfileID:      profileID,
	}

	err = database.CreateSubscriber(context.Background(), subscriber3)
	if err != nil {
		t.Fatalf("Couldn't complete CreateSubscriber: %s", err)
	}

	count, err = database.CountSubscribersInProfile(context.Background(), profileID)
	if err != nil {
		t.Fatalf("Couldn't complete CountSubscribersInProfile: %s", err)
	}

	if count != 2 {
		t.Fatalf("Expected 2 subscribers in profile, but got %d", count)
	}
}
