package db_test

import (
	"path/filepath"
	"testing"

	"github.com/yeastengine/ella/internal/db"
)

func TestSubscribersEndToEnd(t *testing.T) {
	tempDir := t.TempDir()
	database, err := db.NewDatabase(filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}
	defer database.Close()

	subscriber := &db.Subscriber{
		UeId:              "imsi-001010100007487",
		PlmnID:            "123456",
		SequenceNumber:    "123456",
		PermanentKeyValue: "123456",
		OpcValue:          "123456",
	}
	res, err := database.ListSubscribers()
	if err != nil {
		t.Fatalf("Couldn't complete RetrieveAll: %s", err)
	}

	if len(res) != 0 {
		t.Fatalf("One or more subscribers were found in DB")
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

	retrievedSubscriber, err := database.GetSubscriber(subscriber.UeId)
	if err != nil {
		t.Fatalf("Couldn't complete Retrieve: %s", err)
	}
	if retrievedSubscriber.UeId != subscriber.UeId {
		t.Fatalf("The subscriber from the database doesn't match the subscriber that was given")
	}

	err = database.UpdateSubscriberSequenceNumber(subscriber.UeId, "654321")
	if err != nil {
		t.Fatalf("Couldn't complete Update: %s", err)
	}

	retrievedSubscriber, err = database.GetSubscriber(subscriber.UeId)
	if err != nil {
		t.Fatalf("Couldn't complete Retrieve: %s", err)
	}
	if retrievedSubscriber.SequenceNumber != "654321" {
		t.Fatalf("Sequence numbers don't match: %s", retrievedSubscriber.SequenceNumber)
	}

	if err = database.UpdateSubscriberProfile(retrievedSubscriber.UeId, "internet", "001", 1, "2314", "200000", "200000", 9, 8); err != nil {
		t.Fatalf("Couldn't complete Update: %s", err)
	}

	retrievedSubscriber, err = database.GetSubscriber(subscriber.UeId)
	if err != nil {
		t.Fatalf("Couldn't complete Retrieve: %s", err)
	}
	if retrievedSubscriber.Dnn != "internet" {
		t.Fatalf("The DNN from the database doesn't match the DNN that was given")
	}
	if retrievedSubscriber.Sd != "001" {
		t.Fatalf("The SD from the database doesn't match the SD that was given")
	}
	if retrievedSubscriber.Sst != 1 {
		t.Fatalf("The SST from the database doesn't match the SST that was given")
	}
	if retrievedSubscriber.PlmnID != "2314" {
		t.Fatalf("The PLMN ID from the database doesn't match the PLMN ID that was given")
	}
	if retrievedSubscriber.BitRateUplink != "200000" {
		t.Fatalf("The uplink bitrate from the database doesn't match the uplink bitrate that was given")
	}
	if retrievedSubscriber.BitRateDownlink != "200000" {
		t.Fatalf("The downlink bitrate from the database doesn't match the downlink bitrate that was given")
	}
	if retrievedSubscriber.Var5qi != 9 {
		t.Fatalf("The var5qi from the database doesn't match the var5qi that was given")
	}
	if retrievedSubscriber.PriorityLevel != 8 {
		t.Fatalf("The priority level from the database doesn't match the priority level that was given")
	}

	if err = database.DeleteSubscriber(subscriber.UeId); err != nil {
		t.Fatalf("Couldn't complete Delete: %s", err)
	}
	res, _ = database.ListSubscribers()
	if len(res) != 0 {
		t.Fatalf("Subscribers weren't deleted from the DB properly")
	}
}
