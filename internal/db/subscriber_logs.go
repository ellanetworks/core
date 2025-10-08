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

const SubscriberLogsTableName = "subscriber_logs"

// Structured table (no raw blob). Keep strings NOT NULL with empty defaults to avoid NullString hassle.
const QueryCreateSubscriberLogsTable = `
	CREATE TABLE IF NOT EXISTS %s (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp  TEXT NOT NULL,                      -- RFC3339
		level      TEXT NOT NULL,                      -- info|warn|error...
		imsi      TEXT NOT NULL DEFAULT '',
		event     TEXT NOT NULL,
		direction	TEXT NOT NULL DEFAULT '',       -- inbound|outbound
		raw			 BLOB NOT NULL,
		details    TEXT NOT NULL DEFAULT ''
);`

const QueryCreateSubscriberLogsIndex = `
	CREATE INDEX IF NOT EXISTS idx_subscriber_logs_imsi ON subscriber_logs (imsi);
	CREATE INDEX IF NOT EXISTS idx_subscriber_logs_timestamp ON subscriber_logs (timestamp);
	CREATE INDEX IF NOT EXISTS idx_subscriber_logs_event ON subscriber_logs (event);
	CREATE INDEX IF NOT EXISTS idx_subscriber_logs_direction ON subscriber_logs (direction);
`

const (
	insertSubscriberLogStmt     = "INSERT INTO %s (timestamp, level, imsi, event, direction, raw, details) VALUES ($SubscriberLog.timestamp, $SubscriberLog.level, $SubscriberLog.imsi, $SubscriberLog.event, $SubscriberLog.direction, $SubscriberLog.raw, $SubscriberLog.details)"
	deleteOldSubscriberLogsStmt = "DELETE FROM %s WHERE timestamp < $cutoffArgs.cutoff"
	deleteAllSubscriberLogsStmt = "DELETE FROM %s"
)

const listSubscriberLogsPagedFilteredStmt = `
  SELECT &SubscriberLog.*
  FROM %s
  WHERE
    ($SubscriberLogFilters.imsi      IS NULL OR imsi      = $SubscriberLogFilters.imsi)
    AND ($SubscriberLogFilters.direction IS NULL OR direction = $SubscriberLogFilters.direction)
    AND ($SubscriberLogFilters.event IS NULL OR event     = $SubscriberLogFilters.event)
    AND ($SubscriberLogFilters.from  IS NULL OR timestamp >= $SubscriberLogFilters.from)
    AND ($SubscriberLogFilters.to    IS NULL OR timestamp <  $SubscriberLogFilters.to)
  ORDER BY id DESC
  LIMIT $ListArgs.limit
  OFFSET $ListArgs.offset
`

const countSubscriberLogsFilteredStmt = `
  SELECT COUNT(*) AS &NumItems.count
  FROM %s
  WHERE
    ($SubscriberLogFilters.imsi      IS NULL OR imsi      = $SubscriberLogFilters.imsi)
    AND ($SubscriberLogFilters.direction IS NULL OR direction = $SubscriberLogFilters.direction)
    AND ($SubscriberLogFilters.event IS NULL OR event     = $SubscriberLogFilters.event)
    AND ($SubscriberLogFilters.from  IS NULL OR timestamp >= $SubscriberLogFilters.from)
    AND ($SubscriberLogFilters.to    IS NULL OR timestamp <  $SubscriberLogFilters.to)
`

type SubscriberLog struct {
	ID        int    `db:"id"`
	Timestamp string `db:"timestamp"` // store as RFC3339 string; parse in API layer if needed
	Level     string `db:"level"`
	IMSI      string `db:"imsi"`
	Event     string `db:"event"`
	Direction string `db:"direction"`
	Raw       []byte `db:"raw"`
	Details   string `db:"details"` // JSON or plain text (we store a string)
}

type zapSubscriberJSON struct {
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	IMSI      string `json:"imsi"`
	Event     string `json:"event"`
	Direction string `json:"direction"`
	Raw       []byte `json:"raw"`
	Details   string `json:"details"`
}

func (db *Database) SubscriberWriteFunc(ctx context.Context) func([]byte) error {
	return func(b []byte) error {
		return db.InsertSubscriberLogJSON(ctx, b)
	}
}

// InsertSubscriberLogJSON parses the zap JSON and inserts a structured row.
func (db *Database) InsertSubscriberLogJSON(ctx context.Context, raw []byte) error {
	const operation = "INSERT"
	const target = SubscriberLogsTableName
	spanName := fmt.Sprintf("%s %s", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	query := fmt.Sprintf(insertSubscriberLogStmt, db.subscriberLogsTable)
	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(query),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
	)

	// Parse incoming JSON
	var z zapSubscriberJSON
	if err := json.Unmarshal(raw, &z); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "unmarshal failed")
		return err
	}

	row := SubscriberLog{
		Timestamp: z.Timestamp,
		Level:     z.Level,
		IMSI:      z.IMSI,
		Event:     z.Event,
		Direction: z.Direction,
		Raw:       z.Raw,
		Details:   z.Details,
	}

	stmt, err := sqlair.Prepare(query, SubscriberLog{})
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

type SubscriberLogFilters struct {
	IMSI      *string `db:"imsi"`      // exact match
	Direction *string `db:"direction"` // "inbound" | "outbound"
	Event     *string `db:"event"`     // exact match
	From      *string `db:"from"`      // RFC3339 (UTC)
	To        *string `db:"to"`        // RFC3339 (UTC), exclusive upper bound
}

func (db *Database) ListSubscriberLogsPage(ctx context.Context, page int, perPage int, filters *SubscriberLogFilters) ([]SubscriberLog, int, error) {
	if filters == nil {
		filters = &SubscriberLogFilters{}
	}

	const operation = "SELECT"
	const target = SubscriberLogsTableName
	spanName := fmt.Sprintf("%s %s (paged+filtered)", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	listSQL := fmt.Sprintf(listSubscriberLogsPagedFilteredStmt, db.subscriberLogsTable)
	countSQL := fmt.Sprintf(countSubscriberLogsFilteredStmt, db.subscriberLogsTable)

	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(listSQL),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
		attribute.Int("page", page),
		attribute.Int("per_page", perPage),
	)

	// Prepare both statements with all the bind models they use
	listStmt, err := sqlair.Prepare(listSQL, ListArgs{}, SubscriberLogFilters{}, SubscriberLog{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "prepare list failed")
		return nil, 0, err
	}
	countStmt, err := sqlair.Prepare(countSQL, SubscriberLogFilters{}, NumItems{})
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
	var logs []SubscriberLog
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

// DeleteOldSubscriberLogs removes logs older than the specified retention period in days.
func (db *Database) DeleteOldSubscriberLogs(ctx context.Context, days int) error {
	const operation = "DELETE"
	const target = SubscriberLogsTableName
	spanName := fmt.Sprintf("%s %s (retention)", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	// Compute UTC cutoff so string comparison works lexicographically for RFC3339
	cutoff := time.Now().AddDate(0, 0, -days).UTC().Format(time.RFC3339)

	stmtStr := fmt.Sprintf(deleteOldSubscriberLogsStmt, db.subscriberLogsTable)
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

func (db *Database) ClearSubscriberLogs(ctx context.Context) error {
	const operation = "DELETE"
	const target = SubscriberLogsTableName
	spanName := fmt.Sprintf("%s %s (all)", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	stmtStr := fmt.Sprintf(deleteAllSubscriberLogsStmt, db.subscriberLogsTable)
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
