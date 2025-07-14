package db_test

import (
	"context"
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
			t.Fatalf("Couldn't complete Close: %s", err)
		}
	}()

	res, err := database.ListRoutes(context.Background())
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

	tx, err := database.BeginTransaction()
	if err != nil {
		t.Fatalf("Couldn't complete BeginTransaction: %s", err)
	}
	committedCreate := false
	defer func() {
		if !committedCreate {
			if rbErr := tx.Rollback(); rbErr != nil {
				t.Fatalf("Couldn't complete Rollback: %s", rbErr)
			}
		}
	}()

	routeID, err := tx.CreateRoute(context.Background(), route)
	if err != nil {
		t.Fatalf("Couldn't complete Create: %s", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("Couldn't complete Commit: %s", err)
	}
	committedCreate = true

	if routeID != 1 {
		t.Fatalf("expected routeID 1, got %d", routeID)
	}

	res, err = database.ListRoutes(context.Background())
	if err != nil {
		t.Fatalf("Couldn't complete RetrieveAll: %s", err)
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

	tx, err = database.BeginTransaction()
	if err != nil {
		t.Fatalf("Couldn't complete BeginTransaction: %s", err)
	}
	committedDelete := false
	defer func() {
		if !committedDelete {
			if rbErr := tx.Rollback(); rbErr != nil {
				t.Fatalf("Couldn't complete Rollback: %s", rbErr)
			}
		}
	}()

	if err := tx.DeleteRoute(context.Background(), routeID); err != nil {
		t.Fatalf("Couldn't complete Delete: %s", err)
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("Couldn't complete Commit: %s", err)
	}

	committedDelete = true

	res, err = database.ListRoutes(context.Background())
	if err != nil {
		t.Fatalf("Couldn't complete RetrieveAll: %s", err)
	}
	if len(res) != 0 {
		t.Fatalf("Routes weren't deleted from the DB properly")
	}
}
