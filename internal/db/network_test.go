package db_test

import (
	"path/filepath"
	"testing"

	"github.com/yeastengine/ella/internal/db"
)

func TestDbNetworksEndToEnd(t *testing.T) {
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

	err = database.InitializeNetwork()
	if err != nil {
		t.Fatalf("Couldn't complete InitializeNetwork: %s", err)
	}

	retrievedNetwork, err := database.GetNetwork()
	if err != nil {
		t.Fatalf("Couldn't complete Retrieve: %s", err)
	}
	if retrievedNetwork.Sst != 1 {
		t.Fatalf("The sst from the database doesn't match the expected default")
	}
	if retrievedNetwork.Sd != "102030" {
		t.Fatalf("The sd from the database doesn't match the expected default")
	}
	if retrievedNetwork.Mcc != "001" {
		t.Fatalf("The mcc from the database doesn't match the expected default")
	}
	if retrievedNetwork.Mnc != "01" {
		t.Fatalf("The mnc from the database doesn't match the expected default")
	}
	if retrievedNetwork.GNodeBs != "" {
		t.Fatalf("The gNodeBs from the database doesn't match the expected default")
	}
	if retrievedNetwork.Upf != "" {
		t.Fatalf("The upf from the database doesn't match the expected default")
	}

	network := &db.Network{
		Sst:     123456,
		Sd:      "1",
		Mcc:     "123",
		Mnc:     "456",
		GNodeBs: "1",
		Upf:     "1",
	}
	err = database.UpdateNetwork(network)
	if err != nil {
		t.Fatalf("Couldn't complete Create: %s", err)
	}

	retrievedNetwork, err = database.GetNetwork()
	if err != nil {
		t.Fatalf("Couldn't complete Retrieve: %s", err)
	}
	if retrievedNetwork.Sst != 123456 {
		t.Fatalf("The sst from the database doesn't match the expected value")
	}
	if retrievedNetwork.Sd != "1" {
		t.Fatalf("The sd from the database doesn't match the expected value")
	}
	if retrievedNetwork.Mcc != "123" {
		t.Fatalf("The mcc from the database doesn't match the expected value")
	}
	if retrievedNetwork.Mnc != "456" {
		t.Fatalf("The mnc from the database doesn't match the expected value")
	}
}
