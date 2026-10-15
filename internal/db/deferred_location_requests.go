// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/canonical/sqlair"
	"github.com/ellanetworks/core/internal/dbwriter"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
)

const (
	DeferredLocationRequestsTableName = "deferred_location_requests"
	LocationReportsTableName          = "location_reports"
)

const deferredLocationSchema = 24

const (
	DeferredLocationRetention       = 7 * 24 * time.Hour
	LocationReportsKept       int64 = 10000
)

const (
	DeferredLocationRequestActive     = "ACTIVE"
	DeferredLocationRequestTerminated = "TERMINATED"
)

const (
	insertDeferredLocationRequestStmt = `INSERT INTO %s (id, supi, method, mode, measurement_quantities, reporting_interval_s, reporting_amount, reporting_infinite, scheduled_loc_time, status, created_at) VALUES ($DeferredLocationRequest.id, $DeferredLocationRequest.supi, $DeferredLocationRequest.method, $DeferredLocationRequest.mode, $DeferredLocationRequest.measurement_quantities, $DeferredLocationRequest.reporting_interval_s, $DeferredLocationRequest.reporting_amount, $DeferredLocationRequest.reporting_infinite, $DeferredLocationRequest.scheduled_loc_time, $DeferredLocationRequest.status, $DeferredLocationRequest.created_at)`
	getDeferredLocationRequestStmt    = `SELECT &DeferredLocationRequest.* FROM %s WHERE id = $DeferredLocationRequest.id`
	terminateDeferredLocationStmt     = `UPDATE %s SET status = 'TERMINATED', termination_cause = $deferredLocationTermination.termination_cause, termination_reason = $deferredLocationTermination.termination_reason, terminated_at = $deferredLocationTermination.terminated_at WHERE id = $deferredLocationTermination.id AND status = 'ACTIVE'`
	terminateAllDeferredLocationStmt  = `UPDATE %s SET status = 'TERMINATED', termination_cause = $deferredLocationTermination.termination_cause, termination_reason = $deferredLocationTermination.termination_reason, terminated_at = $deferredLocationTermination.terminated_at WHERE status = 'ACTIVE'`
	deleteDeferredLocationRequestStmt = `DELETE FROM %s WHERE id = $DeferredLocationRequest.id`
	deleteEndedDeferredLocationStmt   = `DELETE FROM %s WHERE status = 'TERMINATED' AND terminated_at < $cutoffArgs.cutoff`
)

const listDeferredLocationRequestsStmt = `
  SELECT &DeferredLocationRequest.*, COUNT(*) OVER() AS &NumItems.count
  FROM %s
  WHERE
    ($DeferredLocationRequestFilters.supi IS NULL OR supi = $DeferredLocationRequestFilters.supi)
    AND ($DeferredLocationRequestFilters.status IS NULL OR status = $DeferredLocationRequestFilters.status)
  ORDER BY created_at DESC, id DESC
  LIMIT $ListArgs.limit
  OFFSET $ListArgs.offset
`

const countDeferredLocationRequestsStmt = `
  SELECT COUNT(*) AS &NumItems.count
  FROM %s
  WHERE
    ($DeferredLocationRequestFilters.supi IS NULL OR supi = $DeferredLocationRequestFilters.supi)
    AND ($DeferredLocationRequestFilters.status IS NULL OR status = $DeferredLocationRequestFilters.status)
`

type DeferredLocationRequest struct {
	ID                    string  `db:"id"`
	SUPI                  string  `db:"supi"`
	Method                string  `db:"method"`
	Mode                  *string `db:"mode"`
	MeasurementQuantities *string `db:"measurement_quantities"`
	ReportingIntervalS    int64   `db:"reporting_interval_s"`
	ReportingAmount       int64   `db:"reporting_amount"`
	ReportingInfinite     bool    `db:"reporting_infinite"`
	ScheduledLocTime      *int64  `db:"scheduled_loc_time"`
	Status                string  `db:"status"`
	TerminationCause      *string `db:"termination_cause"`
	TerminationReason     *string `db:"termination_reason"`
	CreatedAt             int64   `db:"created_at"`
	TerminatedAt          *int64  `db:"terminated_at"`
}

type DeferredLocationRequestFilters struct {
	SUPI   *string `db:"supi"`
	Status *string `db:"status"`
}

type DeferredLocationTermination struct {
	Cause  string
	Reason *string
	At     time.Time
}

type deferredLocationTermination struct {
	ID                string  `db:"id"`
	TerminationCause  string  `db:"termination_cause"`
	TerminationReason *string `db:"termination_reason"`
	TerminatedAt      int64   `db:"terminated_at"`
}

func startLocationQuery(ctx context.Context, table, operation, metric string) (context.Context, trace.Span, *prometheus.Timer) {
	querySummary := fmt.Sprintf("%s %s", operation, table)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName(operation),
			semconv.DBCollectionName(table),
		),
	)

	DBQueriesTotal.WithLabelValues(table, metric).Inc()

	return ctx, span, prometheus.NewTimer(DBQueryDuration.WithLabelValues(table, metric))
}

func (db *Database) CreateDeferredLocationRequest(ctx context.Context, r *DeferredLocationRequest) error {
	ctx, span, timer := startLocationQuery(ctx, DeferredLocationRequestsTableName, "INSERT", "insert")
	defer span.End()
	defer timer.ObserveDuration()

	if err := db.checkOpSchema(deferredLocationSchema); err != nil {
		recordSpanError(span, err)

		return err
	}

	id, err := uuid.NewV7()
	if err != nil {
		recordSpanError(span, err)

		return fmt.Errorf("generate id: %w", err)
	}

	r.ID = id.String()
	r.Status = DeferredLocationRequestActive
	r.TerminationCause = nil
	r.TerminationReason = nil
	r.TerminatedAt = nil
	r.CreatedAt = dbwriter.EpochMillis(time.Now())

	if err := db.conn().Query(ctx, db.insertDeferredLocationRequestStmt, r).Run(); err != nil {
		recordSpanError(span, err)

		return fmt.Errorf("query failed: %w", err)
	}

	return nil
}

