// Copyright 2024 Ella Networks

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/dbwriter"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
)

const AuditLogsTableName = "audit_logs"

// Structured table (no raw blob). Keep strings NOT NULL with empty defaults to avoid NullString hassle.
const QueryCreateAuditLogsTable = `
	CREATE TABLE IF NOT EXISTS %s (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp  TEXT NOT NULL,                      -- RFC3339
		level      TEXT NOT NULL,                      -- info|warn|error...
		actor      TEXT NOT NULL DEFAULT '',
		action     TEXT NOT NULL,
		ip         TEXT NOT NULL DEFAULT '',
		details    TEXT NOT NULL DEFAULT ''
);`

const (
	insertAuditLogStmt     = "INSERT INTO %s (timestamp, level, actor, action, ip, details) VALUES ($AuditLog.timestamp, $AuditLog.level, $AuditLog.actor, $AuditLog.action, $AuditLog.ip, $AuditLog.details)"
	listAuditLogsPageStmt  = "SELECT &AuditLog.*, COUNT(*) OVER() AS &NumItems.count FROM %s ORDER BY id DESC LIMIT $ListArgs.limit OFFSET $ListArgs.offset"
	deleteOldAuditLogsStmt = "DELETE FROM %s WHERE timestamp < $cutoffArgs.cutoff"
	countAuditLogsStmt     = "SELECT COUNT(*) AS &NumItems.count FROM %s"
)

// InsertAuditLogJSON parses the zap JSON and inserts a structured row.
func (db *Database) InsertAuditLog(ctx context.Context, auditLog *dbwriter.AuditLog) error {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "INSERT", AuditLogsTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("INSERT"),
			attribute.String("db.collection", AuditLogsTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(AuditLogsTableName, "insert"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(AuditLogsTableName, "insert").Inc()

	err := db.conn.Query(ctx, db.insertAuditLogStmt, auditLog).Run()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

func (db *Database) ListAuditLogsPage(ctx context.Context, page, perPage int) ([]dbwriter.AuditLog, int, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s (paged)", "SELECT", AuditLogsTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("SELECT"),
			attribute.String("db.collection", AuditLogsTableName),
			attribute.Int("page", page),
			attribute.Int("per_page", perPage),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(AuditLogsTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(AuditLogsTableName, "select").Inc()

	args := ListArgs{
		Limit:  perPage,
		Offset: (page - 1) * perPage,
	}

	var logs []dbwriter.AuditLog

	var counts []NumItems

	err := db.conn.Query(ctx, db.listAuditLogsStmt, args).GetAll(&logs, &counts)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			span.SetStatus(codes.Ok, "no rows")

			fallbackCount, countErr := db.CountAuditLogs(ctx)
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

	return logs, count, nil
}

// DeleteOldAuditLogs removes logs older than the specified retention period in days.
func (db *Database) DeleteOldAuditLogs(ctx context.Context, days int) error {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s (retention)", "DELETE", AuditLogsTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("DELETE"),
			attribute.String("db.collection", AuditLogsTableName),
			attribute.Int("retention.days", days),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(AuditLogsTableName, "delete"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(AuditLogsTableName, "delete").Inc()

	// Compute UTC cutoff so string comparison works lexicographically for RFC3339
	cutoff := time.Now().AddDate(0, 0, -days).UTC().Format(time.RFC3339)

	args := cutoffArgs{Cutoff: cutoff}

	if err := db.conn.Query(ctx, db.deleteOldAuditLogsStmt, args).Run(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

func (db *Database) CountAuditLogs(ctx context.Context) (int, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "COUNT", AuditLogsTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("COUNT"),
			attribute.String("db.collection", AuditLogsTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(AuditLogsTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(AuditLogsTableName, "select").Inc()

	var result NumItems

	err := db.conn.Query(ctx, db.countAuditLogsStmt).Get(&result)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			span.SetStatus(codes.Ok, "no rows")
			return 0, nil
		}

		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return 0, fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return result.Count, nil
}
