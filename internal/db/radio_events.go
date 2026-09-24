// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

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
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
)

const RadioEventsTableName = "network_logs"

const (
	insertRadioEventStmt     = "INSERT INTO %s (timestamp, protocol, message_type, direction, local_address, remote_address, radio_name, raw, details) VALUES ($RadioEvent.timestamp, $RadioEvent.protocol, $RadioEvent.message_type, $RadioEvent.direction, $RadioEvent.local_address, $RadioEvent.remote_address, $RadioEvent.radio_name, $RadioEvent.raw, $RadioEvent.details)"
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
    AND ($RadioEventFilters.radio_name IS NULL OR radio_name = $RadioEventFilters.radio_name)
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
    AND ($RadioEventFilters.radio_name IS NULL OR radio_name = $RadioEventFilters.radio_name)
    AND ($RadioEventFilters.message_type IS NULL OR message_type     = $RadioEventFilters.message_type)
    AND ($RadioEventFilters.timestamp_from  IS NULL OR timestamp >= $RadioEventFilters.timestamp_from)
    AND ($RadioEventFilters.timestamp_to    IS NULL OR timestamp <  $RadioEventFilters.timestamp_to)
`

type RadioEventFilters struct {
	Protocol      *string `db:"protocol"`       // exact match
	Direction     *string `db:"direction"`      // "inbound" | "outbound"
	RadioName     *string `db:"radio_name"`     // exact match
	MessageType   *string `db:"message_type"`   // exact match
	TimestampFrom *int64  `db:"timestamp_from"` // epoch milliseconds, inclusive lower bound
	TimestampTo   *int64  `db:"timestamp_to"`   // epoch milliseconds, exclusive upper bound
}

func (db *Database) InsertRadioEvent(ctx context.Context, radioEvent *dbwriter.RadioEvent) error {
	querySummary := fmt.Sprintf("%s %s", "INSERT", RadioEventsTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("INSERT"),
			semconv.DBCollectionName(RadioEventsTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(RadioEventsTableName, "insert"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(RadioEventsTableName, "insert").Inc()

	err := db.conn().Query(ctx, db.insertRadioEventStmt, radioEvent).Run()
	if err != nil {
		recordSpanError(span, err)

		return fmt.Errorf("query failed: %w", err)
	}

	return nil
}

func (db *Database) ListRadioEvents(ctx context.Context, page int, perPage int, filters *RadioEventFilters) ([]dbwriter.RadioEvent, int, error) {
	querySummary := fmt.Sprintf("%s %s (paged+filtered)", "SELECT", RadioEventsTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			semconv.DBCollectionName(RadioEventsTableName),
			attribute.Int("ella.db.page", page),
			attribute.Int("ella.db.page_size", perPage),
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

	err := db.conn().Query(ctx, db.listRadioEventsStmt, args, filters).GetAll(&logs, &counts)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			var fallbackCount NumItems

			countErr := db.conn().Query(ctx, db.countRadioEventsStmt, filters).Get(&fallbackCount)
			if countErr != nil {
				return nil, 0, nil
			}

			return nil, fallbackCount.Count, nil
		}

		recordSpanError(span, err)

		return nil, 0, fmt.Errorf("query failed: %w", err)
	}

	count := 0
	if len(counts) > 0 {
		count = counts[0].Count
	}

	return logs, count, nil
}

// DeleteOldRadioEvents removes logs older than the specified retention period in days.
func (db *Database) DeleteOldRadioEvents(ctx context.Context, days int) error {
	querySummary := fmt.Sprintf("%s %s (retention)", "DELETE", RadioEventsTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("DELETE"),
			semconv.DBCollectionName(RadioEventsTableName),
			attribute.Int("retention.days", days),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(RadioEventsTableName, "delete"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(RadioEventsTableName, "delete").Inc()

	cutoff := dbwriter.EpochMillis(time.Now().UTC().AddDate(0, 0, -days))

	args := cutoffArgs{Cutoff: cutoff}

	err := db.conn().Query(ctx, db.deleteOldRadioEventsStmt, args).Run()
	if err != nil {
		recordSpanError(span, err)

		return fmt.Errorf("query failed: %w", err)
	}

	return nil
}

func (db *Database) ClearRadioEvents(ctx context.Context) error {
	querySummary := fmt.Sprintf("%s %s (all)", "DELETE", RadioEventsTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("DELETE"),
			semconv.DBCollectionName(RadioEventsTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(RadioEventsTableName, "delete"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(RadioEventsTableName, "delete").Inc()

	err := db.conn().Query(ctx, db.deleteAllRadioEventsStmt).Run()
	if err != nil {
		recordSpanError(span, err)

		return fmt.Errorf("query failed: %w", err)
	}

	return nil
}

func (db *Database) GetRadioEventByID(ctx context.Context, id int) (*dbwriter.RadioEvent, error) {
	querySummary := fmt.Sprintf("%s %s (by ID)", "SELECT", RadioEventsTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			semconv.DBCollectionName(RadioEventsTableName),
			attribute.Int("radio_event.id", id),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(RadioEventsTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(RadioEventsTableName, "select").Inc()

	log := dbwriter.RadioEvent{ID: id}

	err := db.conn().Query(ctx, db.getRadioEventByIDStmt, log).Get(&log)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return &log, nil
}