func (db *Database) GetDeferredLocationRequest(ctx context.Context, id string) (*DeferredLocationRequest, error) {
	ctx, span, timer := startLocationQuery(ctx, DeferredLocationRequestsTableName, "SELECT", "select")
	defer span.End()
	defer timer.ObserveDuration()

	if !db.appliedSchemaAtLeast(ctx, deferredLocationSchema) {
		return nil, ErrNotFound
	}

	var r DeferredLocationRequest

	if err := db.conn().Query(ctx, db.getDeferredLocationRequestStmt, DeferredLocationRequest{ID: id}).Get(&r); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return &r, nil
}

func (db *Database) ListDeferredLocationRequests(ctx context.Context, page, perPage int, filters *DeferredLocationRequestFilters) ([]DeferredLocationRequest, int, error) {
	ctx, span, timer := startLocationQuery(ctx, DeferredLocationRequestsTableName, "SELECT", "select")
	defer span.End()
	defer timer.ObserveDuration()

	if !db.appliedSchemaAtLeast(ctx, deferredLocationSchema) {
		return nil, 0, nil
	}

	if filters == nil {
		filters = &DeferredLocationRequestFilters{}
	}

	args := ListArgs{Limit: perPage, Offset: (page - 1) * perPage}

	var (
		requests []DeferredLocationRequest
		counts   []NumItems
	)

	if err := db.conn().Query(ctx, db.listDeferredLocationRequestsStmt, args, filters).GetAll(&requests, &counts); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			var total NumItems

			if err := db.conn().Query(ctx, db.countDeferredLocationRequestsStmt, filters).Get(&total); err != nil {
				recordSpanError(span, err)

				return nil, 0, fmt.Errorf("count failed: %w", err)
			}

			return nil, total.Count, nil
		}

		recordSpanError(span, err)

		return nil, 0, fmt.Errorf("query failed: %w", err)
	}

	total := 0
	if len(counts) > 0 {
		total = counts[0].Count
	}

	return requests, total, nil
}

func (db *Database) TerminateDeferredLocationRequest(ctx context.Context, id string, t DeferredLocationTermination) error {
	ctx, span, timer := startLocationQuery(ctx, DeferredLocationRequestsTableName, "UPDATE", "update")
	defer span.End()
	defer timer.ObserveDuration()

	if err := db.checkOpSchema(deferredLocationSchema); err != nil {
		recordSpanError(span, err)

		return err
	}

	n, err := db.execCounting(ctx, db.terminateDeferredLocationStmt, toTerminationRow(id, t))
	if err != nil {
		recordSpanError(span, err)

		return err
	}

	if n == 0 {
		return ErrNotFound
	}

	return nil
}

func (db *Database) TerminateActiveDeferredLocationRequests(ctx context.Context, t DeferredLocationTermination) (int, error) {
	ctx, span, timer := startLocationQuery(ctx, DeferredLocationRequestsTableName, "UPDATE", "update")
	defer span.End()
	defer timer.ObserveDuration()

	if !db.appliedSchemaAtLeast(ctx, deferredLocationSchema) {
		return 0, nil
	}

	n, err := db.execCounting(ctx, db.terminateAllDeferredLocationStmt, toTerminationRow("", t))
	if err != nil {
		recordSpanError(span, err)

		return 0, err
	}

	return n, nil
}

func (db *Database) DeleteDeferredLocationRequest(ctx context.Context, id string) error {
	ctx, span, timer := startLocationQuery(ctx, DeferredLocationRequestsTableName, "DELETE", "delete")
	defer span.End()
	defer timer.ObserveDuration()

	if !db.appliedSchemaAtLeast(ctx, deferredLocationSchema) {
		return ErrNotFound
	}

	n, err := db.execCounting(ctx, db.deleteDeferredLocationRequestStmt, DeferredLocationRequest{ID: id})
	if err != nil {
		recordSpanError(span, err)

		return err
	}

	if n == 0 {
		return ErrNotFound
	}

	return nil
}

func (db *Database) DeleteEndedDeferredLocationRequests(ctx context.Context, endedBefore time.Time) error {
	ctx, span, timer := startLocationQuery(ctx, DeferredLocationRequestsTableName, "DELETE", "delete")
	defer span.End()
	defer timer.ObserveDuration()

	if !db.appliedSchemaAtLeast(ctx, deferredLocationSchema) {
		return nil
	}

	if err := db.conn().Query(ctx, db.deleteEndedDeferredLocationStmt, cutoffArgs{Cutoff: dbwriter.EpochMillis(endedBefore)}).Run(); err != nil {
		recordSpanError(span, err)

		return fmt.Errorf("query failed: %w", err)
	}

	return nil
}

func (db *Database) execCounting(ctx context.Context, stmt *sqlair.Statement, args ...any) (int, error) {
	var outcome sqlair.Outcome

	if err := db.conn().Query(ctx, stmt, args...).Get(&outcome); err != nil {
		return 0, fmt.Errorf("query failed: %w", err)
	}

	n, err := outcome.Result().RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("rows affected: %w", err)
	}

	return int(n), nil
}

func toTerminationRow(id string, t DeferredLocationTermination) deferredLocationTermination {
	return deferredLocationTermination{
		ID:                id,
		TerminationCause:  t.Cause,
		TerminationReason: t.Reason,
		TerminatedAt:      dbwriter.EpochMillis(t.At),
	}
}
