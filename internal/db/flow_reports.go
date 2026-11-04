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
	"github.com/ellanetworks/core/internal/logger"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

const FlowReportsTableName = "flow_reports"

const (
	insertFlowReportStmt     = "INSERT INTO %s (subscriber_id, source_ip, destination_ip, source_port, destination_port, protocol, packets, bytes, start_time, end_time, direction, action) VALUES ($FlowReport.subscriber_id, $FlowReport.source_ip, $FlowReport.destination_ip, $FlowReport.source_port, $FlowReport.destination_port, $FlowReport.protocol, $FlowReport.packets, $FlowReport.bytes, $FlowReport.start_time, $FlowReport.end_time, $FlowReport.direction, $FlowReport.action)"
	deleteOldFlowReportsStmt = "DELETE FROM %s WHERE end_time < $cutoffArgs.cutoff"
	deleteAllFlowReportsStmt = "DELETE FROM %s"
)

const listFlowReportsPagedFilteredStmt = `
  SELECT &FlowReport.*, COUNT(*) OVER() AS &NumItems.count
  FROM %s
  WHERE
    ($FlowReportFilters.subscriber_id IS NULL OR subscriber_id = $FlowReportFilters.subscriber_id)
    AND ($FlowReportFilters.protocol IS NULL OR protocol = $FlowReportFilters.protocol)
    AND ($FlowReportFilters.source_ip IS NULL OR source_ip = $FlowReportFilters.source_ip)
    AND ($FlowReportFilters.destination_ip IS NULL OR destination_ip = $FlowReportFilters.destination_ip)
    AND ($FlowReportFilters.source_port IS NULL OR source_port = $FlowReportFilters.source_port)
    AND ($FlowReportFilters.destination_port IS NULL OR destination_port = $FlowReportFilters.destination_port)
    AND ($FlowReportFilters.range_start IS NULL OR end_time >= $FlowReportFilters.range_start)
    AND ($FlowReportFilters.range_end IS NULL OR start_time < $FlowReportFilters.range_end)
    AND ($FlowReportFilters.direction IS NULL OR direction = $FlowReportFilters.direction)
    AND ($FlowReportFilters.action IS NULL OR action = $FlowReportFilters.action)
  ORDER BY id DESC
  LIMIT $ListArgs.limit
  OFFSET $ListArgs.offset
`

const countFlowReportsFilteredStmt = `
  SELECT COUNT(*) AS &NumItems.count
  FROM %s
  WHERE
    ($FlowReportFilters.subscriber_id IS NULL OR subscriber_id = $FlowReportFilters.subscriber_id)
    AND ($FlowReportFilters.protocol IS NULL OR protocol = $FlowReportFilters.protocol)
    AND ($FlowReportFilters.source_ip IS NULL OR source_ip = $FlowReportFilters.source_ip)
    AND ($FlowReportFilters.destination_ip IS NULL OR destination_ip = $FlowReportFilters.destination_ip)
    AND ($FlowReportFilters.source_port IS NULL OR source_port = $FlowReportFilters.source_port)
    AND ($FlowReportFilters.destination_port IS NULL OR destination_port = $FlowReportFilters.destination_port)
    AND ($FlowReportFilters.range_start IS NULL OR end_time >= $FlowReportFilters.range_start)
    AND ($FlowReportFilters.range_end IS NULL OR start_time < $FlowReportFilters.range_end)
    AND ($FlowReportFilters.direction IS NULL OR direction = $FlowReportFilters.direction)
    AND ($FlowReportFilters.action IS NULL OR action = $FlowReportFilters.action)
`

