// Copyright 2024 Ella Networks

package db_test

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"github.com/ellanetworks/core/internal/db"
)

func TestGetHexSd(t *testing.T) {
	testCases := []struct {
		name     string
		input    []byte // nil => unset; when set must be len 3
		expected string
	}{
		{
			name:     "Unset (nil)",
			input:    nil,
			expected: "",
		},
		{
			name:     "Normal case with leading zeros",
			input:    []byte{0x01, 0x20, 0x30},
			expected: "0x012030",
		},
		{
			name:     "Zero value",
			input:    []byte{0x00, 0x00, 0x00},
			expected: "0x000000",
		},
		{
			name:     "Maximum 24-bit value",
			input:    []byte{0xFF, 0xFF, 0xFF},
			expected: "0xffffff",
		},
		{
			name:     "No padding needed (hex letters lowercased by %02x)",
			input:    []byte{0xAB, 0xCD, 0xEF},
			expected: "0xabcdef",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			op := &db.Operator{Sd: tc.input}
			result := op.GetHexSd()
			if result != tc.expected {
				t.Errorf("expected %q but got %q (input=%v)", tc.expected, result, tc.input)
			}
		})
	}
}

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

	retrievedOperator, err := database.GetOperator(context.Background())
	if err != nil {
		t.Fatalf("Couldn't complete Retrieve: %s", err)
	}
	if retrievedOperator.Mcc != "001" {
		t.Fatalf("The mcc from the database doesn't match the expected default")
	}
	if retrievedOperator.Mnc != "01" {
		t.Fatalf("The mnc from the database doesn't match the expected default")
	}

	retrievedOperatorCode, err := database.GetOperatorCode(context.Background())
	if err != nil {
		t.Fatalf("Couldn't complete Retrieve: %s", err)
	}

	if retrievedOperatorCode == "" {
		t.Fatalf("The operator code from the database doesn't match the expected default")
	}

	err = database.UpdateOperatorSlice(context.Background(), 1, []byte{0x10, 0x20, 0x30})
	if err != nil {
		t.Fatalf("Couldn't complete Create: %s", err)
	}

	retrievedOperator, err = database.GetOperator(context.Background())
	if err != nil {
		t.Fatalf("Couldn't complete Retrieve: %s", err)
	}
	if retrievedOperator.Sst != 1 {
		t.Fatalf("The sst from the database doesn't match the expected value")
	}
	expectedSd := []byte{0x10, 0x20, 0x30}

	if !bytes.Equal(retrievedOperator.Sd, expectedSd) {
		t.Fatalf("The sd from the database doesn't match the expected value")
	}

	if retrievedOperator.GetHexSd() != "0x102030" {
		t.Fatalf("The hex sd from the database doesn't match the expected value, got %q", retrievedOperator.GetHexSd())
	}

	retrievedOperatorCode, err = database.GetOperatorCode(context.Background())
	if err != nil {
		t.Fatalf("Couldn't complete Retrieve: %s", err)
	}
	if retrievedOperatorCode == "" {
		t.Fatalf("The operator code from the database doesn't match the expected default")
	}

	mcc := "002"
	mnc := "02"
	err = database.UpdateOperatorID(context.Background(), mcc, mnc)
	if err != nil {
		t.Fatalf("Couldn't complete Create: %s", err)
	}

	operator, err := database.GetOperator(context.Background())
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

	if !bytes.Equal(operator.Sd, expectedSd) {
		t.Fatalf("The sd from the database doesn't match the expected value")
	}
}
