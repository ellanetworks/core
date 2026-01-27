// Copyright 2024 Ella Networks

package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/canonical/sqlair"
	"github.com/prometheus/client_golang/prometheus"
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
	listRoutesPageStmt = "SELECT &Route.* FROM %s ORDER BY id DESC LIMIT $ListArgs.limit OFFSET $ListArgs.offset"
	getRouteStmt       = "SELECT &Route.* FROM %s WHERE id==$Route.id"
	createRouteStmt    = "INSERT INTO %s (destination, gateway, interface, metric) VALUES ($Route.destination, $Route.gateway, $Route.interface, $Route.metric)"
	deleteRouteStmt    = "DELETE FROM %s WHERE id==$Route.id"
	countRoutesStmt    = "SELECT COUNT(*) AS &NumItems.count FROM %s"
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

func (db *Database) ListRoutesPage(ctx context.Context, page int, perPage int) ([]Route, int, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s (paged)", "SELECT", RoutesTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("SELECT"),
			attribute.String("db.collection", RoutesTableName),
			attribute.Int("page", page),
			attribute.Int("per_page", perPage),
		),
	)
	defer span.End()

	count, err := db.CountRoutes(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "count failed")

		return nil, 0, err
	}

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(RoutesTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(RoutesTableName, "select").Inc()

	var routes []Route

	args := ListArgs{
		Limit:  perPage,
		Offset: (page - 1) * perPage,
	}

	err = db.conn.Query(ctx, db.listRoutesStmt, args).GetAll(&routes)
	if err != nil {
		if err == sql.ErrNoRows {
			span.SetStatus(codes.Ok, "no rows")
			return nil, count, nil
		}

		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return nil, 0, err
	}

	span.SetStatus(codes.Ok, "")

	return routes, count, nil
}

func (db *Database) GetRoute(ctx context.Context, id int64) (*Route, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "SELECT", RoutesTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("SELECT"),
			attribute.String("db.collection", RoutesTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(RoutesTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(RoutesTableName, "select").Inc()

	row := Route{ID: id}

	err := db.conn.Query(ctx, db.getRouteStmt, row).Get(&row)
	if err != nil {
		if err == sql.ErrNoRows {
			span.SetStatus(codes.Ok, "no rows")
			return nil, ErrNotFound
		}

		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return nil, err
	}

	span.SetStatus(codes.Ok, "")

	return &row, nil
}

func (t *Transaction) CreateRoute(ctx context.Context, route *Route) (int64, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "INSERT", RoutesTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("INSERT"),
			attribute.String("db.collection", RoutesTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(RoutesTableName, "insert"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(RoutesTableName, "insert").Inc()

	var outcome sqlair.Outcome

	err := t.tx.Query(ctx, t.db.createRouteStmt, route).Get(&outcome)
	if err != nil {
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
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "DELETE", RoutesTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("DELETE"),
			attribute.String("db.collection", RoutesTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(RoutesTableName, "delete"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(RoutesTableName, "delete").Inc()

	err := t.tx.Query(ctx, t.db.deleteRouteStmt, Route{ID: id}).Run()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")

		return err
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

// NumRoutes returns route count
func (db *Database) CountRoutes(ctx context.Context) (int, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "SELECT", RoutesTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("SELECT"),
			attribute.String("db.collection", RoutesTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(RoutesTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(RoutesTableName, "select").Inc()

	var result NumItems

	err := db.conn.Query(ctx, db.countRoutesStmt).Get(&result)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")

		return 0, err
	}

	span.SetStatus(codes.Ok, "")

	return result.Count, nil
}