const listFlowReportsFilteredByDayStmt = `
SELECT &FlowReport.*
FROM %s
WHERE
    ($FlowReportFilters.subscriber_id IS NULL OR subscriber_id = $FlowReportFilters.subscriber_id)
    AND ($FlowReportFilters.protocol IS NULL OR protocol = $FlowReportFilters.protocol)
    AND ($FlowReportFilters.source_ip IS NULL OR source_ip = $FlowReportFilters.source_ip)
    AND ($FlowReportFilters.destination_ip IS NULL OR destination_ip = $FlowReportFilters.destination_ip)
    AND ($FlowReportFilters.source_port IS NULL OR source_port = $FlowReportFilters.source_port)
    AND ($FlowReportFilters.destination_port IS NULL OR destination_port = $FlowReportFilters.destination_port)
    AND ($FlowReportFilters.range_start IS NULL OR end_time >= $FlowReportFilters.range_start)
    AND ($FlowReportFilters.range_end IS NULL OR start_time < $FlowReportFilters.range_end)
    AND ($FlowReportFilters.direction IS NULL OR direction = $FlowReportFilters.direction)
    AND ($FlowReportFilters.action IS NULL OR action = $FlowReportFilters.action)
ORDER BY end_time ASC
`

const listFlowReportsFilteredBySubscriberStmt = `
SELECT &FlowReport.*
FROM %s
WHERE
    ($FlowReportFilters.subscriber_id IS NULL OR subscriber_id = $FlowReportFilters.subscriber_id)
    AND ($FlowReportFilters.protocol IS NULL OR protocol = $FlowReportFilters.protocol)
    AND ($FlowReportFilters.source_ip IS NULL OR source_ip = $FlowReportFilters.source_ip)
    AND ($FlowReportFilters.destination_ip IS NULL OR destination_ip = $FlowReportFilters.destination_ip)
    AND ($FlowReportFilters.source_port IS NULL OR source_port = $FlowReportFilters.source_port)
    AND ($FlowReportFilters.destination_port IS NULL OR destination_port = $FlowReportFilters.destination_port)
    AND ($FlowReportFilters.range_start IS NULL OR end_time >= $FlowReportFilters.range_start)
    AND ($FlowReportFilters.range_end IS NULL OR start_time < $FlowReportFilters.range_end)
    AND ($FlowReportFilters.direction IS NULL OR direction = $FlowReportFilters.direction)
    AND ($FlowReportFilters.action IS NULL OR action = $FlowReportFilters.action)
ORDER BY subscriber_id ASC, end_time ASC
`

const flowReportProtocolCountsStmt = `
SELECT protocol AS &FlowReportProtocolCount.protocol, COUNT(*) AS &FlowReportProtocolCount.count
FROM %s
WHERE
    ($FlowReportFilters.subscriber_id IS NULL OR subscriber_id = $FlowReportFilters.subscriber_id)
    AND ($FlowReportFilters.protocol IS NULL OR protocol = $FlowReportFilters.protocol)
    AND ($FlowReportFilters.source_ip IS NULL OR source_ip = $FlowReportFilters.source_ip)
    AND ($FlowReportFilters.destination_ip IS NULL OR destination_ip = $FlowReportFilters.destination_ip)
    AND ($FlowReportFilters.source_port IS NULL OR source_port = $FlowReportFilters.source_port)
    AND ($FlowReportFilters.destination_port IS NULL OR destination_port = $FlowReportFilters.destination_port)
    AND ($FlowReportFilters.range_start IS NULL OR end_time >= $FlowReportFilters.range_start)
    AND ($FlowReportFilters.range_end IS NULL OR start_time < $FlowReportFilters.range_end)
    AND ($FlowReportFilters.direction IS NULL OR direction = $FlowReportFilters.direction)
    AND ($FlowReportFilters.action IS NULL OR action = $FlowReportFilters.action)
GROUP BY protocol
ORDER BY COUNT(*) DESC
`

const flowReportTopDestinationsUplinkStmt = `
SELECT destination_ip AS &FlowReportIPCount.ip, COUNT(*) AS &FlowReportIPCount.count
FROM %s
WHERE
    ($FlowReportFilters.subscriber_id IS NULL OR subscriber_id = $FlowReportFilters.subscriber_id)
    AND ($FlowReportFilters.protocol IS NULL OR protocol = $FlowReportFilters.protocol)
    AND ($FlowReportFilters.source_ip IS NULL OR source_ip = $FlowReportFilters.source_ip)
    AND ($FlowReportFilters.destination_ip IS NULL OR destination_ip = $FlowReportFilters.destination_ip)
    AND ($FlowReportFilters.source_port IS NULL OR source_port = $FlowReportFilters.source_port)
    AND ($FlowReportFilters.destination_port IS NULL OR destination_port = $FlowReportFilters.destination_port)
    AND ($FlowReportFilters.range_start IS NULL OR end_time >= $FlowReportFilters.range_start)
    AND ($FlowReportFilters.range_end IS NULL OR start_time < $FlowReportFilters.range_end)
    AND direction = 'uplink'
    AND ($FlowReportFilters.action IS NULL OR action = $FlowReportFilters.action)
GROUP BY destination_ip
ORDER BY COUNT(*) DESC
LIMIT 10
`

