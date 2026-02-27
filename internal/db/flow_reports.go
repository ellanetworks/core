// SPDX-FileCopyrightText: 2026-present Ella Networks
// SPDX-License-Identifier: Apache-2.0

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

const FlowReportsTableName = "flow_reports"

// Schema for flow reports table
const QueryCreateFlowReportsTable = `
	CREATE TABLE IF NOT EXISTS %s (
		id              INTEGER PRIMARY KEY AUTOINCREMENT,
		subscriber_id   TEXT NOT NULL,              -- IMSI (looked up from PDR ID)
		source_ip       TEXT NOT NULL,              -- IP address as string
		destination_ip  TEXT NOT NULL,              -- IP address as string
		source_port     INTEGER NOT NULL DEFAULT 0, -- 0 if N/A
		destination_port INTEGER NOT NULL DEFAULT 0,-- 0 if N/A
		protocol        INTEGER NOT NULL,           -- IP protocol number
		packets         INTEGER NOT NULL,           -- Total packets
		bytes           INTEGER NOT NULL,           -- Total bytes
		start_time      TEXT NOT NULL,              -- RFC3339
		end_time        TEXT NOT NULL,              -- RFC3339

		FOREIGN KEY (subscriber_id) REFERENCES subscribers(imsi) ON DELETE CASCADE
	);`

const QueryCreateFlowReportsIndex = `
	CREATE INDEX IF NOT EXISTS idx_flow_reports_subscriber_id ON flow_reports (subscriber_id);
	CREATE INDEX IF NOT EXISTS idx_flow_reports_end_time ON flow_reports (end_time);
	CREATE INDEX IF NOT EXISTS idx_flow_reports_protocol ON flow_reports (protocol);
	CREATE INDEX IF NOT EXISTS idx_flow_reports_source_ip ON flow_reports (source_ip);
	CREATE INDEX IF NOT EXISTS idx_flow_reports_destination_ip ON flow_reports (destination_ip);
`

const (
	insertFlowReportStmt     = "INSERT INTO %s (subscriber_id, source_ip, destination_ip, source_port, destination_port, protocol, packets, bytes, start_time, end_time) VALUES ($FlowReport.subscriber_id, $FlowReport.source_ip, $FlowReport.destination_ip, $FlowReport.source_port, $FlowReport.destination_port, $FlowReport.protocol, $FlowReport.packets, $FlowReport.bytes, $FlowReport.start_time, $FlowReport.end_time)"
	getFlowReportByIDStmt    = "SELECT &FlowReport.* FROM %s WHERE id = $FlowReport.id"
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
    AND ($FlowReportFilters.end_time_from IS NULL OR end_time >= $FlowReportFilters.end_time_from)
    AND ($FlowReportFilters.end_time_to IS NULL OR end_time < $FlowReportFilters.end_time_to)
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
    AND ($FlowReportFilters.end_time_from IS NULL OR end_time >= $FlowReportFilters.end_time_from)
    AND ($FlowReportFilters.end_time_to IS NULL OR end_time < $FlowReportFilters.end_time_to)
`

const listFlowReportsFilteredByDayStmt = `
SELECT &FlowReport.*
FROM %s
WHERE
    ($FlowReportFilters.subscriber_id IS NULL OR subscriber_id = $FlowReportFilters.subscriber_id)
    AND ($FlowReportFilters.protocol IS NULL OR protocol = $FlowReportFilters.protocol)
    AND ($FlowReportFilters.source_ip IS NULL OR source_ip = $FlowReportFilters.source_ip)
    AND ($FlowReportFilters.destination_ip IS NULL OR destination_ip = $FlowReportFilters.destination_ip)
    AND ($FlowReportFilters.end_time_from IS NULL OR end_time >= $FlowReportFilters.end_time_from)
    AND ($FlowReportFilters.end_time_to IS NULL OR end_time < $FlowReportFilters.end_time_to)
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
    AND ($FlowReportFilters.end_time_from IS NULL OR end_time >= $FlowReportFilters.end_time_from)
    AND ($FlowReportFilters.end_time_to IS NULL OR end_time < $FlowReportFilters.end_time_to)
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
    AND ($FlowReportFilters.end_time_from IS NULL OR end_time >= $FlowReportFilters.end_time_from)
    AND ($FlowReportFilters.end_time_to IS NULL OR end_time < $FlowReportFilters.end_time_to)
GROUP BY protocol
ORDER BY COUNT(*) DESC
`

const flowReportTopSourcesStmt = `
SELECT source_ip AS &FlowReportIPCount.ip, COUNT(*) AS &FlowReportIPCount.count
FROM %s
WHERE
    ($FlowReportFilters.subscriber_id IS NULL OR subscriber_id = $FlowReportFilters.subscriber_id)
    AND ($FlowReportFilters.protocol IS NULL OR protocol = $FlowReportFilters.protocol)
    AND ($FlowReportFilters.source_ip IS NULL OR source_ip = $FlowReportFilters.source_ip)
    AND ($FlowReportFilters.destination_ip IS NULL OR destination_ip = $FlowReportFilters.destination_ip)
    AND ($FlowReportFilters.end_time_from IS NULL OR end_time >= $FlowReportFilters.end_time_from)
    AND ($FlowReportFilters.end_time_to IS NULL OR end_time < $FlowReportFilters.end_time_to)
GROUP BY source_ip
ORDER BY COUNT(*) DESC
LIMIT 10
`

const flowReportTopDestinationsStmt = `
SELECT destination_ip AS &FlowReportIPCount.ip, COUNT(*) AS &FlowReportIPCount.count
FROM %s
WHERE
    ($FlowReportFilters.subscriber_id IS NULL OR subscriber_id = $FlowReportFilters.subscriber_id)
    AND ($FlowReportFilters.protocol IS NULL OR protocol = $FlowReportFilters.protocol)
    AND ($FlowReportFilters.source_ip IS NULL OR source_ip = $FlowReportFilters.source_ip)
    AND ($FlowReportFilters.destination_ip IS NULL OR destination_ip = $FlowReportFilters.destination_ip)
    AND ($FlowReportFilters.end_time_from IS NULL OR end_time >= $FlowReportFilters.end_time_from)
    AND ($FlowReportFilters.end_time_to IS NULL OR end_time < $FlowReportFilters.end_time_to)
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
	SubscriberID  *string `db:"subscriber_id"`  // exact match (IMSI)
	Protocol      *uint8  `db:"protocol"`       // exact match
	SourceIP      *string `db:"source_ip"`      // exact match
	DestinationIP *string `db:"destination_ip"` // exact match
	EndTimeFrom   *string `db:"end_time_from"`  // RFC3339 (UTC)
	EndTimeTo     *string `db:"end_time_to"`    // RFC3339 (UTC), exclusive upper bound
}

