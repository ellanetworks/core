// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// SPDX-FileCopyrightText: Ella Networks Inc.

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

const DailyUsageTableName = "daily_usage"

const (
	incrementDailyUsageStmt = "INSERT INTO %s (epoch_day, imsi, bytes_uplink, bytes_downlink) VALUES ($DailyUsage.epoch_day, $DailyUsage.imsi, $DailyUsage.bytes_uplink, $DailyUsage.bytes_downlink) ON CONFLICT(epoch_day, imsi) DO UPDATE SET bytes_uplink = bytes_uplink + $DailyUsage.bytes_uplink, bytes_downlink = bytes_downlink + $DailyUsage.bytes_downlink"
	deleteOldDailyUsageStmt = "DELETE FROM %s WHERE epoch_day < $cutoffDaysArgs.cutoff_days"
	deleteAllDailyUsageStmt = "DELETE FROM %s"
)

const (
	getUsagePerDayStmt = `
SELECT
    epoch_day AS &UsagePerDay.epoch_day,
    SUM(bytes_uplink)   AS &UsagePerDay.bytes_uplink,
    SUM(bytes_downlink) AS &UsagePerDay.bytes_downlink
FROM %s
WHERE
    ($UsageFilters.imsi IS NULL OR imsi == $UsageFilters.imsi)
    AND epoch_day >= $UsageFilters.start_date
    AND epoch_day <= $UsageFilters.end_date
GROUP BY epoch_day
ORDER BY epoch_day ASC`
)

const (
	getUsagePerSubscriberStmt = `
SELECT
    s.imsi AS &UsagePerSub.imsi,
    COALESCE(SUM(u.bytes_uplink), 0)   AS &UsagePerSub.bytes_uplink,
    COALESCE(SUM(u.bytes_downlink), 0) AS &UsagePerSub.bytes_downlink
FROM %s AS s
LEFT JOIN %s AS u
    ON u.imsi = s.imsi
    AND u.epoch_day >= $UsageFilters.start_date
    AND u.epoch_day <= $UsageFilters.end_date
WHERE ($UsageFilters.imsi IS NULL OR s.imsi = $UsageFilters.imsi)
GROUP BY s.imsi
ORDER BY COALESCE(SUM(u.bytes_uplink), 0) + COALESCE(SUM(u.bytes_downlink), 0) DESC, s.imsi ASC
LIMIT $UsageFilters.limit`
)

type UsagePerDay struct {
	EpochDay      int64 `db:"epoch_day"`
	BytesUplink   int64 `db:"bytes_uplink"`
	BytesDownlink int64 `db:"bytes_downlink"`
}

type UsagePerSub struct {
	IMSI          string `db:"imsi"`
	BytesUplink   int64  `db:"bytes_uplink"`
	BytesDownlink int64  `db:"bytes_downlink"`
}

type DailyUsage struct {
	EpochDay      int64  `db:"epoch_day"`
	IMSI          string `db:"imsi"`
	BytesUplink   int64  `db:"bytes_uplink"`
	BytesDownlink int64  `db:"bytes_downlink"`
}

type UsageFilters struct {
	IMSI      *string `db:"imsi"` // exact match
	StartDate int64   `db:"start_date"`
	EndDate   int64   `db:"end_date"`
	Limit     int64   `db:"limit"` // SQLite treats a negative limit as unbounded
}

// NoUsageLimit returns every matching row.
const NoUsageLimit int64 = -1

func DaysSinceEpoch(t time.Time) int64 {
	t = t.UTC()
	y, m, d := t.Date()

	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC).Unix() / 86400
}

type DayRange struct {
	First int64
	Last  int64
}

func NewDayRange(start time.Time, end time.Time) DayRange {
	first := DaysSinceEpoch(start)

	if !end.After(start) {
		return DayRange{First: first, Last: first - 1}
	}

	return DayRange{First: first, Last: DaysSinceEpoch(end.Add(-time.Nanosecond))}
}

func (r DayRange) Len() int64 {
	if r.Last < r.First {
		return 0
	}

	return r.Last - r.First + 1
}

func (r DayRange) Day(offset int64) time.Time {
	return time.Unix((r.First+offset)*86400, 0).UTC()
}

func (d *DailyUsage) GetDay() time.Time {
	return time.Unix(d.EpochDay*86400, 0).UTC()
}

func (d *UsagePerDay) GetDay() time.Time {
	return time.Unix(d.EpochDay*86400, 0).UTC()
}

