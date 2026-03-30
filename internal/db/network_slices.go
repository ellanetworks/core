// Copyright 2026 Ella Networks

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/canonical/sqlair"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
)

const NetworkSlicesTableName = "network_slices"

const (
	listNetworkSlicesStmt        = "SELECT &NetworkSlice.* FROM %s ORDER BY id ASC"
	getNetworkSliceByIDStmt      = "SELECT &NetworkSlice.* FROM %s WHERE id==$NetworkSlice.id"
	getNetworkSliceBySstSdStmt   = "SELECT &NetworkSlice.* FROM %s WHERE sst==$NetworkSlice.sst AND sd==$NetworkSlice.sd"
	getNetworkSliceBySstNullStmt = "SELECT &NetworkSlice.* FROM %s WHERE sst==$NetworkSlice.sst AND sd IS NULL"
	getNetworkSliceByNameStmt    = "SELECT &NetworkSlice.* FROM %s WHERE name==$NetworkSlice.name"
	createNetworkSliceStmt       = "INSERT INTO %s (sst, sd, name) VALUES ($NetworkSlice.sst, $NetworkSlice.sd, $NetworkSlice.name)"
	updateNetworkSliceStmt       = "UPDATE %s SET sst=$NetworkSlice.sst, sd=$NetworkSlice.sd, name=$NetworkSlice.name WHERE id==$NetworkSlice.id"
	deleteNetworkSliceStmt       = "DELETE FROM %s WHERE id==$NetworkSlice.id"
	countNetworkSlicesStmt       = "SELECT COUNT(*) AS &NumItems.count FROM %s"
)

type NetworkSlice struct {
	ID   int     `db:"id"`
	Sst  int32   `db:"sst"`
	Sd   *string `db:"sd"` // hex string, nullable
	Name string  `db:"name"`
}

func (db *Database) ListNetworkSlices(ctx context.Context) ([]NetworkSlice, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "SELECT", NetworkSlicesTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			attribute.String("db.collection", NetworkSlicesTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(NetworkSlicesTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(NetworkSlicesTableName, "select").Inc()

	var slices []NetworkSlice

	err := db.conn.Query(ctx, db.listNetworkSlicesStmt).GetAll(&slices)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			span.SetStatus(codes.Ok, "no rows")
			return nil, nil
		}

		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return nil, fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return slices, nil
}

func (db *Database) GetNetworkSliceByID(ctx context.Context, id int) (*NetworkSlice, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "SELECT", NetworkSlicesTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			attribute.String("db.collection", NetworkSlicesTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(NetworkSlicesTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(NetworkSlicesTableName, "select").Inc()

	row := NetworkSlice{ID: id}

	err := db.conn.Query(ctx, db.getNetworkSliceByIDStmt, row).Get(&row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			span.SetStatus(codes.Ok, "no rows")
			return nil, ErrNotFound
		}

		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return nil, fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return &row, nil
}

func (db *Database) GetNetworkSliceBySstSd(ctx context.Context, sst int32, sd *string) (*NetworkSlice, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s (by sst/sd)", "SELECT", NetworkSlicesTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			attribute.String("db.collection", NetworkSlicesTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(NetworkSlicesTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(NetworkSlicesTableName, "select").Inc()

	var row NetworkSlice

	var err error

	if sd == nil {
		row = NetworkSlice{Sst: sst}

		err = db.conn.Query(ctx, db.getNetworkSliceBySstNullStmt, row).Get(&row)
	} else {
		row = NetworkSlice{Sst: sst, Sd: sd}

		err = db.conn.Query(ctx, db.getNetworkSliceBySstSdStmt, row).Get(&row)
	}

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			span.SetStatus(codes.Ok, "no rows")
			return nil, ErrNotFound
		}

		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return nil, fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return &row, nil
}

func (db *Database) GetNetworkSliceByName(ctx context.Context, name string) (*NetworkSlice, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s (by name)", "SELECT", NetworkSlicesTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			attribute.String("db.collection", NetworkSlicesTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(NetworkSlicesTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(NetworkSlicesTableName, "select").Inc()

	row := NetworkSlice{Name: name}

	err := db.conn.Query(ctx, db.getNetworkSliceByNameStmt, row).Get(&row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			span.SetStatus(codes.Ok, "no rows")
			return nil, ErrNotFound
		}

		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return nil, fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return &row, nil
}

func (db *Database) CreateNetworkSlice(ctx context.Context, ns *NetworkSlice) error {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "INSERT", NetworkSlicesTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("INSERT"),
			attribute.String("db.collection", NetworkSlicesTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(NetworkSlicesTableName, "insert"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(NetworkSlicesTableName, "insert").Inc()

	err := db.conn.Query(ctx, db.createNetworkSliceStmt, ns).Run()
	if err != nil {
		if isUniqueNameError(err) {
			span.RecordError(ErrAlreadyExists)
			span.SetStatus(codes.Error, "unique constraint failed")

			return ErrAlreadyExists
		}

		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

func (db *Database) UpdateNetworkSlice(ctx context.Context, ns *NetworkSlice) error {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "UPDATE", NetworkSlicesTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("UPDATE"),
			attribute.String("db.collection", NetworkSlicesTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(NetworkSlicesTableName, "update"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(NetworkSlicesTableName, "update").Inc()

	var outcome sqlair.Outcome

	err := db.conn.Query(ctx, db.updateNetworkSliceStmt, ns).Get(&outcome)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return fmt.Errorf("query failed: %w", err)
	}

	rowsAffected, err := outcome.Result().RowsAffected()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "retrieving rows affected failed")

		return fmt.Errorf("retrieving rows affected failed: %w", err)
	}

	if rowsAffected == 0 {
		span.RecordError(ErrNotFound)
		span.SetStatus(codes.Error, "not found")

		return ErrNotFound
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

func (db *Database) DeleteNetworkSlice(ctx context.Context, id int) error {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "DELETE", NetworkSlicesTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("DELETE"),
			attribute.String("db.collection", NetworkSlicesTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(NetworkSlicesTableName, "delete"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(NetworkSlicesTableName, "delete").Inc()

	var outcome sqlair.Outcome

	err := db.conn.Query(ctx, db.deleteNetworkSliceStmt, NetworkSlice{ID: id}).Get(&outcome)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return fmt.Errorf("query failed: %w", err)
	}

	rowsAffected, err := outcome.Result().RowsAffected()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "retrieving rows affected failed")

		return fmt.Errorf("retrieving rows affected failed: %w", err)
	}

	if rowsAffected == 0 {
		span.RecordError(ErrNotFound)
		span.SetStatus(codes.Error, "not found")

		return ErrNotFound
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

func (db *Database) CountNetworkSlices(ctx context.Context) (int, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "SELECT", NetworkSlicesTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			attribute.String("db.collection", NetworkSlicesTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(NetworkSlicesTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(NetworkSlicesTableName, "select").Inc()

	var result NumItems

	err := db.conn.Query(ctx, db.countNetworkSlicesStmt).Get(&result)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return 0, fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return result.Count, nil
}