func (db *Database) InsertFlowReport(ctx context.Context, flowReport *dbwriter.FlowReport) error {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "INSERT", FlowReportsTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("INSERT"),
			attribute.String("db.collection", FlowReportsTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(FlowReportsTableName, "insert"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(FlowReportsTableName, "insert").Inc()

	err := db.conn.Query(ctx, db.insertFlowReportStmt, flowReport).Run()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

func (db *Database) ListFlowReports(ctx context.Context, page int, perPage int, filters *FlowReportFilters) ([]dbwriter.FlowReport, int, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s (paged+filtered)", "SELECT", FlowReportsTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("SELECT"),
			attribute.String("db.collection", FlowReportsTableName),
			attribute.Int("page", page),
			attribute.Int("per_page", perPage),
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

	err := db.conn.Query(ctx, db.listFlowReportsStmt, args, filters).GetAll(&reports, &counts)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			span.SetStatus(codes.Ok, "no rows")

			var fallbackCount NumItems

			countErr := db.conn.Query(ctx, db.countFlowReportsStmt, filters).Get(&fallbackCount)
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

	return reports, count, nil
}

// DeleteOldFlowReports removes flow reports older than the specified retention period in days.
func (db *Database) DeleteOldFlowReports(ctx context.Context, days int) error {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s (retention)", "DELETE", FlowReportsTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("DELETE"),
			attribute.String("db.collection", FlowReportsTableName),
			attribute.Int("retention.days", days),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(FlowReportsTableName, "delete"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(FlowReportsTableName, "delete").Inc()

	// Compute UTC cutoff so string comparison works lexicographically for RFC3339
	cutoff := time.Now().AddDate(0, 0, -days).UTC().Format(time.RFC3339)

	args := cutoffArgs{Cutoff: cutoff}

	err := db.conn.Query(ctx, db.deleteOldFlowReportsStmt, args).Run()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

func (db *Database) ClearFlowReports(ctx context.Context) error {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s (all)", "DELETE", FlowReportsTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("DELETE"),
			attribute.String("db.collection", FlowReportsTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(FlowReportsTableName, "delete"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(FlowReportsTableName, "delete").Inc()

	err := db.conn.Query(ctx, db.deleteAllFlowReportsStmt).Run()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

func (db *Database) ListFlowReportsByDay(ctx context.Context, filters *FlowReportFilters) ([]dbwriter.FlowReport, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s (by day)", "SELECT", FlowReportsTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("SELECT"),
			attribute.String("db.collection", FlowReportsTableName),
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

	err := db.conn.Query(ctx, db.listFlowReportsByDayStmt, filters).GetAll(&results)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			span.SetStatus(codes.Ok, "no rows")
			return nil, nil
		}

		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return nil, fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return results, nil
}

func (db *Database) ListFlowReportsBySubscriber(ctx context.Context, filters *FlowReportFilters) ([]dbwriter.FlowReport, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s (by subscriber)", "SELECT", FlowReportsTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("SELECT"),
			attribute.String("db.collection", FlowReportsTableName),
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

	err := db.conn.Query(ctx, db.listFlowReportsBySubscriberStmt, filters).GetAll(&results)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			span.SetStatus(codes.Ok, "no rows")
			return nil, nil
		}

		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return nil, fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return results, nil
}

// GetFlowReportStats returns aggregated protocol counts, top source IPs, and top destination IPs.
func (db *Database) GetFlowReportStats(ctx context.Context, filters *FlowReportFilters) ([]FlowReportProtocolCount, []FlowReportIPCount, []FlowReportIPCount, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s (stats)", "SELECT", FlowReportsTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("SELECT"),
			attribute.String("db.collection", FlowReportsTableName),
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

	err := db.conn.Query(ctx, db.flowReportProtocolCountsStmt, filters).GetAll(&protocols)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		span.RecordError(err)
		span.SetStatus(codes.Error, "protocol counts query failed")

		return nil, nil, nil, fmt.Errorf("protocol counts query failed: %w", err)
	}

	var sources []FlowReportIPCount

	err = db.conn.Query(ctx, db.flowReportTopSourcesStmt, filters).GetAll(&sources)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		span.RecordError(err)
		span.SetStatus(codes.Error, "top sources query failed")

		return nil, nil, nil, fmt.Errorf("top sources query failed: %w", err)
	}

	var destinations []FlowReportIPCount

	err = db.conn.Query(ctx, db.flowReportTopDestinationsStmt, filters).GetAll(&destinations)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		span.RecordError(err)
		span.SetStatus(codes.Error, "top destinations query failed")

		return nil, nil, nil, fmt.Errorf("top destinations query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return protocols, sources, destinations, nil
}
