// Copyright 2024 Ella Networks

package db_test

import (
	"path/filepath"
	"testing"

	"github.com/ellanetworks/core/internal/db"
)

func TestGetHexSd(t *testing.T) {
	testCases := []struct {
		name     string
		input    int
		expected string
	}{
		{
			name:     "Normal case with leading zeros",
			input:    0x012030,
			expected: "012030",
		},
		{
			name:     "Zero value",
			input:    0x0,
			expected: "000000",
		},
		{
			name:     "Maximum 6-digit hex value",
			input:    0xFFFFFF,
			expected: "FFFFFF",
		},
		{
			name:     "Value with no additional padding needed",
			input:    0xABCDEF,
			expected: "ABCDEF",
		},
		{
			name:  "Value greater than six hex digits",
			input: 0x1234567,
			// Note: %06X will not truncate numbers larger than 6 digits.
			expected: "1234567",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			op := &db.Operator{Sd: tc.input}
			result := op.GetHexSd()
			if result != tc.expected {
				t.Errorf("For input 0x%X, expected %s but got %s", tc.input, tc.expected, result)
			}
		})
	}
}

func TestDbOperatorsEndToEnd(t *testing.T) {
	tempDir := t.TempDir()
	database, err := db.NewDatabase(filepath.Join(tempDir, "db.sqlite3"), initialOperator)
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	}()

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
	err = database.UpdateOperatorID(mcc, mnc)
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
