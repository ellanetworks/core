// Copyright 2024 Ella Networks

package db_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ellanetworks/core/internal/db"
)

func TestDataNetworksEndToEnd(t *testing.T) {
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

	res, total, err := database.ListDataNetworksPage(context.Background(), 1, 10)
	if err != nil {
		t.Fatalf("couldn't list data networks: %s", err)
	}

	if total != 1 {
		t.Fatalf("Expected total count to be 1, but got %d", total)
	}

	if len(res) != 1 {
		t.Fatalf("Expected only one data network (the default one), but found %d", len(res))
	}

	if res[0].Name != "internet" {
		t.Fatalf("Expected the default data network to be named 'internet', but got '%s'", res[0].Name)
	}

	if res[0].IPPool != DefaultDNIPPool {
		t.Fatalf("Expected the default data network to have IP pool '%s', but got '%s'", DefaultDNIPPool, res[0].IPPool)
	}

	newDN := &db.DataNetwork{
		Name:   "not-internet",
		IPPool: "2.2.2.0/29",
	}

	err = database.CreateDataNetwork(context.Background(), newDN)
	if err != nil {
		t.Fatalf("couldn't create data network: %s", err)
	}

	res, total, err = database.ListDataNetworksPage(context.Background(), 1, 10)
	if err != nil {
		t.Fatalf("couldn't list data networks: %s", err)
	}

	if total != 2 {
		t.Fatalf("Expected total count to be 2, but got %d", total)
	}

	if len(res) != 2 {
		t.Fatalf("Expected two data networks, but found %d", len(res))
	}

	retrievedDN, err := database.GetDataNetwork(context.Background(), newDN.Name)
	if err != nil {
		t.Fatalf("couldn't get data network: %s", err)
	}

	if retrievedDN.Name != newDN.Name || retrievedDN.IPPool != newDN.IPPool {
		t.Fatalf("The data network from the database doesn't match the one that was created")
	}

	if err = database.DeleteDataNetwork(context.Background(), newDN.Name); err != nil {
		t.Fatalf("couldn't delete data network: %s", err)
	}

	res, total, err = database.ListDataNetworksPage(context.Background(), 1, 10)
	if err != nil {
		t.Fatalf("couldn't list data networks: %s", err)
	}

	if total != 1 {
		t.Fatalf("Expected total count to be 1 after deletion, but got %d", total)
	}

	if len(res) != 1 {
		t.Fatalf("Expected only one data network (the default one) after deletion, but found %d", len(res))
	}
}
