// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// SPDX-FileCopyrightText: Ella Networks Inc.

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
)

const NetworkSlicesTableName = "network_slices"

const (
	listNetworkSlicesPagedSQL = "SELECT id, sst, sd, name, COUNT(*) OVER() FROM " + NetworkSlicesTableName + " LIMIT ? OFFSET ?"
	listAllNetworkSlicesSQL   = "SELECT id, sst, sd, name FROM " + NetworkSlicesTableName + " ORDER BY id ASC"
	getNetworkSliceSQL        = "SELECT id, sst, sd, name FROM " + NetworkSlicesTableName + " WHERE name==?"
	getNetworkSliceByIDSQL    = "SELECT id, sst, sd, name FROM " + NetworkSlicesTableName + " WHERE id==?"
	createNetworkSliceSQL     = "INSERT INTO " + NetworkSlicesTableName + " (id, sst, sd, name) VALUES (?, ?, ?, ?)"
	editNetworkSliceSQL       = "UPDATE " + NetworkSlicesTableName + " SET sst=?, sd=? WHERE name==?"
	deleteNetworkSliceSQL     = "DELETE FROM " + NetworkSlicesTableName + " WHERE name==?"
	countNetworkSlicesSQL     = "SELECT COUNT(*) FROM " + NetworkSlicesTableName
	listNetworkSlicesByIDsSQL = "SELECT id, sst, sd, name FROM " + NetworkSlicesTableName + " WHERE id IN (SELECT value FROM json_each(?))"
)

type NetworkSlice struct {
	ID   string // UUIDv7
	Sst  int32
	Sd   *string
	Name string
}

func (db *Database) ListNetworkSlicesPage(ctx context.Context, page, perPage int) ([]NetworkSlice, int, error) {
	ctx, span, timer := startDBQueryNamed(ctx, "SELECT "+NetworkSlicesTableName+" (paged)", NetworkSlicesTableName, "SELECT", "select", attribute.Int("ella.db.page", page), attribute.Int("ella.db.page_size", perPage))
	defer span.End()
	defer timer.ObserveDuration()

	slices, count, err := queryPageWithTotal(ctx, db.reader(), listNetworkSlicesPagedSQL, scanNetworkSlice, perPage, (page-1)*perPage)
	if err != nil {
		recordSpanError(span, err)

		return nil, 0, fmt.Errorf("query failed: %w", err)
	}

	if len(slices) == 0 {
		fallbackCount, err := db.CountNetworkSlices(ctx)
		if err != nil {
			recordSpanError(span, err)

			return nil, 0, fmt.Errorf("count failed: %w", err)
		}

		return nil, fallbackCount, nil
	}

	return slices, count, nil
}

func (db *Database) ListAllNetworkSlices(ctx context.Context) ([]NetworkSlice, error) {
	ctx, span, timer := startDBQuery(ctx, NetworkSlicesTableName, "SELECT", "select")
	defer span.End()
	defer timer.ObserveDuration()

	slices, err := queryAll(ctx, db.reader(), listAllNetworkSlicesSQL, scanNetworkSlice)
	if err != nil {
		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return slices, nil
}

func (db *Database) GetNetworkSlice(ctx context.Context, name string) (*NetworkSlice, error) {
	ctx, span, timer := startDBQuery(ctx, NetworkSlicesTableName, "SELECT", "select")
	defer span.End()
	defer timer.ObserveDuration()

	row, err := scanNetworkSlice(db.reader().QueryRowContext(ctx, getNetworkSliceSQL, name))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return &row, nil
}

func (db *Database) GetNetworkSliceByID(ctx context.Context, id string) (*NetworkSlice, error) {
	ctx, span, timer := startDBQuery(ctx, NetworkSlicesTableName, "SELECT", "select")
	defer span.End()
	defer timer.ObserveDuration()

	row, err := scanNetworkSlice(db.reader().QueryRowContext(ctx, getNetworkSliceByIDSQL, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return &row, nil
}

func (db *Database) CreateNetworkSlice(ctx context.Context, slice *NetworkSlice) error {
	ctx, span, timer := startDBQuery(ctx, NetworkSlicesTableName, "INSERT", "insert")
	defer span.End()
	defer timer.ObserveDuration()

	if slice.ID == "" {
		id, err := uuid.NewV7()
		if err != nil {
			recordSpanError(span, err)

			return fmt.Errorf("generate network slice id: %w", err)
		}

		slice.ID = id.String()
	}

	_, err := opCreateNetworkSlice.Invoke(ctx, db, slice)
	if err != nil {
		recordSpanError(span, err)

		return err
	}

	return nil
}

func (db *Database) UpdateNetworkSlice(ctx context.Context, slice *NetworkSlice) error {
	ctx, span, timer := startDBQuery(ctx, NetworkSlicesTableName, "UPDATE", "update")
	defer span.End()
	defer timer.ObserveDuration()

	_, err := opUpdateNetworkSlice.Invoke(ctx, db, slice)
	if err != nil {
		recordSpanError(span, err)

		return err
	}

	return nil
}

func (db *Database) DeleteNetworkSlice(ctx context.Context, name string) error {
	ctx, span, timer := startDBQuery(ctx, NetworkSlicesTableName, "DELETE", "delete")
	defer span.End()
	defer timer.ObserveDuration()

	_, err := opDeleteNetworkSlice.Invoke(ctx, db, &stringPayload{Value: name})
	if err != nil {
		recordSpanError(span, err)

		return err
	}

	return nil
}

func (db *Database) CountNetworkSlices(ctx context.Context) (int, error) {
	ctx, span, timer := startDBQuery(ctx, NetworkSlicesTableName, "SELECT", "select")
	defer span.End()
	defer timer.ObserveDuration()

	count, err := queryCount(ctx, db.reader(), countNetworkSlicesSQL)
	if err != nil {
		recordSpanError(span, err)

		return 0, fmt.Errorf("query failed: %w", err)
	}

	return count, nil
}

func (db *Database) ListNetworkSlicesByIDs(ctx context.Context, ids []string) ([]NetworkSlice, error) {
	querySummary := fmt.Sprintf("%s %s (by IDs)", "SELECT", NetworkSlicesTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			semconv.DBCollectionName(NetworkSlicesTableName),
		),
	)
	defer span.End()

	if len(ids) == 0 {
		return nil, nil
	}

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(NetworkSlicesTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(NetworkSlicesTableName, "select").Inc()

	slices, err := queryAll(ctx, db.reader(), listNetworkSlicesByIDsSQL, scanNetworkSlice, jsonArray(ids))
	if err != nil {
		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return slices, nil
}

func scanNetworkSlice(row scanner) (NetworkSlice, error) {
	var v NetworkSlice

	if err := row.Scan(&v.ID, &v.Sst, &v.Sd, &v.Name); err != nil {
		return NetworkSlice{}, err
	}

	return v, nil
}