func (db *Database) IncrementDailyUsage(ctx context.Context, usage DailyUsage) error {
	querySummary := fmt.Sprintf("%s %s", "INSERT", DailyUsageTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("INSERT"),
			semconv.DBCollectionName(DailyUsageTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(DailyUsageTableName, "insert"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(DailyUsageTableName, "insert").Inc()

	_, err := opIncrementDailyUsage.Invoke(ctx, db, &usage)
	if err != nil {
		recordSpanError(span, err)

		return err
	}

	return nil
}

type DailyUsageBatch struct {
	Rows []DailyUsage
}

type droppedDailyUsage struct {
	Rows          int
	BytesUplink   int64
	BytesDownlink int64
}

func (db *Database) IncrementDailyUsageBatch(ctx context.Context, usages []DailyUsage) error {
	if len(usages) == 0 {
		return nil
	}

	operation := "INSERT"

	var batchAttrs []attribute.KeyValue

	if len(usages) > 1 {
		operation = "BATCH INSERT"
		batchAttrs = []attribute.KeyValue{semconv.DBOperationBatchSize(len(usages))}
	}

	querySummary := fmt.Sprintf("%s %s", operation, DailyUsageTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName(operation),
			semconv.DBCollectionName(DailyUsageTableName),
		),
		trace.WithAttributes(batchAttrs...),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(DailyUsageTableName, "batch_insert"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(DailyUsageTableName, "batch_insert").Inc()

	dropped, err := opIncrementDailyUsageBatch.Invoke(ctx, db, &DailyUsageBatch{Rows: usages})
	if err != nil {
		recordSpanError(span, err)

		return err
	}

	if dropped.Rows > 0 {
		logger.From(ctx, logger.DBLog).Error("usage bytes lost: no subscriber row to charge",
			zap.Int("rows", dropped.Rows),
			logger.UplinkVolume(uint64(dropped.BytesUplink)),
			logger.DownlinkVolume(uint64(dropped.BytesDownlink)))
	}

	return nil
}

func (db *Database) GetUsagePerDay(ctx context.Context, imsi string, days DayRange) ([]UsagePerDay, error) {
	querySummary := fmt.Sprintf("%s %s", "SELECT", DailyUsageTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			semconv.DBCollectionName(DailyUsageTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(DailyUsageTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(DailyUsageTableName, "select").Inc()

	dailyUsageFilters := UsageFilters{
		StartDate: days.First,
		EndDate:   days.Last,
		Limit:     NoUsageLimit,
	}

	if imsi != "" {
		dailyUsageFilters.IMSI = &imsi
	}

	var dailyUsage []UsagePerDay

	err := db.conn().Query(ctx, db.getUsagePerDayStmt, dailyUsageFilters).GetAll(&dailyUsage)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}

		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return dailyUsage, nil
}

func (db *Database) GetUsagePerSubscriber(ctx context.Context, imsi string, days DayRange, limit int64) ([]UsagePerSub, error) {
	querySummary := fmt.Sprintf("%s %s", "SELECT", DailyUsageTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			semconv.DBCollectionName(DailyUsageTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(DailyUsageTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(DailyUsageTableName, "select").Inc()

	dailyUsageFilters := UsageFilters{
		StartDate: days.First,
		EndDate:   days.Last,
		Limit:     limit,
	}

	if imsi != "" {
		dailyUsageFilters.IMSI = &imsi
	}

	var dailyUsage []UsagePerSub

	err := db.conn().Query(ctx, db.getUsagePerSubscriberStmt, dailyUsageFilters).GetAll(&dailyUsage)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}

		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return dailyUsage, nil
}

func (db *Database) ClearDailyUsage(ctx context.Context) error {
	querySummary := fmt.Sprintf("%s %s", "DELETE", DailyUsageTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("DELETE"),
			semconv.DBCollectionName(DailyUsageTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(DailyUsageTableName, "delete"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(DailyUsageTableName, "delete").Inc()

	_, err := opClearDailyUsage.Invoke(ctx, db, &emptyPayload{})
	if err != nil {
		recordSpanError(span, err)

		return err
	}

	return nil
}

func (db *Database) DeleteOldDailyUsage(ctx context.Context, days int) error {
	querySummary := fmt.Sprintf("%s %s (retention)", "DELETE", DailyUsageTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("DELETE"),
			semconv.DBCollectionName(DailyUsageTableName),
			attribute.Int("retention.days", days),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(DailyUsageTableName, "delete"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(DailyUsageTableName, "delete").Inc()

	// Compute the cutoff on the leader so every follower applies the same
	// day boundary during Raft replay; time.Now inside the apply path would
	// desync replicas.
	cutoffDay := time.Now().UTC().AddDate(0, 0, -days).Unix() / 86400

	_, err := opDeleteOldDailyUsage.Invoke(ctx, db, &int64Payload{Value: cutoffDay})
	if err != nil {
		recordSpanError(span, err)

		return err
	}

	return nil
}
