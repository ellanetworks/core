// Copyright 2024 Ella Networks

package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/canonical/sqlair"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
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

func (db *Database) ListRoutes(ctx context.Context) ([]Route, error) {
	operation := "SELECT"
	target := RoutesTableName
	spanName := fmt.Sprintf("%s %s", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	stmt := fmt.Sprintf(listRoutesStmt, db.routesTable)
	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(stmt),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
	)

	q, err := sqlair.Prepare(stmt, Route{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "prepare failed")
		return nil, err
	}

	var routes []Route
	err = db.conn.Query(ctx, q).GetAll(&routes)
	if err != nil {
		if err == sql.ErrNoRows {
			span.SetStatus(codes.Ok, "no rows")
			return nil, nil
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")
		return nil, err
	}

	span.SetStatus(codes.Ok, "")
	return routes, nil
}

func (db *Database) GetRoute(ctx context.Context, id int64) (*Route, error) {
	operation := "SELECT"
	target := RoutesTableName
	spanName := fmt.Sprintf("%s %s", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	stmt := fmt.Sprintf(getRouteStmt, db.routesTable)
	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(stmt),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
	)

	row := Route{ID: id}
	q, err := sqlair.Prepare(stmt, Route{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "prepare failed")
		return nil, err
	}

	if err := db.conn.Query(ctx, q, row).Get(&row); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")
		return nil, err
	}

	span.SetStatus(codes.Ok, "")
	return &row, nil
}

func (t *Transaction) CreateRoute(ctx context.Context, route *Route) (int64, error) {
	operation := "INSERT"
	target := t.db.routesTable
	spanName := fmt.Sprintf("%s %s", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	stmt := fmt.Sprintf(createRouteStmt, t.db.routesTable)
	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(stmt),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
	)

	var outcome sqlair.Outcome
	q, err := sqlair.Prepare(stmt, Route{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "prepare failed")
		return 0, err
	}

	if err := t.tx.Query(ctx, q, route).Get(&outcome); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")
		return 0, err
	}

	id, err := outcome.Result().LastInsertId()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "retrieving insert ID failed")
		return 0, err
	}

	span.SetStatus(codes.Ok, "")
	return id, nil
}

func (t *Transaction) DeleteRoute(ctx context.Context, id int64) error {
	operation := "DELETE"
	target := t.db.routesTable
	spanName := fmt.Sprintf("%s %s", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	stmt := fmt.Sprintf(deleteRouteStmt, t.db.routesTable)
	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(stmt),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
	)

	q, err := sqlair.Prepare(stmt, Route{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "prepare failed")
		return err
	}

	if err := t.tx.Query(ctx, q, Route{ID: id}).Run(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")
		return err
	}

	span.SetStatus(codes.Ok, "")
	return nil
}
