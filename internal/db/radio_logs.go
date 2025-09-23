// Copyright 2024 Ella Networks

package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/canonical/sqlair"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
)

const RadioLogsTableName = "radio_logs"

// Structured table (no raw blob). Keep strings NOT NULL with empty defaults to avoid NullString hassle.
const QueryCreateRadioLogsTable = `
	CREATE TABLE IF NOT EXISTS %s (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp  TEXT NOT NULL,                      -- RFC3339
		level      TEXT NOT NULL,                      -- info|warn|error...
		ran_id      TEXT NOT NULL DEFAULT '',
		event     TEXT NOT NULL,
		details    TEXT NOT NULL DEFAULT ''
);`

const (
	insertRadioLogStmt     = "INSERT INTO %s (timestamp, level, ran_id, event, details) VALUES ($RadioLog.timestamp, $RadioLog.level, $RadioLog.RanID, $RadioLog.event, $RadioLog.details)"
	listRadioLogsPagedStmt = "SELECT &RadioLog.* FROM %s ORDER BY id DESC LIMIT $ListArgs.limit OFFSET $ListArgs.offset"
	deleteOldRadioLogsStmt = "DELETE FROM %s WHERE timestamp < $cutoffArgs.cutoff"
	countRadioLogsStmt     = "SELECT COUNT(*) AS &NumItems.count FROM %s"
)

type RadioLog struct {
	ID        int    `db:"id"`
	Timestamp string `db:"timestamp"` // store as RFC3339 string; parse in API layer if needed
	Level     string `db:"level"`
	RanID     string `db:"ran_id"`
	Event     string `db:"event"`
	Details   string `db:"details"` // JSON or plain text (we store a string)
}

type zapRadioJSON struct {
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	RanID     string `json:"ran_id"`
	Event     string `json:"event"`
	Details   string `json:"details"` // could be string or object in the future
}

func (db *Database) RadioWriteFunc(ctx context.Context) func([]byte) error {
	return func(b []byte) error {
		return db.InsertRadioLogJSON(ctx, b)
	}
}

// InsertRadioLogJSON parses the zap JSON and inserts a structured row.
func (db *Database) InsertRadioLogJSON(ctx context.Context, raw []byte) error {
	const operation = "INSERT"
	const target = RadioLogsTableName
	spanName := fmt.Sprintf("%s %s", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	query := fmt.Sprintf(insertRadioLogStmt, db.radioLogsTable)
	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(query),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
	)

	// Parse incoming JSON
	var z zapRadioJSON
	if err := json.Unmarshal(raw, &z); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "unmarshal failed")
		return err
	}

	row := RadioLog{
		Timestamp: z.Timestamp,
		Level:     z.Level,
		RanID:     z.RanID,
		Event:     z.Event,
		Details:   z.Details,
	}

	stmt, err := sqlair.Prepare(query, RadioLog{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "prepare failed")
		return err
	}
	if err := db.conn.Query(ctx, stmt, row).Run(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")
		return err
	}

	span.SetStatus(codes.Ok, "")
	return nil
}

func (db *Database) ListRadioLogsPage(ctx context.Context, page, perPage int) ([]RadioLog, int, error) {
	const operation = "SELECT"
	const target = RadioLogsTableName
	spanName := fmt.Sprintf("%s %s (paged)", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	stmtStr := fmt.Sprintf(listRadioLogsPagedStmt, db.radioLogsTable)
	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(stmtStr),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
		attribute.Int("page", page),
		attribute.Int("per_page", perPage),
	)

	stmt, err := sqlair.Prepare(stmtStr, ListArgs{}, RadioLog{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "prepare failed")
		return nil, 0, err
	}

	args := ListArgs{
		Limit:  perPage,
		Offset: (page - 1) * perPage,
	}

	count, err := db.CountRadioLogs(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "count failed")
		return nil, 0, err
	}

	var logs []RadioLog

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

// DeleteOldRadioLogs removes logs older than the specified retention period in days.
func (db *Database) DeleteOldRadioLogs(ctx context.Context, days int) error {
	const operation = "DELETE"
	const target = RadioLogsTableName
	spanName := fmt.Sprintf("%s %s (retention)", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	// Compute UTC cutoff so string comparison works lexicographically for RFC3339
	cutoff := time.Now().AddDate(0, 0, -days).UTC().Format(time.RFC3339)

	stmtStr := fmt.Sprintf(deleteOldRadioLogsStmt, db.radioLogsTable)
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

func (db *Database) CountRadioLogs(ctx context.Context) (int, error) {
	const operation = "COUNT"
	const target = RadioLogsTableName
	spanName := fmt.Sprintf("%s %s", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	stmtStr := fmt.Sprintf(countRadioLogsStmt, db.radioLogsTable)
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
