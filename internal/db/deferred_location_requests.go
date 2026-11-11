// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/dbwriter"
	"github.com/google/uuid"
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
	insertDeferredLocationRequestStmt = `INSERT INTO ` + DeferredLocationRequestsTableName + ` (id, supi, ldr_type, method, mode, measurement_quantities, reporting_interval_s, reporting_amount, reporting_infinite, scheduled_loc_time, status, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	getDeferredLocationRequestStmt    = `SELECT id, supi, ldr_type, method, mode, measurement_quantities, reporting_interval_s, reporting_amount, reporting_infinite, scheduled_loc_time, status, termination_cause, termination_reason, created_at, terminated_at, COALESCE(p.last_seq, 0) FROM ` + DeferredLocationRequestsTableName + ` LEFT JOIN (SELECT request_id, MAX(seq) AS last_seq FROM ` + LocationReportsTableName + ` GROUP BY request_id) p ON p.request_id = id WHERE id = ?`
	terminateDeferredLocationStmt     = `UPDATE ` + DeferredLocationRequestsTableName + ` SET status = 'TERMINATED', termination_cause = ?, termination_reason = ?, terminated_at = ? WHERE id = ? AND status = 'ACTIVE'`
	terminateAllDeferredLocationStmt  = `UPDATE ` + DeferredLocationRequestsTableName + ` SET status = 'TERMINATED', termination_cause = ?, termination_reason = ?, terminated_at = ? WHERE status = 'ACTIVE'`
	deleteDeferredLocationRequestStmt = `DELETE FROM ` + DeferredLocationRequestsTableName + ` WHERE id = ?`
	deleteEndedDeferredLocationStmt   = `DELETE FROM ` + DeferredLocationRequestsTableName + ` WHERE status = 'TERMINATED' AND terminated_at < ?`
	deleteOrphanLocationReportsStmt   = `DELETE FROM ` + LocationReportsTableName + ` WHERE request_id NOT IN (SELECT id FROM ` + DeferredLocationRequestsTableName + `)`
)

const listDeferredLocationRequestsStmt = `
  SELECT id, supi, ldr_type, method, mode, measurement_quantities, reporting_interval_s, reporting_amount, reporting_infinite, scheduled_loc_time, status, termination_cause, termination_reason, created_at, terminated_at, COALESCE(p.last_seq, 0), COUNT(*) OVER()
  FROM ` + DeferredLocationRequestsTableName + `
  LEFT JOIN (SELECT request_id, MAX(seq) AS last_seq FROM ` + LocationReportsTableName + ` GROUP BY request_id) p ON p.request_id = id
  WHERE
    (?1 IS NULL OR supi = ?1)
    AND (?2 IS NULL OR status = ?2)
  ORDER BY created_at DESC, id DESC
  LIMIT ?3
  OFFSET ?4
`

const countDeferredLocationRequestsStmt = `
  SELECT COUNT(*)
  FROM ` + DeferredLocationRequestsTableName + `
  WHERE
    (?1 IS NULL OR supi = ?1)
    AND (?2 IS NULL OR status = ?2)
