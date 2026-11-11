// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"fmt"
)

const (
	insertLocationReportStmt = `INSERT INTO ` + LocationReportsTableName + ` (request_id, seq, reported_at, location_data) VALUES (?, ?, ?, ?)`
	pruneLocationReportsStmt = `DELETE FROM ` + LocationReportsTableName + ` WHERE request_id = ? AND seq <= ?`
	listLocationReportsStmt  = `SELECT request_id, seq, reported_at, location_data FROM ` + LocationReportsTableName + ` WHERE request_id = ? AND seq > ? ORDER BY seq LIMIT ?`
)

type LocationReport struct {
	RequestID    string
	Seq          int64
	ReportedAt   int64
	LocationData string
}

func (db *Database) AddLocationReport(ctx context.Context, r *LocationReport, keep int64) error {
	ctx, span, timer := startDBQuery(ctx, LocationReportsTableName, "INSERT", "insert")
	defer span.End()
	defer timer.ObserveDuration()

	if err := db.checkOpSchema(deferredLocationSchema); err != nil {
		recordSpanError(span, err)

		return err
	}

	tx, err := db.writer().BeginTx(ctx, nil)
	if err != nil {
		recordSpanError(span, err)

		return fmt.Errorf("begin transaction failed: %w", err)
	}

	if _, err := tx.ExecContext(ctx, insertLocationReportStmt, r.RequestID, r.Seq, r.ReportedAt, r.LocationData); err != nil {
		_ = tx.Rollback()

		recordSpanError(span, err)

		return fmt.Errorf("insert failed: %w", err)
	}

	if r.Seq > keep {
		if _, err := tx.ExecContext(ctx, pruneLocationReportsStmt, r.RequestID, r.Seq-keep); err != nil {
			_ = tx.Rollback()

			recordSpanError(span, err)

			return fmt.Errorf("prune failed: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		recordSpanError(span, err)

		return fmt.Errorf("commit failed: %w", err)
	}

	return nil
}

func (db *Database) ListLocationReports(ctx context.Context, requestID string, afterSeq int64, limit int) ([]LocationReport, error) {
	ctx, span, timer := startDBQuery(ctx, LocationReportsTableName, "SELECT", "select")
	defer span.End()
	defer timer.ObserveDuration()

	if !db.appliedSchemaAtLeast(ctx, deferredLocationSchema) {
		return nil, nil
	}

	reports, err := queryAll(ctx, db.reader(), listLocationReportsStmt, scanLocationReport, requestID, afterSeq, limit)
	if err != nil {
		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return reports, nil
}

func scanLocationReport(row scanner) (LocationReport, error) {
	var v LocationReport

	if err := row.Scan(&v.RequestID, &v.Seq, &v.ReportedAt, &v.LocationData); err != nil {
		return LocationReport{}, err
	}

	return v, nil
}