type FlowReportProtocolCount struct {
	Protocol uint8 `db:"protocol"`
	Count    int   `db:"count"`
}

type FlowReportIPCount struct {
	IP    string `db:"ip"`
	Count int    `db:"count"`
}

type FlowReportFilters struct {
	SubscriberID    *string `db:"subscriber_id"`    // exact match (IMSI)
	Protocol        *uint8  `db:"protocol"`         // exact match
	SourceIP        *string `db:"source_ip"`        // exact match
	DestinationIP   *string `db:"destination_ip"`   // exact match
	SourcePort      *uint16 `db:"source_port"`      // exact match
	DestinationPort *uint16 `db:"destination_port"` // exact match
	RangeStart      *int64  `db:"range_start"`      // epoch milliseconds, inclusive
	RangeEnd        *int64  `db:"range_end"`        // epoch milliseconds, exclusive
	Direction       *string `db:"direction"`        // "uplink" or "downlink"
	Action          *uint8  `db:"action"`           // 0 = "allow", 1 = "drop"
}

// InsertFlowReports inserts multiple flow reports in a single SQLite
// transaction. This is significantly faster than individual inserts because
// SQLite only performs one fsync per transaction instead of one per row.
func (db *Database) InsertFlowReports(ctx context.Context, flowReports []*dbwriter.FlowReport) error {
	if len(flowReports) == 0 {
		return nil
	}

	operation := "INSERT"

	var batchAttrs []attribute.KeyValue

	if len(flowReports) > 1 {
		operation = "BATCH INSERT"
		batchAttrs = []attribute.KeyValue{semconv.DBOperationBatchSize(len(flowReports))}
	}

	querySummary := fmt.Sprintf("%s %s", operation, FlowReportsTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName(operation),
			semconv.DBCollectionName(FlowReportsTableName),
		),
		trace.WithAttributes(batchAttrs...),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(FlowReportsTableName, "batch_insert"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(FlowReportsTableName, "batch_insert").Inc()

	tx, err := db.conn().Begin(ctx, nil)
	if err != nil {
		recordSpanError(span, err)

		return fmt.Errorf("begin transaction failed: %w", err)
	}

	orphaned := 0

	for _, fr := range flowReports {
		if err := tx.Query(ctx, db.insertFlowReportStmt, fr).Run(); err != nil {
			if isForeignKeyError(err) {
				orphaned++
				continue
			}

			_ = tx.Rollback()

			recordSpanError(span, err)

			return fmt.Errorf("insert failed: %w", err)
		}
	}

	if orphaned > 0 {
		span.SetAttributes(attribute.Int("ella.db.batch_orphaned", orphaned))
		logger.DBLog.Warn("Dropped flow reports for deleted subscribers",
			zap.Int("dropped", orphaned),
			zap.Int("batch_size", len(flowReports)),
		)
	}

	if err := tx.Commit(); err != nil {
		recordSpanError(span, err)

		return fmt.Errorf("commit failed: %w", err)
	}

	return nil
}

func (db *Database) ListFlowReports(ctx context.Context, page int, perPage int, filters *FlowReportFilters) ([]dbwriter.FlowReport, int, error) {
	querySummary := fmt.Sprintf("%s %s (paged+filtered)", "SELECT", FlowReportsTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			semconv.DBCollectionName(FlowReportsTableName),
			attribute.Int("ella.db.page", page),
			attribute.Int("ella.db.page_size", perPage),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(FlowReportsTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(FlowReportsTableName, "select").Inc()

	if filters == nil {
		filters = &FlowReportFilters{}
	}

	args := ListArgs{
		Limit:  perPage,
		Offset: (page - 1) * perPage,
	}

	var reports []dbwriter.FlowReport

	var counts []NumItems

	err := db.conn().Query(ctx, db.listFlowReportsStmt, args, filters).GetAll(&reports, &counts)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			var fallbackCount NumItems

			countErr := db.conn().Query(ctx, db.countFlowReportsStmt, filters).Get(&fallbackCount)
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

	return reports, count, nil
}

// DeleteOldFlowReports removes flow reports older than the specified retention period in days.
func (db *Database) DeleteOldFlowReports(ctx context.Context, days int) error {
	querySummary := fmt.Sprintf("%s %s (retention)", "DELETE", FlowReportsTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("DELETE"),
			semconv.DBCollectionName(FlowReportsTableName),
			attribute.Int("retention.days", days),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(FlowReportsTableName, "delete"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(FlowReportsTableName, "delete").Inc()

	cutoff := dbwriter.EpochMillis(time.Now().UTC().AddDate(0, 0, -days))

	args := cutoffArgs{Cutoff: cutoff}

	err := db.conn().Query(ctx, db.deleteOldFlowReportsStmt, args).Run()
	if err != nil {
		recordSpanError(span, err)

		return fmt.Errorf("query failed: %w", err)
	}

	return nil
}

func (db *Database) ClearFlowReports(ctx context.Context) error {
	querySummary := fmt.Sprintf("%s %s (all)", "DELETE", FlowReportsTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("DELETE"),
			semconv.DBCollectionName(FlowReportsTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(FlowReportsTableName, "delete"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(FlowReportsTableName, "delete").Inc()

	err := db.conn().Query(ctx, db.deleteAllFlowReportsStmt).Run()
	if err != nil {
		recordSpanError(span, err)

		return fmt.Errorf("query failed: %w", err)
	}

	return nil
}

func (db *Database) ListFlowReportsByDay(ctx context.Context, filters *FlowReportFilters) ([]dbwriter.FlowReport, error) {
	querySummary := fmt.Sprintf("%s %s (by day)", "SELECT", FlowReportsTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			semconv.DBCollectionName(FlowReportsTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(FlowReportsTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(FlowReportsTableName, "select").Inc()

	if filters == nil {
		filters = &FlowReportFilters{}
	}

	var results []dbwriter.FlowReport

	err := db.conn().Query(ctx, db.listFlowReportsByDayStmt, filters).GetAll(&results)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}

		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return results, nil
}

func (db *Database) ListFlowReportsBySubscriber(ctx context.Context, filters *FlowReportFilters) ([]dbwriter.FlowReport, error) {
	querySummary := fmt.Sprintf("%s %s (by subscriber)", "SELECT", FlowReportsTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			semconv.DBCollectionName(FlowReportsTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(FlowReportsTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(FlowReportsTableName, "select").Inc()

	if filters == nil {
		filters = &FlowReportFilters{}
	}

	var results []dbwriter.FlowReport

	err := db.conn().Query(ctx, db.listFlowReportsBySubscriberStmt, filters).GetAll(&results)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}

		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return results, nil
}

// GetFlowReportStats returns aggregated protocol counts and top destination IPs for uplink traffic.
func (db *Database) GetFlowReportStats(ctx context.Context, filters *FlowReportFilters) ([]FlowReportProtocolCount, []FlowReportIPCount, error) {
	querySummary := fmt.Sprintf("%s %s (stats)", "SELECT", FlowReportsTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			semconv.DBCollectionName(FlowReportsTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(FlowReportsTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(FlowReportsTableName, "select").Inc()

	if filters == nil {
		filters = &FlowReportFilters{}
	}

	var protocols []FlowReportProtocolCount

	err := db.conn().Query(ctx, db.flowReportProtocolCountsStmt, filters).GetAll(&protocols)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		recordSpanError(span, err)

		return nil, nil, fmt.Errorf("protocol counts query failed: %w", err)
	}

	var destinationsUplink []FlowReportIPCount

	err = db.conn().Query(ctx, db.flowReportTopDestinationsUplinkStmt, filters).GetAll(&destinationsUplink)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		recordSpanError(span, err)

		return nil, nil, fmt.Errorf("top destinations uplink query failed: %w", err)
	}

	return protocols, destinationsUplink, nil
}