`

type DeferredLocationRequest struct {
	ID                    string
	SUPI                  string
	LdrType               string
	Method                string
	Mode                  *string
	MeasurementQuantities *string
	ReportingIntervalS    *int64
	ReportingAmount       *int64
	ReportingInfinite     bool
	ScheduledLocTime      *int64
	Status                string
	TerminationCause      *string
	TerminationReason     *string
	CreatedAt             int64
	TerminatedAt          *int64
	LastReportSeq         int64
}

type DeferredLocationRequestFilters struct {
	SUPI   *string
	Status *string
}

type DeferredLocationTermination struct {
	Cause  string
	Reason *string
	At     time.Time
}

func (db *Database) CreateDeferredLocationRequest(ctx context.Context, r *DeferredLocationRequest) error {
	ctx, span, timer := startDBQuery(ctx, DeferredLocationRequestsTableName, "INSERT", "insert")
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

	if _, err := db.writer().ExecContext(ctx, insertDeferredLocationRequestStmt, r.ID, r.SUPI, r.LdrType, r.Method, r.Mode, r.MeasurementQuantities, r.ReportingIntervalS, r.ReportingAmount, r.ReportingInfinite, r.ScheduledLocTime, r.Status, r.CreatedAt); err != nil {
		if isUniqueNameError(err) {
			return ErrAlreadyExists
		}

		recordSpanError(span, err)

		return fmt.Errorf("query failed: %w", err)
	}

	return nil
}

func (db *Database) GetDeferredLocationRequest(ctx context.Context, id string) (*DeferredLocationRequest, error) {
	ctx, span, timer := startDBQuery(ctx, DeferredLocationRequestsTableName, "SELECT", "select")
	defer span.End()
	defer timer.ObserveDuration()

	if !db.appliedSchemaAtLeast(ctx, deferredLocationSchema) {
		return nil, ErrNotFound
	}

	r, err := scanDeferredLocationRequest(db.reader().QueryRowContext(ctx, getDeferredLocationRequestStmt, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return &r, nil
}

func (db *Database) ListDeferredLocationRequests(ctx context.Context, page, perPage int, filters *DeferredLocationRequestFilters) ([]DeferredLocationRequest, int, error) {
	ctx, span, timer := startDBQuery(ctx, DeferredLocationRequestsTableName, "SELECT", "select")
	defer span.End()
	defer timer.ObserveDuration()

	if !db.appliedSchemaAtLeast(ctx, deferredLocationSchema) {
		return nil, 0, nil
	}

	if filters == nil {
		filters = &DeferredLocationRequestFilters{}
	}

	requests, total, err := queryPage(ctx, db.reader(), listDeferredLocationRequestsStmt, scanDeferredLocationRequest, filters.SUPI, filters.Status, perPage, (page-1)*perPage)
	if err != nil {
		recordSpanError(span, err)

		return nil, 0, fmt.Errorf("query failed: %w", err)
	}

	if len(requests) == 0 {
		count, err := queryCount(ctx, db.reader(), countDeferredLocationRequestsStmt, filters.SUPI, filters.Status)
		if err != nil {
			recordSpanError(span, err)

			return nil, 0, fmt.Errorf("count failed: %w", err)
		}

		return nil, count, nil
	}

	return requests, total, nil
}

func (db *Database) TerminateDeferredLocationRequest(ctx context.Context, id string, t DeferredLocationTermination) error {
	ctx, span, timer := startDBQuery(ctx, DeferredLocationRequestsTableName, "UPDATE", "update")
	defer span.End()
	defer timer.ObserveDuration()

	if err := db.checkOpSchema(deferredLocationSchema); err != nil {
		recordSpanError(span, err)

		return err
	}

	n, err := db.execCounting(ctx, terminateDeferredLocationStmt, t.Cause, t.Reason, dbwriter.EpochMillis(t.At), id)
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
	ctx, span, timer := startDBQuery(ctx, DeferredLocationRequestsTableName, "UPDATE", "update")
	defer span.End()
	defer timer.ObserveDuration()

	if !db.appliedSchemaAtLeast(ctx, deferredLocationSchema) {
		return 0, nil
	}

	n, err := db.execCounting(ctx, terminateAllDeferredLocationStmt, t.Cause, t.Reason, dbwriter.EpochMillis(t.At))
	if err != nil {
		recordSpanError(span, err)

		return 0, err
	}

	return n, nil
}

func (db *Database) DeleteDeferredLocationRequest(ctx context.Context, id string) error {
	ctx, span, timer := startDBQuery(ctx, DeferredLocationRequestsTableName, "DELETE", "delete")
	defer span.End()
	defer timer.ObserveDuration()

	if !db.appliedSchemaAtLeast(ctx, deferredLocationSchema) {
		return ErrNotFound
	}

	n, err := db.execCounting(ctx, deleteDeferredLocationRequestStmt, id)
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
	ctx, span, timer := startDBQuery(ctx, DeferredLocationRequestsTableName, "DELETE", "delete")
	defer span.End()
	defer timer.ObserveDuration()

	if !db.appliedSchemaAtLeast(ctx, deferredLocationSchema) {
		return nil
	}

	if _, err := db.writer().ExecContext(ctx, deleteEndedDeferredLocationStmt, dbwriter.EpochMillis(endedBefore)); err != nil {
		recordSpanError(span, err)

		return fmt.Errorf("query failed: %w", err)
	}

	if _, err := db.writer().ExecContext(ctx, deleteOrphanLocationReportsStmt); err != nil {
		recordSpanError(span, err)

		return fmt.Errorf("delete orphan reports: %w", err)
	}

	return nil
}

func (db *Database) execCounting(ctx context.Context, query string, args ...any) (int, error) {
	res, err := db.writer().ExecContext(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("query failed: %w", err)
	}

	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("rows affected: %w", err)
	}

	return int(n), nil
}

func scanDeferredLocationRequest(row scanner) (DeferredLocationRequest, error) {
	var v DeferredLocationRequest

	if err := row.Scan(&v.ID, &v.SUPI, &v.LdrType, &v.Method, &v.Mode, &v.MeasurementQuantities, &v.ReportingIntervalS, &v.ReportingAmount, &v.ReportingInfinite, &v.ScheduledLocTime, &v.Status, &v.TerminationCause, &v.TerminationReason, &v.CreatedAt, &v.TerminatedAt, &v.LastReportSeq); err != nil {
		return DeferredLocationRequest{}, err
	}

	return v, nil
}
