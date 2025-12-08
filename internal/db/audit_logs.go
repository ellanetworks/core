// Copyright 2024 Ella Networks

package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/canonical/sqlair"
	"github.com/ellanetworks/core/internal/dbwriter"
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
	listAuditLogsPageStmt  = "SELECT &AuditLog.* FROM %s ORDER BY id DESC LIMIT $ListArgs.limit OFFSET $ListArgs.offset"
	deleteOldAuditLogsStmt = "DELETE FROM %s WHERE timestamp < $cutoffArgs.cutoff"
	countAuditLogsStmt     = "SELECT COUNT(*) AS &NumItems.count FROM %s"
)

// InsertAuditLogJSON parses the zap JSON and inserts a structured row.
func (db *Database) InsertAuditLog(ctx context.Context, auditLog *dbwriter.AuditLog) error {
	const operation = "INSERT"
	const target = AuditLogsTableName
	spanName := fmt.Sprintf("%s %s", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	query := fmt.Sprintf(insertAuditLogStmt, db.auditLogsTable)
	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(query),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
	)

	stmt, err := sqlair.Prepare(query, dbwriter.AuditLog{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "prepare failed")
		return err
	}
	if err := db.conn.Query(ctx, stmt, auditLog).Run(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")
		return err
	}

	span.SetStatus(codes.Ok, "")
	return nil
}

func (db *Database) ListAuditLogsPage(ctx context.Context, page, perPage int) ([]dbwriter.AuditLog, int, error) {
	const operation = "SELECT"
	const target = AuditLogsTableName
	spanName := fmt.Sprintf("%s %s (paged)", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	stmtStr := fmt.Sprintf(listAuditLogsPageStmt, db.auditLogsTable)
	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(stmtStr),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
		attribute.Int("page", page),
		attribute.Int("per_page", perPage),
	)

	stmt, err := sqlair.Prepare(stmtStr, ListArgs{}, dbwriter.AuditLog{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "prepare failed")
		return nil, 0, err
	}

	args := ListArgs{
		Limit:  perPage,
		Offset: (page - 1) * perPage,
	}

	count, err := db.CountAuditLogs(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "count failed")
		return nil, 0, err
	}

	var logs []dbwriter.AuditLog

	if err := db.conn.Query(ctx, stmt, args).GetAll(&logs); err != nil {
		if err == sql.ErrNoRows {
			span.SetStatus(codes.Ok, "no rows")
			return nil, count, nil
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")
		return nil, 0, err
	}

	span.SetStatus(codes.Ok, "")
	return logs, count, nil
}

// DeleteOldAuditLogs removes logs older than the specified retention period in days.
func (db *Database) DeleteOldAuditLogs(ctx context.Context, days int) error {
	const operation = "DELETE"
	const target = AuditLogsTableName
	spanName := fmt.Sprintf("%s %s (retention)", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	// Compute UTC cutoff so string comparison works lexicographically for RFC3339
	cutoff := time.Now().AddDate(0, 0, -days).UTC().Format(time.RFC3339)

	stmtStr := fmt.Sprintf(deleteOldAuditLogsStmt, db.auditLogsTable)
	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(stmtStr),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
		attribute.Int("retention.days", days),
		attribute.String("retention.cutoff", cutoff),
	)

	stmt, err := sqlair.Prepare(stmtStr, cutoffArgs{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "prepare failed")
		return err
	}

	args := cutoffArgs{Cutoff: cutoff}
	if err := db.conn.Query(ctx, stmt, args).Run(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")
		return err
	}

	span.SetStatus(codes.Ok, "")
	return nil
}

func (db *Database) CountAuditLogs(ctx context.Context) (int, error) {
	const operation = "COUNT"
	const target = AuditLogsTableName
	spanName := fmt.Sprintf("%s %s", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	stmtStr := fmt.Sprintf(countAuditLogsStmt, db.auditLogsTable)
	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(stmtStr),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
	)

	stmt, err := sqlair.Prepare(stmtStr, NumItems{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "prepare failed")
		return 0, err
	}

	var result NumItems

	if err := db.conn.Query(ctx, stmt).Get(&result); err != nil {
		if err == sql.ErrNoRows {
			span.SetStatus(codes.Ok, "no rows")
			return 0, nil
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")
		return 0, err
	}

	span.SetStatus(codes.Ok, "")
	return result.Count, nil
}
