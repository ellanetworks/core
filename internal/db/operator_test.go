// Copyright 2024 Ella Networks

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

	retrievedOperator, err := database.GetOperator()
	if err != nil {
		t.Fatalf("Couldn't complete Retrieve: %s", err)
	}
	if retrievedOperator.Mcc != "001" {
		t.Fatalf("The mcc from the database doesn't match the expected default")
	}
	if retrievedOperator.Mnc != "01" {
		t.Fatalf("The mnc from the database doesn't match the expected default")
	}

	retrievedOperatorCode, err := database.GetOperatorCode()
	if err != nil {
		t.Fatalf("Couldn't complete Retrieve: %s", err)
	}

	if retrievedOperatorCode != "0123456789ABCDEF0123456789ABCDEF" {
		t.Fatalf("The operator code from the database doesn't match the expected default")
	}

	operator := &db.Operator{
		Mcc: "123",
		Mnc: "456",
	}
	err = database.UpdateOperator(operator)
	if err != nil {
		t.Fatalf("Couldn't complete Create: %s", err)
	}

	retrievedOperator, err = database.GetOperator()
	if err != nil {
		t.Fatalf("Couldn't complete Retrieve: %s", err)
	}
	if retrievedOperator.Mcc != "123" {
		t.Fatalf("The mcc from the database doesn't match the expected value")
	}
	if retrievedOperator.Mnc != "456" {
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
