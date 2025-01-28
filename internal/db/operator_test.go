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

	err = database.UpdateOperatorSlice(1, 1056816)
	if err != nil {
		t.Fatalf("Couldn't complete Create: %s", err)
	}

	retrievedOperator, err = database.GetOperator()
	if err != nil {
		t.Fatalf("Couldn't complete Retrieve: %s", err)
	}
	if retrievedOperator.Sst != 1 {
		t.Fatalf("The sst from the database doesn't match the expected value")
	}
	if retrievedOperator.Sd != 1056816 {
		t.Fatalf("The sd from the database doesn't match the expected value")
	}
	if retrievedOperator.GetHexSd() != "102030" {
		t.Fatalf("The hex sd from the database doesn't match the expected value")
	}
	retrievedOperatorCode, err = database.GetOperatorCode()
	if err != nil {
		t.Fatalf("Couldn't complete Retrieve: %s", err)
	}
	if retrievedOperatorCode != "0123456789ABCDEF0123456789ABCDEF" {
		t.Fatalf("The operator code from the database doesn't match the expected default")
	}

	mcc := "002"
	mnc := "02"
	err = database.UpdateOperatorId(mcc, mnc)
	if err != nil {
		t.Fatalf("Couldn't complete Create: %s", err)
	}

	operator, err := database.GetOperator()
	if err != nil {
		t.Fatalf("Couldn't complete Retrieve: %s", err)
	}
	if operator.Mcc != mcc {
		t.Fatalf("The mcc from the database doesn't match the expected value")
	}
	if operator.Mnc != mnc {
		t.Fatalf("The mnc from the database doesn't match the expected value")
	}
	if operator.Sst != 1 {
		t.Fatalf("The sst from the database doesn't match the expected value")
	}
	if operator.Sd != 1056816 {
		t.Fatalf("The sd from the database doesn't match the expected value")
	}
}
