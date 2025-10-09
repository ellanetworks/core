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
		direction	TEXT NOT NULL DEFAULT '',       -- inbound|outbound
		raw       BLOB NOT NULL,
		details    TEXT NOT NULL DEFAULT ''
);`

const (
	insertRadioLogStmt     = "INSERT INTO %s (timestamp, level, ran_id, event, direction, raw, details) VALUES ($RadioLog.timestamp, $RadioLog.level, $RadioLog.ran_id, $RadioLog.event, $RadioLog.direction, $RadioLog.raw, $RadioLog.details)"
	deleteOldRadioLogsStmt = "DELETE FROM %s WHERE timestamp < $cutoffArgs.cutoff"
	deleteAllRadioLogsStmt = "DELETE FROM %s"
)

const listRadioLogsPagedFilteredStmt = `
  SELECT &RadioLog.*
  FROM %s
  WHERE
    ($RadioLogFilters.ran_id      IS NULL OR ran_id      = $RadioLogFilters.ran_id)
    AND ($RadioLogFilters.direction IS NULL OR direction = $RadioLogFilters.direction)
    AND ($RadioLogFilters.event     IS NULL OR event     = $RadioLogFilters.event)
    AND ($RadioLogFilters.from      IS NULL OR timestamp >= $RadioLogFilters.from)
    AND ($RadioLogFilters.to        IS NULL OR timestamp <  $RadioLogFilters.to)
  ORDER BY id DESC
  LIMIT $ListArgs.limit
  OFFSET $ListArgs.offset
`

const countRadioLogsFilteredStmt = `
  SELECT COUNT(*) AS &NumItems.count
  FROM %s
  WHERE
    ($RadioLogFilters.ran_id      IS NULL OR ran_id      = $RadioLogFilters.ran_id)
    AND ($RadioLogFilters.direction IS NULL OR direction = $RadioLogFilters.direction)
    AND ($RadioLogFilters.event     IS NULL OR event     = $RadioLogFilters.event)
    AND ($RadioLogFilters.from      IS NULL OR timestamp >= $RadioLogFilters.from)
    AND ($RadioLogFilters.to        IS NULL OR timestamp <  $RadioLogFilters.to)
`

type RadioLog struct {
	ID        int    `db:"id"`
	Timestamp string `db:"timestamp"` // store as RFC3339 string; parse in API layer if needed
	Level     string `db:"level"`
	RanID     string `db:"ran_id"`
	Event     string `db:"event"`
	Direction string `db:"direction"`
	Raw       []byte `db:"raw"`
	Details   string `db:"details"` // JSON or plain text (we store a string)
}

type zapRadioJSON struct {
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	RanID     string `json:"ran_id"`
	Event     string `json:"event"`
	Direction string `json:"direction"`
	Raw       []byte `json:"raw"`
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
		Direction: z.Direction,
		Raw:       z.Raw,
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

type RadioLogFilters struct {
	RanID     *string `db:"ran_id"`    // exact match
	Direction *string `db:"direction"` // "inbound" | "outbound"
	Event     *string `db:"event"`     // exact match
	From      *string `db:"from"`      // RFC3339 (UTC)
	To        *string `db:"to"`        // RFC3339 (UTC), exclusive upper bound
}

func (db *Database) ListRadioLogs(ctx context.Context, page int, perPage int, filters *RadioLogFilters) ([]RadioLog, int, error) {
	if filters == nil {
		filters = &RadioLogFilters{}
	}

	const operation = "SELECT"
	const target = RadioLogsTableName
	spanName := fmt.Sprintf("%s %s (paged+filtered)", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	listSQL := fmt.Sprintf(listRadioLogsPagedFilteredStmt, db.radioLogsTable)
	countSQL := fmt.Sprintf(countRadioLogsFilteredStmt, db.radioLogsTable)

	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(listSQL),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
		attribute.Int("page", page),
		attribute.Int("per_page", perPage),
	)

	// Prepare both statements with all the bind models they use
	listStmt, err := sqlair.Prepare(listSQL, ListArgs{}, RadioLogFilters{}, RadioLog{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "prepare list failed")
		return nil, 0, err
	}

	countStmt, err := sqlair.Prepare(countSQL, RadioLogFilters{}, NumItems{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "prepare count failed")
		return nil, 0, err
	}

	args := ListArgs{
		Limit:  perPage,
		Offset: (page - 1) * perPage,
	}

	// Count with filters
	var total NumItems
	if err := db.conn.Query(ctx, countStmt, filters).Get(&total); err != nil && err != sql.ErrNoRows {
		span.RecordError(err)
		span.SetStatus(codes.Error, "count failed")
		return nil, 0, err
	}

	// Rows with filters
	var logs []RadioLog
	if err := db.conn.Query(ctx, listStmt, args, filters).GetAll(&logs); err != nil {
		if err == sql.ErrNoRows {
			span.SetStatus(codes.Ok, "no rows")
			return nil, total.Count, nil
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")
		return nil, 0, err
	}

	span.SetStatus(codes.Ok, "")
	return logs, total.Count, nil
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

func (db *Database) ClearRadioLogs(ctx context.Context) error {
	const operation = "DELETE"
	const target = RadioLogsTableName
	spanName := fmt.Sprintf("%s %s (clear all)", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	stmtStr := fmt.Sprintf(deleteAllRadioLogsStmt, db.radioLogsTable)
	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(stmtStr),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
	)

	stmt, err := sqlair.Prepare(stmtStr)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "prepare failed")
		return err
	}

	if err := db.conn.Query(ctx, stmt).Run(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")
		return err
	}

	span.SetStatus(codes.Ok, "")
	return nil
}
