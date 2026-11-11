// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
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
	insertFlowReportStmt            = "INSERT INTO " + FlowReportsTableName + " (subscriber_id, source_ip, destination_ip, source_port, destination_port, protocol, packets, bytes, start_time, end_time, direction, action) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
	insertFlowReportOfKnownIMSIStmt = "INSERT INTO " + FlowReportsTableName + " (subscriber_id, source_ip, destination_ip, source_port, destination_port, protocol, packets, bytes, start_time, end_time, direction, action) SELECT ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ? WHERE EXISTS (SELECT 1 FROM " + SubscribersTableName + " WHERE imsi = ?) OR EXISTS (SELECT 1 FROM " + VisitorsTableName + " WHERE imsi = ?)"
	deleteOldFlowReportsStmt        = "DELETE FROM " + FlowReportsTableName + " WHERE end_time < ?"
	deleteAllFlowReportsStmt        = "DELETE FROM " + FlowReportsTableName
)

const listFlowReportsPagedFilteredStmt = `
  SELECT id, subscriber_id, source_ip, destination_ip, source_port, destination_port, protocol, packets, bytes, start_time, end_time, direction, action, COUNT(*) OVER()
  FROM ` + FlowReportsTableName + `
  WHERE
    (?1 IS NULL OR subscriber_id = ?1)
    AND (?2 IS NULL OR protocol = ?2)
    AND (?3 IS NULL OR source_ip = ?3)
    AND (?4 IS NULL OR destination_ip = ?4)
    AND (?5 IS NULL OR source_port = ?5)
    AND (?6 IS NULL OR destination_port = ?6)
    AND (?7 IS NULL OR end_time >= ?7)
    AND (?8 IS NULL OR start_time < ?8)
    AND (?9 IS NULL OR direction = ?9)
    AND (?10 IS NULL OR action = ?10)
  ORDER BY id DESC
  LIMIT ?11
  OFFSET ?12
`

const countFlowReportsFilteredStmt = `
  SELECT COUNT(*)
  FROM ` + FlowReportsTableName + `
  WHERE
    (?1 IS NULL OR subscriber_id = ?1)
    AND (?2 IS NULL OR protocol = ?2)
    AND (?3 IS NULL OR source_ip = ?3)
    AND (?4 IS NULL OR destination_ip = ?4)
    AND (?5 IS NULL OR source_port = ?5)
    AND (?6 IS NULL OR destination_port = ?6)
    AND (?7 IS NULL OR end_time >= ?7)
    AND (?8 IS NULL OR start_time < ?8)
    AND (?9 IS NULL OR direction = ?9)
    AND (?10 IS NULL OR action = ?10)
`

const listFlowReportsFilteredByDayStmt = `
SELECT id, subscriber_id, source_ip, destination_ip, source_port, destination_port, protocol, packets, bytes, start_time, end_time, direction, action
FROM ` + FlowReportsTableName + `
WHERE
    (?1 IS NULL OR subscriber_id = ?1)
    AND (?2 IS NULL OR protocol = ?2)
    AND (?3 IS NULL OR source_ip = ?3)
    AND (?4 IS NULL OR destination_ip = ?4)
    AND (?5 IS NULL OR source_port = ?5)
    AND (?6 IS NULL OR destination_port = ?6)
    AND (?7 IS NULL OR end_time >= ?7)
    AND (?8 IS NULL OR start_time < ?8)
    AND (?9 IS NULL OR direction = ?9)
    AND (?10 IS NULL OR action = ?10)
ORDER BY end_time ASC
`

const listFlowReportsFilteredBySubscriberStmt = `
SELECT id, subscriber_id, source_ip, destination_ip, source_port, destination_port, protocol, packets, bytes, start_time, end_time, direction, action
FROM ` + FlowReportsTableName + `
WHERE
    (?1 IS NULL OR subscriber_id = ?1)
    AND (?2 IS NULL OR protocol = ?2)
    AND (?3 IS NULL OR source_ip = ?3)
    AND (?4 IS NULL OR destination_ip = ?4)
    AND (?5 IS NULL OR source_port = ?5)
    AND (?6 IS NULL OR destination_port = ?6)
    AND (?7 IS NULL OR end_time >= ?7)
    AND (?8 IS NULL OR start_time < ?8)
    AND (?9 IS NULL OR direction = ?9)
    AND (?10 IS NULL OR action = ?10)
ORDER BY subscriber_id ASC, end_time ASC
`

const flowReportProtocolCountsStmt = `
SELECT protocol, COUNT(*)
FROM ` + FlowReportsTableName + `
WHERE
    (?1 IS NULL OR subscriber_id = ?1)
    AND (?2 IS NULL OR protocol = ?2)
    AND (?3 IS NULL OR source_ip = ?3)
    AND (?4 IS NULL OR destination_ip = ?4)
    AND (?5 IS NULL OR source_port = ?5)
    AND (?6 IS NULL OR destination_port = ?6)
    AND (?7 IS NULL OR end_time >= ?7)
    AND (?8 IS NULL OR start_time < ?8)
    AND (?9 IS NULL OR direction = ?9)
    AND (?10 IS NULL OR action = ?10)
GROUP BY protocol
ORDER BY COUNT(*) DESC
`

