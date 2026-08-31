// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db_test

import (
	"context"
	"testing"
)

func TestDbOperatorsEndToEnd(t *testing.T) {
	database := setupTestDB(t)

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
}
