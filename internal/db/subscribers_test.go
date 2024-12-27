package db_test

import (
	"path/filepath"
	"testing"

	"github.com/ellanetworks/core/internal/db"
)

func TestSubscribersDbEndToEnd(t *testing.T) {
	tempDir := t.TempDir()
	database, err := db.NewDatabase(filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	}()

	res, err := database.ListSubscribers()
	if err != nil {
		t.Fatalf("Couldn't complete RetrieveAll: %s", err)
	}

	if len(res) != 0 {
		t.Fatalf("One or more subscribers were found in DB")
	}

	subscriber := &db.Subscriber{
		Imsi:              "001010100007487",
		SequenceNumber:    "123456",
		PermanentKeyValue: "123456",
		OpcValue:          "123456",
	}
	err = database.CreateSubscriber(subscriber)
	if err != nil {
		t.Fatalf("Couldn't complete Create: %s", err)
	}

	res, err = database.ListSubscribers()
	if err != nil {
		t.Fatalf("Couldn't complete RetrieveAll: %s", err)
	}
	if len(res) != 1 {
		t.Fatalf("One or more subscribers weren't found in DB")
	}

	retrievedSubscriber, err := database.GetSubscriber(subscriber.Imsi)
	if err != nil {
		t.Fatalf("Couldn't complete Retrieve: %s", err)
	}
	if retrievedSubscriber.Imsi != subscriber.Imsi {
		t.Fatalf("The subscriber from the database doesn't match the subscriber that was given")
	}
	if retrievedSubscriber.SequenceNumber != subscriber.SequenceNumber {
		t.Fatalf("The sequence number from the database doesn't match the sequence number that was given")
	}
	if retrievedSubscriber.PermanentKeyValue != subscriber.PermanentKeyValue {
		t.Fatalf("The permanent key value from the database doesn't match the permanent key value that was given")
	}
	if retrievedSubscriber.OpcValue != subscriber.OpcValue {
		t.Fatalf("The OPC value from the database doesn't match the OPC value that was given")
	}

	profileData := &db.Profile{
		Name:     "myprofilename",
		UeIpPool: "0.0.0.0/24",
	}
	err = database.CreateProfile(profileData)
	if err != nil {
		t.Fatalf("Couldn't complete Create: %s", err)
	}

	subscriber.SequenceNumber = "654321"
	if err = database.UpdateSubscriber(subscriber); err != nil {
		t.Fatalf("Couldn't complete Update: %s", err)
	}

	retrievedSubscriber, err = database.GetSubscriber(subscriber.Imsi)
	if err != nil {
		t.Fatalf("Couldn't complete Retrieve: %s", err)
	}

	if retrievedSubscriber.SequenceNumber != "654321" {
		t.Fatalf("Sequence numbers don't match: %s", retrievedSubscriber.SequenceNumber)
	}

	if err = database.DeleteSubscriber(subscriber.Imsi); err != nil {
		t.Fatalf("Couldn't complete Delete: %s", err)
	}
	res, _ = database.ListSubscribers()
	if len(res) != 0 {
		t.Fatalf("Subscribers weren't deleted from the DB properly")
	}
}
