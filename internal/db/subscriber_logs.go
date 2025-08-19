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
		details    TEXT NOT NULL DEFAULT ''
);`

const (
	insertSubscriberLogStmt     = "INSERT INTO %s (timestamp, level, imsi, event, details) VALUES ($SubscriberLog.timestamp, $SubscriberLog.level, $SubscriberLog.imsi, $SubscriberLog.event, $SubscriberLog.details)"
	listSubscriberLogsStmt      = "SELECT &SubscriberLog.* FROM %s ORDER BY id DESC"
	deleteOldSubscriberLogsStmt = "DELETE FROM %s WHERE timestamp < $cutoffArgs.cutoff"
)

type SubscriberLog struct {
	ID        int    `db:"id"`
	Timestamp string `db:"timestamp"` // store as RFC3339 string; parse in API layer if needed
	Level     string `db:"level"`
	IMSI      string `db:"imsi"`
	Event     string `db:"event"`
	Details   string `db:"details"` // JSON or plain text (we store a string)
}

type zapSubscriberJSON struct {
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	IMSI      string `json:"imsi"`
	Event     string `json:"event"`
	Details   string `json:"details"` // could be string or object in the future
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

func (db *Database) ListSubscriberLogs(ctx context.Context) ([]SubscriberLog, error) {
	const operation = "SELECT"
	const target = SubscriberLogsTableName
	spanName := fmt.Sprintf("%s %s", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	stmt := fmt.Sprintf(listSubscriberLogsStmt, db.subscriberLogsTable)
	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(stmt),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
	)

	q, err := sqlair.Prepare(stmt, SubscriberLog{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "prepare failed")
		return nil, err
	}

	var logs []SubscriberLog
	if err := db.conn.Query(ctx, q).GetAll(&logs); err != nil {
		if err == sql.ErrNoRows {
			span.SetStatus(codes.Ok, "no rows")
			return nil, nil
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")
		return nil, err
	}

	span.SetStatus(codes.Ok, "")
	return logs, nil
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
