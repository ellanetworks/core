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

const RadioEventsTableName = "network_logs"

// Structured table (no raw blob). Keep strings NOT NULL with empty defaults to avoid NullString hassle.
const QueryCreateRadioEventsTable = `
	CREATE TABLE IF NOT EXISTS %s (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp  TEXT NOT NULL,                      -- RFC3339
		protocol      TEXT NOT NULL,
		message_type TEXT NOT NULL,
		direction	TEXT NOT NULL DEFAULT '',       -- inbound|outbound
		local_address TEXT NOT NULL DEFAULT '',
		remote_address TEXT NOT NULL DEFAULT '',
		raw			 BLOB NOT NULL,
		details    TEXT NOT NULL DEFAULT ''
);`

const QueryCreateRadioEventsIndex = `
	CREATE INDEX IF NOT EXISTS idx_network_logs_protocol ON network_logs (protocol);
	CREATE INDEX IF NOT EXISTS idx_network_logs_timestamp ON network_logs (timestamp);
	CREATE INDEX IF NOT EXISTS idx_network_logs_message_type ON network_logs (message_type);
	CREATE INDEX IF NOT EXISTS idx_network_logs_direction ON network_logs (direction);
	CREATE INDEX IF NOT EXISTS idx_network_logs_local_address ON network_logs (local_address);
	CREATE INDEX IF NOT EXISTS idx_network_logs_remote_address ON network_logs (remote_address);
`

const (
	insertRadioEventStmt     = "INSERT INTO %s (timestamp, protocol, message_type, direction, local_address, remote_address, raw, details) VALUES ($RadioEvent.timestamp, $RadioEvent.protocol, $RadioEvent.message_type, $RadioEvent.direction, $RadioEvent.local_address, $RadioEvent.remote_address, $RadioEvent.raw, $RadioEvent.details)"
	getRadioEventByIDStmt    = "SELECT &RadioEvent.* FROM %s WHERE id = $RadioEvent.id"
	deleteOldRadioEventsStmt = "DELETE FROM %s WHERE timestamp < $cutoffArgs.cutoff"
	deleteAllRadioEventsStmt = "DELETE FROM %s"
)

const listRadioEventsPagedFilteredStmt = `
  SELECT &RadioEvent.*, COUNT(*) OVER() AS &NumItems.count
  FROM %s
  WHERE
    ($RadioEventFilters.protocol      IS NULL OR protocol      = $RadioEventFilters.protocol)
    AND ($RadioEventFilters.direction IS NULL OR direction = $RadioEventFilters.direction)
    AND ($RadioEventFilters.local_address IS NULL OR local_address = $RadioEventFilters.local_address)
    AND ($RadioEventFilters.remote_address IS NULL OR remote_address = $RadioEventFilters.remote_address)
    AND ($RadioEventFilters.message_type IS NULL OR message_type     = $RadioEventFilters.message_type)
    AND ($RadioEventFilters.timestamp_from  IS NULL OR timestamp >= $RadioEventFilters.timestamp_from)
    AND ($RadioEventFilters.timestamp_to    IS NULL OR timestamp <  $RadioEventFilters.timestamp_to)
  ORDER BY id DESC
  LIMIT $ListArgs.limit
  OFFSET $ListArgs.offset
`

const countRadioEventsFilteredStmt = `
  SELECT COUNT(*) AS &NumItems.count
  FROM %s
  WHERE
    ($RadioEventFilters.protocol      IS NULL OR protocol      = $RadioEventFilters.protocol)
    AND ($RadioEventFilters.direction IS NULL OR direction = $RadioEventFilters.direction)
    AND ($RadioEventFilters.local_address IS NULL OR local_address = $RadioEventFilters.local_address)
    AND ($RadioEventFilters.remote_address IS NULL OR remote_address = $RadioEventFilters.remote_address)
    AND ($RadioEventFilters.message_type IS NULL OR message_type     = $RadioEventFilters.message_type)
    AND ($RadioEventFilters.timestamp_from  IS NULL OR timestamp >= $RadioEventFilters.timestamp_from)
    AND ($RadioEventFilters.timestamp_to    IS NULL OR timestamp <  $RadioEventFilters.timestamp_to)
`

