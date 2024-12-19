package db_test

import (
	"path/filepath"
	"testing"

	"github.com/yeastengine/ella/internal/db"
)

func TestNetworkSlicesEndToEnd(t *testing.T) {
	tempDir := t.TempDir()
	database, err := db.NewDatabase(filepath.Join(tempDir, "db.sqlite3"))
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}
	defer database.Close()

	res, err := database.ListNetworkSlices()
	if err != nil {
		t.Fatalf("Couldn't complete RetrieveAll: %s", err)
	}

	if len(res) != 0 {
		t.Fatalf("One or more networkSlices were found in DB")
	}

	networkSlice := &db.NetworkSlice{
		Name:    "my-networkSlice",
		Sst:     "123456",
		Sd:      "1",
		Mcc:     "123",
		Mnc:     "456",
		GNodeBs: "1",
		Upf:     "1",
	}
	err = database.CreateNetworkSlice(networkSlice)
	if err != nil {
		t.Fatalf("Couldn't complete Create: %s", err)
	}

	res, err = database.ListNetworkSlices()
	if err != nil {
		t.Fatalf("Couldn't complete RetrieveAll: %s", err)
	}
	if len(res) != 1 {
		t.Fatalf("One or more networkSlices weren't found in DB")
	}

	retrievedNetworkSlice, err := database.GetNetworkSlice(networkSlice.Name)
	if err != nil {
		t.Fatalf("Couldn't complete Retrieve: %s", err)
	}
	if retrievedNetworkSlice.Name != networkSlice.Name {
		t.Fatalf("The networkSlice name from the database doesn't match the networkSlice name that was given")
	}
	if retrievedNetworkSlice.Sst != networkSlice.Sst {
		t.Fatalf("The sst from the database doesn't match the sst that was given")
	}
	if retrievedNetworkSlice.Sd != networkSlice.Sd {
		t.Fatalf("The sd from the database doesn't match the sd that was given")
	}
	if retrievedNetworkSlice.Mcc != networkSlice.Mcc {
		t.Fatalf("The mcc from the database doesn't match the mcc that was given")
	}
	if retrievedNetworkSlice.Mnc != networkSlice.Mnc {
		t.Fatalf("The mnc from the database doesn't match the mnc that was given")
	}
	if retrievedNetworkSlice.GNodeBs != networkSlice.GNodeBs {
		t.Fatalf("The gNodeBs from the database doesn't match the gNodeBs that was given")
	}
	if retrievedNetworkSlice.Upf != networkSlice.Upf {
		t.Fatalf("The upf from the database doesn't match the upf that was given")
	}

	if err = database.DeleteNetworkSlice(networkSlice.Name); err != nil {
		t.Fatalf("Couldn't complete Delete: %s", err)
	}
	res, _ = database.ListNetworkSlices()
	if len(res) != 0 {
		t.Fatalf("NetworkSlices weren't deleted from the DB properly")
	}
}
