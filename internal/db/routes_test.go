// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/db"
)

func TestDBRoutesEndToEnd(t *testing.T) {
	database := setupTestDB(t)

	res, total, err := database.ListRoutesPage(context.Background(), 1, 10)
	if err != nil {
		t.Fatalf("Couldn't complete RetrieveAll: %s", err)
	}

	if total != 0 {
		t.Fatalf("Expected total count to be 0, but got %d", total)
	}

	if len(res) != 0 {
		t.Fatalf("One or more routes were found in DB")
	}

	route := &db.Route{
		Destination: "1.2.3.4/24",
		Gateway:     "2.1.2.1",
		Interface:   db.N3,
		Metric:      1,
	}

	routeID, err := database.CreateRoute(context.Background(), route)
	if err != nil {
		t.Fatalf("Couldn't complete Create: %s", err)
	}

	if routeID != 1 {
		t.Fatalf("expected routeID 1, got %d", routeID)
	}

	res, total, err = database.ListRoutesPage(context.Background(), 1, 10)
	if err != nil {
		t.Fatalf("Couldn't complete RetrieveAll: %s", err)
	}

	if total != 1 {
		t.Fatalf("Expected total count to be 1, but got %d", total)
	}

	if len(res) != 1 {
		t.Fatalf("One or more routes weren't found in DB")
	}

	retrievedRoute, err := database.GetRoute(context.Background(), routeID)
	if err != nil {
		t.Fatalf("Couldn't complete Retrieve: %s", err)
	}

	if retrievedRoute.Destination != route.Destination {
		t.Fatalf("The route from the database doesn't match the route that was given")
	}

	if retrievedRoute.Gateway != route.Gateway {
		t.Fatalf("The permanent key value from the database doesn't match the permanent key value that was given")
	}

	if retrievedRoute.Interface != route.Interface {
		t.Fatalf("The OPC value from the database doesn't match the OPC value that was given")
	}

	if err := database.DeleteRoute(context.Background(), routeID); err != nil {
		t.Fatalf("Couldn't complete Delete: %s", err)
	}

	res, total, err = database.ListRoutesPage(context.Background(), 1, 10)
	if err != nil {
		t.Fatalf("Couldn't complete RetrieveAll: %s", err)
	}

	if total != 0 {
		t.Fatalf("Expected total count to be 0, but got %d", total)
	}

	if len(res) != 0 {
		t.Fatalf("Routes weren't deleted from the DB properly")
	}
}