type RadioEventFilters struct {
	Protocol      *string `db:"protocol"`       // exact match
	Direction     *string `db:"direction"`      // "inbound" | "outbound"
	LocalAddress  *string `db:"local_address"`  // exact match
	RemoteAddress *string `db:"remote_address"` // exact match
	MessageType   *string `db:"message_type"`   // exact match
	TimestampFrom *string `db:"timestamp_from"` // RFC3339 (UTC)
	TimestampTo   *string `db:"timestamp_to"`   // RFC3339 (UTC), exclusive upper bound
}

func (db *Database) InsertRadioEvent(ctx context.Context, radioEvent *dbwriter.RadioEvent) error {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "INSERT", RadioEventsTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("INSERT"),
			attribute.String("db.collection", RadioEventsTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(RadioEventsTableName, "insert"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(RadioEventsTableName, "insert").Inc()

	err := db.conn.Query(ctx, db.insertRadioEventStmt, radioEvent).Run()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

func (db *Database) ListRadioEvents(ctx context.Context, page int, perPage int, filters *RadioEventFilters) ([]dbwriter.RadioEvent, int, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s (paged+filtered)", "SELECT", RadioEventsTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("SELECT"),
			attribute.String("db.collection", RadioEventsTableName),
			attribute.Int("page", page),
			attribute.Int("per_page", perPage),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(RadioEventsTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(RadioEventsTableName, "select").Inc()

	if filters == nil {
		filters = &RadioEventFilters{}
	}

	args := ListArgs{
		Limit:  perPage,
		Offset: (page - 1) * perPage,
	}

	var logs []dbwriter.RadioEvent

	var counts []NumItems

	err := db.conn.Query(ctx, db.listRadioEventsStmt, args, filters).GetAll(&logs, &counts)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			span.SetStatus(codes.Ok, "no rows")

			var fallbackCount NumItems

			countErr := db.conn.Query(ctx, db.countRadioEventsStmt, filters).Get(&fallbackCount)
			if countErr != nil {
				return nil, 0, nil
			}

			return nil, fallbackCount.Count, nil
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

// DeleteOldRadioEvents removes logs older than the specified retention period in days.
func (db *Database) DeleteOldRadioEvents(ctx context.Context, days int) error {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s (retention)", "DELETE", RadioEventsTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("DELETE"),
			attribute.String("db.collection", RadioEventsTableName),
			attribute.Int("retention.days", days),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(RadioEventsTableName, "delete"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(RadioEventsTableName, "delete").Inc()

	// Compute UTC cutoff so string comparison works lexicographically for RFC3339
	cutoff := time.Now().AddDate(0, 0, -days).UTC().Format(time.RFC3339)

	args := cutoffArgs{Cutoff: cutoff}

	err := db.conn.Query(ctx, db.deleteOldRadioEventsStmt, args).Run()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

func (db *Database) ClearRadioEvents(ctx context.Context) error {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s (all)", "DELETE", RadioEventsTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("DELETE"),
			attribute.String("db.collection", RadioEventsTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(RadioEventsTableName, "delete"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(RadioEventsTableName, "delete").Inc()

	err := db.conn.Query(ctx, db.deleteAllRadioEventsStmt).Run()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

func (db *Database) GetRadioEventByID(ctx context.Context, id int) (*dbwriter.RadioEvent, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s (by ID)", "SELECT", RadioEventsTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("SELECT"),
			attribute.String("db.collection", RadioEventsTableName),
			attribute.Int("id", id),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(RadioEventsTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(RadioEventsTableName, "select").Inc()

	log := dbwriter.RadioEvent{ID: id}

	err := db.conn.Query(ctx, db.getRadioEventByIDStmt, log).Get(&log)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			span.SetStatus(codes.Ok, "no rows")
			return nil, ErrNotFound
		}

		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return nil, fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return &log, nil
}
