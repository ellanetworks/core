// Copyright 2024 Ella Networks

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
	listNetworkSlicesPagedStmt = "SELECT &NetworkSlice.*, COUNT(*) OVER() AS &NumItems.count FROM %s LIMIT $ListArgs.limit OFFSET $ListArgs.offset"
	listAllNetworkSlicesStmt   = "SELECT &NetworkSlice.* FROM %s ORDER BY id ASC"
	getNetworkSliceStmt        = "SELECT &NetworkSlice.* FROM %s WHERE name==$NetworkSlice.name"
	getNetworkSliceByIDStmt    = "SELECT &NetworkSlice.* FROM %s WHERE id==$NetworkSlice.id"
	createNetworkSliceStmt     = "INSERT INTO %s (sst, sd, name) VALUES ($NetworkSlice.sst, $NetworkSlice.sd, $NetworkSlice.name)"
	editNetworkSliceStmt       = "UPDATE %s SET sst=$NetworkSlice.sst, sd=$NetworkSlice.sd WHERE name==$NetworkSlice.name"
	deleteNetworkSliceStmt     = "DELETE FROM %s WHERE name==$NetworkSlice.name"
	countNetworkSlicesStmt     = "SELECT COUNT(*) AS &NumItems.count FROM %s"
	listNetworkSlicesByIDsStmt = "SELECT &NetworkSlice.* FROM %s WHERE id IN ($SliceIDs[:])"
)

type NetworkSlice struct {
	ID   int     `db:"id"`
	Sst  int32   `db:"sst"`
	Sd   *string `db:"sd"`
	Name string  `db:"name"`
}

func (db *Database) ListNetworkSlicesPage(ctx context.Context, page, perPage int) ([]NetworkSlice, int, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s (paged)", "SELECT", NetworkSlicesTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			attribute.String("db.collection", NetworkSlicesTableName),
			attribute.Int("page", page),
			attribute.Int("per_page", perPage),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(NetworkSlicesTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(NetworkSlicesTableName, "select").Inc()

	var slices []NetworkSlice

	var counts []NumItems

	args := ListArgs{
		Limit:  perPage,
		Offset: (page - 1) * perPage,
	}

	err := db.shared.Query(ctx, db.listNetworkSlicesStmt, args).GetAll(&slices, &counts)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			span.SetStatus(codes.Ok, "no rows")

			fallbackCount, countErr := db.CountNetworkSlices(ctx)
			if countErr != nil {
				return nil, 0, nil
			}

			return nil, fallbackCount, nil
		}

		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return nil, 0, fmt.Errorf("query failed: %w", err)
	}

	count := 0
	if len(counts) > 0 {
		count = counts[0].Count
	}

	span.SetStatus(codes.Ok, "")

	return slices, count, nil
}

func (db *Database) ListAllNetworkSlices(ctx context.Context) ([]NetworkSlice, error) {
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

	err := db.shared.Query(ctx, db.listAllNetworkSlicesStmt).GetAll(&slices)
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

func (db *Database) GetNetworkSlice(ctx context.Context, name string) (*NetworkSlice, error) {
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

	row := NetworkSlice{Name: name}

	err := db.shared.Query(ctx, db.getNetworkSliceStmt, row).Get(&row)
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

	err := db.shared.Query(ctx, db.getNetworkSliceByIDStmt, row).Get(&row)
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

func (db *Database) CreateNetworkSlice(ctx context.Context, slice *NetworkSlice) error {
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

	err := db.shared.Query(ctx, db.createNetworkSliceStmt, slice).Run()
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

func (db *Database) UpdateNetworkSlice(ctx context.Context, slice *NetworkSlice) error {
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

	err := db.shared.Query(ctx, db.editNetworkSliceStmt, slice).Get(&outcome)
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

func (db *Database) DeleteNetworkSlice(ctx context.Context, name string) error {
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

	err := db.shared.Query(ctx, db.deleteNetworkSliceStmt, NetworkSlice{Name: name}).Get(&outcome)
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

	err := db.shared.Query(ctx, db.countNetworkSlicesStmt).Get(&result)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return 0, fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return result.Count, nil
}

func (db *Database) ListNetworkSlicesByIDs(ctx context.Context, ids []int) ([]NetworkSlice, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s (by IDs)", "SELECT", NetworkSlicesTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			attribute.String("db.collection", NetworkSlicesTableName),
		),
	)
	defer span.End()

	if len(ids) == 0 {
		span.SetStatus(codes.Ok, "empty input")
		return nil, nil
	}

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(NetworkSlicesTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(NetworkSlicesTableName, "select").Inc()

	var slices []NetworkSlice

	err := db.shared.Query(ctx, db.listNetworkSlicesByIDsStmt, SliceIDs(ids)).GetAll(&slices)
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
