// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

const (
	insertLocationReportStmt = `INSERT INTO %s (request_id, seq, reported_at, location_data) VALUES ($LocationReport.request_id, $LocationReport.seq, $LocationReport.reported_at, $LocationReport.location_data)`
	pruneLocationReportsStmt = `DELETE FROM %s WHERE request_id = $locationReportCursor.request_id AND seq <= $locationReportCursor.seq`
	listLocationReportsStmt  = `SELECT &LocationReport.* FROM %s WHERE request_id = $locationReportCursor.request_id AND seq > $locationReportCursor.seq ORDER BY seq LIMIT $locationReportCursor.limit`
)

type LocationReport struct {
	RequestID    string `db:"request_id"`
	Seq          int64  `db:"seq"`
	ReportedAt   int64  `db:"reported_at"`
	LocationData string `db:"location_data"`
}

type locationReportCursor struct {
	RequestID string `db:"request_id"`
	Seq       int64  `db:"seq"`
	Limit     int    `db:"limit"`
}

func (db *Database) AddLocationReport(ctx context.Context, r *LocationReport, keep int64) error {
	ctx, span, timer := startLocationQuery(ctx, LocationReportsTableName, "INSERT", "insert")
	defer span.End()
	defer timer.ObserveDuration()

	if err := db.checkOpSchema(deferredLocationSchema); err != nil {
		recordSpanError(span, err)

		return err
	}

	tx, err := db.conn().Begin(ctx, nil)
	if err != nil {
		recordSpanError(span, err)

		return fmt.Errorf("begin transaction failed: %w", err)
	}

	if err := tx.Query(ctx, db.insertLocationReportStmt, r).Run(); err != nil {
		_ = tx.Rollback()

		recordSpanError(span, err)

		return fmt.Errorf("insert failed: %w", err)
	}

	if r.Seq > keep {
		if err := tx.Query(ctx, db.pruneLocationReportsStmt, locationReportCursor{RequestID: r.RequestID, Seq: r.Seq - keep}).Run(); err != nil {
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
	ctx, span, timer := startLocationQuery(ctx, LocationReportsTableName, "SELECT", "select")
	defer span.End()
	defer timer.ObserveDuration()

	if !db.appliedSchemaAtLeast(ctx, deferredLocationSchema) {
		return nil, nil
	}

	var reports []LocationReport

	cursor := locationReportCursor{RequestID: requestID, Seq: afterSeq, Limit: limit}

	if err := db.conn().Query(ctx, db.listLocationReportsStmt, cursor).GetAll(&reports); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}

		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return reports, nil
}
