// Copyright 2024 Ella Networks

package db_test

import (
	"path/filepath"
	"testing"

	"github.com/ellanetworks/core/internal/db"
)

func TestDBRoutesEndToEnd(t *testing.T) {
	tempDir := t.TempDir()
	database, err := db.NewDatabase(filepath.Join(tempDir, "db.sqlite3"), initialOperator)
	if err != nil {
		t.Fatalf("Couldn't complete NewDatabase: %s", err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			panic(err)
		}
	}()

	res, err := database.ListRoutes()
	if err != nil {
		t.Fatalf("Couldn't complete RetrieveAll: %s", err)
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
	routeId, err := database.CreateRoute(route)
	if err != nil {
		t.Fatalf("Couldn't complete Create: %s", err)
	}

	res, err = database.ListRoutes()
	if err != nil {
		t.Fatalf("Couldn't complete RetrieveAll: %s", err)
	}
	if len(res) != 1 {
		t.Fatalf("One or more routes weren't found in DB")
	}

	retrievedRoute, err := database.GetRoute(routeId)
	if err != nil {
		t.Fatalf("Couldn't complete Retrieve: %s", err)
	}
	if retrievedRoute.Destination != route.Destination {
		t.Fatalf("The route from the database doesn't match the route that was given")
	}

	if err = database.DeleteRoute(routeId); err != nil {
		t.Fatalf("Couldn't complete Delete: %s", err)
	}
	res, _ = database.ListRoutes()
	if len(res) != 0 {
		t.Fatalf("Routes weren't deleted from the DB properly")
	}
}
