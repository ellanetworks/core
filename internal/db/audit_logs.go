// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// SPDX-FileCopyrightText: Ella Networks Inc.

package db

import (
	"context"
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/dbwriter"
	"go.opentelemetry.io/otel/attribute"
)

const AuditLogsTableName = "audit_logs"

const (
	insertAuditLogSQL     = "INSERT INTO " + AuditLogsTableName + " (id, timestamp, level, actor, action, ip, details) VALUES (?, ?, ?, ?, ?, ?, ?)"
	deleteOldAuditLogsSQL = "DELETE FROM " + AuditLogsTableName + " WHERE timestamp < ?"
)

// id is a UUIDv7, so it's not strictly time-ordered (clock skew between
// generators); the secondary sort by id keeps the ordering stable when
// timestamps tie.
const listAuditLogsFilteredPageSQL = `
  SELECT id, timestamp, level, actor, action, ip, details, COUNT(*) OVER()
  FROM ` + AuditLogsTableName + `
  WHERE
    (?1 IS NULL OR actor = ?1)
    AND (?2 IS NULL OR action = ?2)
    AND (?3 IS NULL OR timestamp >= ?3)
    AND (?4 IS NULL OR timestamp < ?4)
  ORDER BY timestamp DESC, id DESC
  LIMIT ?5
  OFFSET ?6
`

// AuditLogFilters holds optional filters for audit log queries.
type AuditLogFilters struct {
	Actor         *string // exact match
	Action        *string // exact match
	TimestampFrom *int64  // epoch milliseconds, inclusive lower bound
	TimestampTo   *int64  // epoch milliseconds, exclusive upper bound
}

// InsertAuditLogJSON parses the zap JSON and inserts a structured row.
func (db *Database) InsertAuditLog(ctx context.Context, auditLog *dbwriter.AuditLog) (err error) {
	ctx, finish := startDBQuery(ctx, AuditLogsTableName, "INSERT")
	defer func() { finish(err) }()

	if auditLog.ID == "" {
		err := fmt.Errorf("InsertAuditLog: ID must be set by the caller")
		return err
	}

	_, err = db.writer().ExecContext(context.WithoutCancel(ctx), insertAuditLogSQL, auditLog.ID, auditLog.Timestamp, auditLog.Level, auditLog.Actor, auditLog.Action, auditLog.IP, auditLog.Details)
	if err != nil {
		return err
	}

	return nil
}

func (db *Database) ListAuditLogsPage(ctx context.Context, filters *AuditLogFilters, page, perPage int) (_ []dbwriter.AuditLog, _ int, err error) {
	if filters == nil {
		filters = &AuditLogFilters{}
	}

	ctx, finish := startDBQueryNamed(ctx, "SELECT "+AuditLogsTableName+" (paged)", AuditLogsTableName, "SELECT", attribute.Int("ella.db.page", page), attribute.Int("ella.db.page_size", perPage))
	defer func() { finish(err) }()

	logs, count, err := queryPageWithTotal(ctx, db.reader(), listAuditLogsFilteredPageSQL, scanAuditLog, filters.Actor, filters.Action, filters.TimestampFrom, filters.TimestampTo, perPage, (page-1)*perPage)
	if err != nil {
		return nil, 0, fmt.Errorf("query failed: %w", err)
	}

	return logs, count, nil
}

// DeleteOldAuditLogs removes logs older than the specified retention period in days.
func (db *Database) DeleteOldAuditLogs(ctx context.Context, days int) (err error) {
	ctx, finish := startDBQueryNamed(ctx, "DELETE "+AuditLogsTableName+" (retention)", AuditLogsTableName, "DELETE", attribute.Int("retention.days", days))
	defer func() { finish(err) }()

	// Compute cutoff entirely in UTC so the boundary is timezone/DST-independent.
	cutoff := dbwriter.EpochMillis(time.Now().UTC().AddDate(0, 0, -days))

	_, err = db.writer().ExecContext(ctx, deleteOldAuditLogsSQL, cutoff)
	if err != nil {
		return err
	}

	return nil
}

// ListAuditLogsByActorPage is a convenience wrapper that filters by actor only.
func (db *Database) ListAuditLogsByActorPage(ctx context.Context, actor string, page, perPage int) ([]dbwriter.AuditLog, int, error) {
	filters := &AuditLogFilters{Actor: &actor}
	return db.ListAuditLogsPage(ctx, filters, page, perPage)
}

func scanAuditLog(row scanner) (dbwriter.AuditLog, error) {
	var v dbwriter.AuditLog

	if err := row.Scan(&v.ID, &v.Timestamp, &v.Level, &v.Actor, &v.Action, &v.IP, &v.Details); err != nil {
		return dbwriter.AuditLog{}, err
	}

	return v, nil
}