const flowReportTopDestinationsUplinkStmt = `
SELECT destination_ip, COUNT(*)
FROM ` + FlowReportsTableName + `
WHERE
    (?1 IS NULL OR subscriber_id = ?1)
    AND (?2 IS NULL OR protocol = ?2)
    AND (?3 IS NULL OR source_ip = ?3)
    AND (?4 IS NULL OR destination_ip = ?4)
    AND (?5 IS NULL OR source_port = ?5)
    AND (?6 IS NULL OR destination_port = ?6)
    AND (?7 IS NULL OR end_time >= ?7)
    AND (?8 IS NULL OR start_time < ?8)
    AND direction = 'uplink'
    AND (?10 IS NULL OR action = ?10)
GROUP BY destination_ip
ORDER BY COUNT(*) DESC
LIMIT 10
`

type FlowReportProtocolCount struct {
	Protocol uint8
	Count    int
}

type FlowReportIPCount struct {
	IP    string
	Count int
}

type FlowReportFilters struct {
	SubscriberID    *string // exact match (IMSI)
	Protocol        *uint8  // exact match
	SourceIP        *string // exact match
	DestinationIP   *string // exact match
	SourcePort      *uint16 // exact match
	DestinationPort *uint16 // exact match
	RangeStart      *int64  // epoch milliseconds, inclusive
	RangeEnd        *int64  // epoch milliseconds, exclusive
	Direction       *string // "uplink" or "downlink"
	Action          *uint8  // 0 = "allow", 1 = "drop"
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

	tx, err := db.writer().BeginTx(ctx, nil)
	if err != nil {
		recordSpanError(span, err)

		return fmt.Errorf("begin transaction failed: %w", err)
	}

	orphaned := 0

	knownIMSIOnly := db.appliedSchemaAtLeast(ctx, roamingSchema)

	for _, fr := range flowReports {
		if knownIMSIOnly {
			inserted, err := insertFlowReportOfKnownIMSI(ctx, tx, fr)
			if err != nil {
				_ = tx.Rollback()

				recordSpanError(span, err)

				return fmt.Errorf("insert failed: %w", err)
			}

			if !inserted {
				orphaned++
			}

			continue
		}

		if _, err := tx.ExecContext(ctx, insertFlowReportStmt, fr.SubscriberID, fr.SourceIP, fr.DestinationIP, fr.SourcePort, fr.DestinationPort, fr.Protocol, fr.Packets, fr.Bytes, fr.StartTime, fr.EndTime, fr.Direction, fr.Action); err != nil {
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
	ctx, span, timer := startDBQueryNamed(ctx, "SELECT "+FlowReportsTableName+" (paged+filtered)", FlowReportsTableName, "SELECT", "select", attribute.Int("ella.db.page", page), attribute.Int("ella.db.page_size", perPage))
	defer span.End()
	defer timer.ObserveDuration()

	if filters == nil {
		filters = &FlowReportFilters{}
	}

	reports, count, err := queryPage(ctx, db.reader(), listFlowReportsPagedFilteredStmt, scanFlowReport, filters.SubscriberID, filters.Protocol, filters.SourceIP, filters.DestinationIP, filters.SourcePort, filters.DestinationPort, filters.RangeStart, filters.RangeEnd, filters.Direction, filters.Action, perPage, (page-1)*perPage)
	if err != nil {
		recordSpanError(span, err)

		return nil, 0, fmt.Errorf("query failed: %w", err)
	}

	if len(reports) == 0 {
		fallbackCount, countErr := queryCount(ctx, db.reader(), countFlowReportsFilteredStmt, filters.SubscriberID, filters.Protocol, filters.SourceIP, filters.DestinationIP, filters.SourcePort, filters.DestinationPort, filters.RangeStart, filters.RangeEnd, filters.Direction, filters.Action)
		if countErr != nil {
			return nil, 0, nil
		}

		return nil, fallbackCount, nil
	}

	return reports, count, nil
}

// DeleteOldFlowReports removes flow reports older than the specified retention period in days.
func (db *Database) DeleteOldFlowReports(ctx context.Context, days int) error {
	ctx, span, timer := startDBQueryNamed(ctx, "DELETE "+FlowReportsTableName+" (retention)", FlowReportsTableName, "DELETE", "delete", attribute.Int("retention.days", days))
	defer span.End()
	defer timer.ObserveDuration()

	cutoff := dbwriter.EpochMillis(time.Now().UTC().AddDate(0, 0, -days))

	_, err := db.writer().ExecContext(ctx, deleteOldFlowReportsStmt, cutoff)
	if err != nil {
		recordSpanError(span, err)

		return fmt.Errorf("query failed: %w", err)
	}

	return nil
}

func (db *Database) ClearFlowReports(ctx context.Context) error {
	ctx, span, timer := startDBQueryNamed(ctx, "DELETE "+FlowReportsTableName+" (all)", FlowReportsTableName, "DELETE", "delete")
	defer span.End()
	defer timer.ObserveDuration()

	_, err := db.writer().ExecContext(ctx, deleteAllFlowReportsStmt)
	if err != nil {
		recordSpanError(span, err)

		return fmt.Errorf("query failed: %w", err)
	}

	return nil
}

func (db *Database) ListFlowReportsByDay(ctx context.Context, filters *FlowReportFilters) ([]dbwriter.FlowReport, error) {
	ctx, span, timer := startDBQueryNamed(ctx, "SELECT "+FlowReportsTableName+" (by day)", FlowReportsTableName, "SELECT", "select")
	defer span.End()
	defer timer.ObserveDuration()

	if filters == nil {
		filters = &FlowReportFilters{}
	}

	results, err := queryAll(ctx, db.reader(), listFlowReportsFilteredByDayStmt, scanFlowReport, filters.SubscriberID, filters.Protocol, filters.SourceIP, filters.DestinationIP, filters.SourcePort, filters.DestinationPort, filters.RangeStart, filters.RangeEnd, filters.Direction, filters.Action)
	if err != nil {
		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return results, nil
}

func (db *Database) ListFlowReportsBySubscriber(ctx context.Context, filters *FlowReportFilters) ([]dbwriter.FlowReport, error) {
	ctx, span, timer := startDBQueryNamed(ctx, "SELECT "+FlowReportsTableName+" (by subscriber)", FlowReportsTableName, "SELECT", "select")
	defer span.End()
	defer timer.ObserveDuration()

	if filters == nil {
		filters = &FlowReportFilters{}
	}

	results, err := queryAll(ctx, db.reader(), listFlowReportsFilteredBySubscriberStmt, scanFlowReport, filters.SubscriberID, filters.Protocol, filters.SourceIP, filters.DestinationIP, filters.SourcePort, filters.DestinationPort, filters.RangeStart, filters.RangeEnd, filters.Direction, filters.Action)
	if err != nil {
		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return results, nil
}

// GetFlowReportStats returns aggregated protocol counts and top destination IPs for uplink traffic.
func (db *Database) GetFlowReportStats(ctx context.Context, filters *FlowReportFilters) ([]FlowReportProtocolCount, []FlowReportIPCount, error) {
	ctx, span, timer := startDBQueryNamed(ctx, "SELECT "+FlowReportsTableName+" (stats)", FlowReportsTableName, "SELECT", "select")
	defer span.End()
	defer timer.ObserveDuration()

	if filters == nil {
		filters = &FlowReportFilters{}
	}

	protocols, err := queryAll(ctx, db.reader(), flowReportProtocolCountsStmt, scanFlowReportProtocolCount, filters.SubscriberID, filters.Protocol, filters.SourceIP, filters.DestinationIP, filters.SourcePort, filters.DestinationPort, filters.RangeStart, filters.RangeEnd, filters.Direction, filters.Action)
	if err != nil {
		recordSpanError(span, err)

		return nil, nil, fmt.Errorf("protocol counts query failed: %w", err)
	}

	destinationsUplink, err := queryAll(ctx, db.reader(), flowReportTopDestinationsUplinkStmt, scanFlowReportIPCount, filters.SubscriberID, filters.Protocol, filters.SourceIP, filters.DestinationIP, filters.SourcePort, filters.DestinationPort, filters.RangeStart, filters.RangeEnd, filters.Direction, filters.Action)
	if err != nil {
		recordSpanError(span, err)

		return nil, nil, fmt.Errorf("top destinations uplink query failed: %w", err)
	}

	return protocols, destinationsUplink, nil
}

func insertFlowReportOfKnownIMSI(ctx context.Context, tx *sql.Tx, fr *dbwriter.FlowReport) (bool, error) {
	res, err := tx.ExecContext(ctx, insertFlowReportOfKnownIMSIStmt, fr.SubscriberID, fr.SourceIP, fr.DestinationIP, fr.SourcePort, fr.DestinationPort, fr.Protocol, fr.Packets, fr.Bytes, fr.StartTime, fr.EndTime, fr.Direction, fr.Action, fr.SubscriberID, fr.SubscriberID)
	if err != nil {
		return false, err
	}

	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}

	return n > 0, nil
}

func scanFlowReport(row scanner) (dbwriter.FlowReport, error) {
	var v dbwriter.FlowReport

	if err := row.Scan(&v.ID, &v.SubscriberID, &v.SourceIP, &v.DestinationIP, &v.SourcePort, &v.DestinationPort, &v.Protocol, &v.Packets, &v.Bytes, &v.StartTime, &v.EndTime, &v.Direction, &v.Action); err != nil {
		return dbwriter.FlowReport{}, err
	}

	return v, nil
}

func scanFlowReportProtocolCount(row scanner) (FlowReportProtocolCount, error) {
	var v FlowReportProtocolCount

	if err := row.Scan(&v.Protocol, &v.Count); err != nil {
		return FlowReportProtocolCount{}, err
	}

	return v, nil
}

func scanFlowReportIPCount(row scanner) (FlowReportIPCount, error) {
	var v FlowReportIPCount

	if err := row.Scan(&v.IP, &v.Count); err != nil {
		return FlowReportIPCount{}, err
	}

	return v, nil
}
