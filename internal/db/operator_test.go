package db_test

import (
	"path/filepath"
	"testing"

	"github.com/ellanetworks/core/internal/db"
)

func TestDbOperatorsEndToEnd(t *testing.T) {
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

	err = database.InitializeOperator()
	if err != nil {
		t.Fatalf("Couldn't complete InitializeOperator: %s", err)
	}

	retrievedOperatorId, err := database.GetOperatorId()
	if err != nil {
		t.Fatalf("Couldn't complete Retrieve: %s", err)
	}
	if retrievedOperatorId.Mcc != "001" {
		t.Fatalf("The mcc from the database doesn't match the expected default")
	}
	if retrievedOperatorId.Mnc != "01" {
		t.Fatalf("The mnc from the database doesn't match the expected default")
	}

	retrievedOperatorCode, err := database.GetOperatorCode()
	if err != nil {
		t.Fatalf("Couldn't complete Retrieve: %s", err)
	}

	if retrievedOperatorCode != "0123456789ABCDEF0123456789ABCDEF" {
		t.Fatalf("The operator code from the database doesn't match the expected default")
	}

	operatorId := &db.OperatorId{
		Mcc: "123",
		Mnc: "456",
	}
	err = database.UpdateOperatorId(operatorId)
	if err != nil {
		t.Fatalf("Couldn't complete Create: %s", err)
	}

	retrievedOperatorId, err = database.GetOperatorId()
	if err != nil {
		t.Fatalf("Couldn't complete Retrieve: %s", err)
	}
	if retrievedOperatorId.Mcc != "123" {
		t.Fatalf("The mcc from the database doesn't match the expected value")
	}
	if retrievedOperatorId.Mnc != "456" {
		t.Fatalf("The mnc from the database doesn't match the expected value")
	}
	retrievedOperatorCode, err = database.GetOperatorCode()
	if err != nil {
		t.Fatalf("Couldn't complete Retrieve: %s", err)
	}
	if retrievedOperatorCode != "0123456789ABCDEF0123456789ABCDEF" {
		t.Fatalf("The operator code from the database doesn't match the expected default")
	}
}
