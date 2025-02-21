// Copyright 2024 Ella Networks

package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/canonical/sqlair"
)

const RoutesTableName = "routes"

const QueryCreateRoutesTable = `
	CREATE TABLE IF NOT EXISTS %s (
 		id INTEGER PRIMARY KEY AUTOINCREMENT,
		destination TEXT NOT NULL,
		gateway TEXT NOT NULL,
		interface TEXT NOT NULL,
		metric INTEGER NOT NULL
)`

const (
	listRoutesStmt   = "SELECT &Route.* FROM %s"
	getRouteStmt     = "SELECT &Route.* FROM %s WHERE destination==$Route.destination"
	getRouteByIDStmt = "SELECT &Route.* FROM %s WHERE id==$Route.id"
	createRouteStmt  = "INSERT INTO %s (destination, gateway, interface, metric) VALUES ($Route.destination, $Route.gateway, $Route.interface, $Route.metric)"
	deleteRouteStmt  = "DELETE FROM %s WHERE destination==$Route.destination"
)

// Route represents a route record.
type Route struct {
	ID          int    `db:"id"`
	Destination string `db:"destination"`
	Gateway     string `db:"gateway"`
	Interface   string `db:"interface"`
	Metric      int    `db:"metric"`
}

func (db *Database) ListRoutes() ([]Route, error) {
	stmt, err := sqlair.Prepare(fmt.Sprintf(listRoutesStmt, db.routesTable), Route{})
	if err != nil {
		return nil, err
	}
	var routes []Route
	err = db.conn.Query(context.Background(), stmt).GetAll(&routes)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return routes, nil
}

func (db *Database) GetRoute(destination string) (*Route, error) {
	row := Route{
		Destination: destination,
	}
	stmt, err := sqlair.Prepare(fmt.Sprintf(getRouteStmt, db.routesTable), Route{})
	if err != nil {
		return nil, err
	}
	err = db.conn.Query(context.Background(), stmt, row).Get(&row)
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (db *Database) GetRouteByID(id int) (*Route, error) {
	row := Route{
		ID: id,
	}
	stmt, err := sqlair.Prepare(fmt.Sprintf(getRouteByIDStmt, db.routesTable), Route{})
	if err != nil {
		return nil, err
	}
	err = db.conn.Query(context.Background(), stmt, row).Get(&row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("route with ID %d not found", id)
		}
		return nil, err
	}
	return &row, nil
}

// CreateRoute creates a new route in the DB (non-transactional).
func (db *Database) CreateRoute(route *Route) error {
	_, err := db.GetRoute(route.Destination)
	if err == nil {
		return fmt.Errorf("route with destination %s already exists", route.Destination)
	}
	stmt, err := sqlair.Prepare(fmt.Sprintf(createRouteStmt, db.routesTable), Route{})
	if err != nil {
		return err
	}
	return db.conn.Query(context.Background(), stmt, route).Run()
}

// DeleteRoute deletes a route from the DB (non-transactional).
func (db *Database) DeleteRoute(destination string) error {
	_, err := db.GetRoute(destination)
	if err != nil {
		return err
	}
	stmt, err := sqlair.Prepare(fmt.Sprintf(deleteRouteStmt, db.routesTable), Route{})
	if err != nil {
		return err
	}
	row := Route{Destination: destination}
	return db.conn.Query(context.Background(), stmt, row).Run()
}

func (t *Transaction) CreateRoute(route *Route) error {
	stmt, err := sqlair.Prepare(fmt.Sprintf(createRouteStmt, t.db.routesTable), Route{})
	if err != nil {
		return err
	}
	return t.tx.Query(context.Background(), stmt, route).Run()
}

func (t *Transaction) DeleteRoute(destination string) error {
	stmt, err := sqlair.Prepare(fmt.Sprintf(deleteRouteStmt, t.db.routesTable), Route{})
	if err != nil {
		return err
	}
	row := Route{Destination: destination}
	return t.tx.Query(context.Background(), stmt, row).Run()
}
