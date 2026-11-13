// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// SPDX-FileCopyrightText: Ella Networks Inc.

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
)

const RoutesTableName = "routes"

const (
	listRoutesPageSQL = "SELECT id, destination, gateway, interface, metric, COUNT(*) OVER() FROM " + RoutesTableName + " ORDER BY id DESC LIMIT ? OFFSET ?"
	listAllRoutesSQL  = "SELECT id, destination, gateway, interface, metric FROM " + RoutesTableName + " ORDER BY id ASC"
	getRouteSQL       = "SELECT id, destination, gateway, interface, metric FROM " + RoutesTableName + " WHERE id==?"
	createRouteSQL    = "INSERT INTO " + RoutesTableName + " (destination, gateway, interface, metric) VALUES (?, ?, ?, ?)"
	deleteRouteSQL    = "DELETE FROM " + RoutesTableName + " WHERE id==?"
	countRoutesSQL    = "SELECT COUNT(*) FROM " + RoutesTableName
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
	ID          int64
	Destination string
	Gateway     string
	Interface   NetworkInterface
	Metric      int
}

func (db *Database) ListRoutesPage(ctx context.Context, page int, perPage int) (_ []Route, _ int, err error) {
	ctx, finish := startDBQueryNamed(ctx, "SELECT "+RoutesTableName+" (paged)", RoutesTableName, "SELECT", attribute.Int("ella.db.page", page), attribute.Int("ella.db.page_size", perPage))
	defer func() { finish(err) }()

	routes, count, err := queryPageWithTotal(ctx, db.reader(), listRoutesPageSQL, scanRoute, perPage, (page-1)*perPage)
	if err != nil {
		return nil, 0, fmt.Errorf("query failed: %w", err)
	}

	if len(routes) == 0 {
		fallbackCount, err := db.CountRoutes(ctx)
		if err != nil {
			return nil, 0, fmt.Errorf("count failed: %w", err)
		}

		return nil, fallbackCount, nil
	}

	return routes, count, nil
}

// ListAllRoutes returns every route. The framed-route overlap validation reads
// the full set to enforce non-overlap against the global routing table.
func (db *Database) ListAllRoutes(ctx context.Context) (_ []Route, err error) {
	ctx, finish := startDBQuery(ctx, RoutesTableName, "SELECT")
	defer func() { finish(err) }()

	routes, err := queryAll(ctx, db.reader(), listAllRoutesSQL, scanRoute)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return routes, nil
}

func (db *Database) GetRoute(ctx context.Context, id int64) (_ *Route, err error) {
	ctx, finish := startDBQuery(ctx, RoutesTableName, "SELECT")
	defer func() { finish(err) }()

	row, err := scanRoute(db.reader().QueryRowContext(ctx, getRouteSQL, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return &row, nil
}

func (db *Database) CreateRoute(ctx context.Context, route *Route) (_ int64, err error) {
	ctx, finish := startDBQuery(ctx, RoutesTableName, "INSERT")
	defer func() { finish(err) }()

	result, err := db.applyCreateRoute(ctx, route)
	if err != nil {
		return 0, err
	}

	id, _ := result.(int64)

	return id, nil
}

func (db *Database) DeleteRoute(ctx context.Context, id int64) (err error) {
	ctx, finish := startDBQuery(ctx, RoutesTableName, "DELETE")
	defer func() { finish(err) }()

	_, err = db.applyDeleteRoute(ctx, &int64Payload{Value: id})
	if err != nil {
		return err
	}

	return nil
}

// NumRoutes returns route count
func (db *Database) CountRoutes(ctx context.Context) (_ int, err error) {
	ctx, finish := startDBQuery(ctx, RoutesTableName, "SELECT")
	defer func() { finish(err) }()

	count, err := queryCount(ctx, db.reader(), countRoutesSQL)
	if err != nil {
		return 0, fmt.Errorf("query failed: %w", err)
	}

	return count, nil
}

func scanRoute(row scanner) (Route, error) {
	var v Route

	if err := row.Scan(&v.ID, &v.Destination, &v.Gateway, &v.Interface, &v.Metric); err != nil {
		return Route{}, err
	}

	return v, nil
}
