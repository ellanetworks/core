package db_test

import (
	"path/filepath"
	"testing"

	"github.com/ellanetworks/core/internal/db"
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
	if retrievedNetwork.Mcc != "001" {
		t.Fatalf("The mcc from the database doesn't match the expected default")
	}
	if retrievedNetwork.Mnc != "01" {
		t.Fatalf("The mnc from the database doesn't match the expected default")
	}

	network := &db.Network{
		Mcc: "123",
		Mnc: "456",
	}
	err = database.UpdateNetwork(network)
	if err != nil {
		t.Fatalf("Couldn't complete Create: %s", err)
	}

	retrievedNetwork, err = database.GetNetwork()
	if err != nil {
		t.Fatalf("Couldn't complete Retrieve: %s", err)
	}
	if retrievedNetwork.Mcc != "123" {
		t.Fatalf("The mcc from the database doesn't match the expected value")
	}
	if retrievedNetwork.Mnc != "456" {
		t.Fatalf("The mnc from the database doesn't match the expected value")
	}
}
