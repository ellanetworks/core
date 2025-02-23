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
	listRoutesStmt  = "SELECT &Route.* FROM %s"
	getRouteStmt    = "SELECT &Route.* FROM %s WHERE id==$Route.id"
	createRouteStmt = "INSERT INTO %s (destination, gateway, interface, metric) VALUES ($Route.destination, $Route.gateway, $Route.interface, $Route.metric)"
	deleteRouteStmt = "DELETE FROM %s WHERE id==$Route.id"
)

// NetworkInterface is an enum for network interface keys.
type NetworkInterface int

const (
	N3 NetworkInterface = iota
	N6
)

func (ni NetworkInterface) String() string {
	switch ni {
	case N3:
		return "n3"
	case N6:
		return "n6"
	default:
		return "Unknown"
	}
}

// Route represents a route record.
type Route struct {
	ID          int64            `db:"id"`
	Destination string           `db:"destination"`
	Gateway     string           `db:"gateway"`
	Interface   NetworkInterface `db:"interface"`
	Metric      int              `db:"metric"`
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

func (db *Database) GetRoute(id int64) (*Route, error) {
	row := Route{
		ID: id,
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

func (t *Transaction) CreateRoute(route *Route) (int64, error) {
	stmt, err := sqlair.Prepare(fmt.Sprintf(createRouteStmt, t.db.routesTable), Route{})
	if err != nil {
		return 0, err
	}
	var outcome sqlair.Outcome
	err = t.tx.Query(context.Background(), stmt, route).Get(&outcome)
	if err != nil {
		return 0, err
	}
	insertedRowID, err := outcome.Result().LastInsertId()
	if err != nil {
		return 0, err
	}
	return insertedRowID, nil
}

func (t *Transaction) DeleteRoute(id int64) error {
	stmt, err := sqlair.Prepare(fmt.Sprintf(deleteRouteStmt, t.db.routesTable), Route{})
	if err != nil {
		return err
	}
	row := Route{ID: id}
	return t.tx.Query(context.Background(), stmt, row).Run()
}
