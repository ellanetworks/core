// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// SPDX-FileCopyrightText: Ella Networks Inc.

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
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
	incrementDailyUsageStmt       = "INSERT INTO %s (epochDay, imsi, mcc, mnc, bytesUplink, bytesDownlink) VALUES ($DailyUsage.epoch_day, $DailyUsage.imsi, $DailyUsage.mcc, $DailyUsage.mnc, $DailyUsage.bytes_uplink, $DailyUsage.bytes_downlink) ON CONFLICT(epochDay, imsi) DO UPDATE SET bytesUplink = bytesUplink + excluded.bytesUplink, bytesDownlink = bytesDownlink + excluded.bytesDownlink"
	incrementDailyUsagePreV26Stmt = "INSERT INTO %s (epoch_day, imsi, bytes_uplink, bytes_downlink) VALUES ($DailyUsage.epoch_day, $DailyUsage.imsi, $DailyUsage.bytes_uplink, $DailyUsage.bytes_downlink) ON CONFLICT(epoch_day, imsi) DO UPDATE SET bytes_uplink = bytes_uplink + $DailyUsage.bytes_uplink, bytes_downlink = bytes_downlink + $DailyUsage.bytes_downlink"
	deleteOldDailyUsageStmt       = "DELETE FROM %s WHERE epochDay < $cutoffDaysArgs.cutoff_days"
	deleteOldDailyUsagePreV26Stmt = "DELETE FROM %s WHERE epoch_day < $cutoffDaysArgs.cutoff_days"
	deleteAllDailyUsageStmt       = "DELETE FROM %s"
)

const (
	getUsagePerDayStmt = `
SELECT
    epochDay           AS &UsagePerDay.epoch_day,
    SUM(bytesUplink)   AS &UsagePerDay.bytes_uplink,
    SUM(bytesDownlink) AS &UsagePerDay.bytes_downlink
FROM %s
WHERE
    ($UsageFilters.imsi IS NULL OR imsi == $UsageFilters.imsi)
    AND ($UsageFilters.roaming IS NULL OR (mcc IS NOT NULL) == $UsageFilters.roaming)
    AND epochDay >= $UsageFilters.start_date
    AND epochDay <= $UsageFilters.end_date
GROUP BY epochDay
ORDER BY epochDay ASC`

	getUsagePerDayPreV26Stmt = `
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
    u.imsi                  AS &UsagePerSub.imsi,
    MAX(u.mcc)              AS &UsagePerSub.mcc,
    MAX(u.mnc)              AS &UsagePerSub.mnc,
    SUM(u.bytesUplink)      AS &UsagePerSub.bytes_uplink,
    SUM(u.bytesDownlink)    AS &UsagePerSub.bytes_downlink
FROM (
    SELECT imsi, mcc, mnc, bytesUplink, bytesDownlink
    FROM %s
    WHERE epochDay >= $UsageFilters.start_date AND epochDay <= $UsageFilters.end_date
    UNION ALL
    SELECT imsi, NULL, NULL, 0, 0
    FROM %s
) AS u
WHERE
    ($UsageFilters.imsi IS NULL OR u.imsi = $UsageFilters.imsi)
    AND ($UsageFilters.roaming IS NULL OR (u.mcc IS NOT NULL) == $UsageFilters.roaming)
GROUP BY u.imsi
ORDER BY SUM(u.bytesUplink) + SUM(u.bytesDownlink) DESC, u.imsi ASC
LIMIT $UsageFilters.limit`

	getUsagePerSubscriberPreV26Stmt = `
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
	IMSI          string  `db:"imsi"`
	Mcc           *string `db:"mcc"`
	Mnc           *string `db:"mnc"`
	BytesUplink   int64   `db:"bytes_uplink"`
	BytesDownlink int64   `db:"bytes_downlink"`
}

type DailyUsage struct {
	EpochDay      int64   `db:"epoch_day"`
	IMSI          string  `db:"imsi"`
	Mcc           *string `db:"mcc"`
	Mnc           *string `db:"mnc"`
	BytesUplink   int64   `db:"bytes_uplink"`
	BytesDownlink int64   `db:"bytes_downlink"`
}

func (u DailyUsage) Validate() error {
	if u.Mcc == nil && u.Mnc == nil {
		return nil
	}

	if u.Mcc == nil || u.Mnc == nil || !isDigits(*u.Mcc, 3, 3) || !isDigits(*u.Mnc, 2, 3) {
		return fmt.Errorf("usage of IMSI %s has an invalid home PLMN", u.IMSI)
	}

	if !isDigits(u.IMSI, 6, 15) || !strings.HasPrefix(u.IMSI, *u.Mcc+*u.Mnc) {
		return fmt.Errorf("usage IMSI %q is not in home PLMN %s", u.IMSI, RoamingPLMN{Mcc: *u.Mcc, Mnc: *u.Mnc})
	}

	return nil
}

type UsageFilters struct {
	IMSI      *string `db:"imsi"` // exact match
	Roaming   *bool   `db:"roaming"`
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

	for _, u := range usages {
		if err := u.Validate(); err != nil {
			return err
		}
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

func (db *Database) GetUsagePerDay(ctx context.Context, imsi string, days DayRange, roaming *bool) ([]UsagePerDay, error) {
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
		Roaming:   roaming,
		StartDate: days.First,
		EndDate:   days.Last,
		Limit:     NoUsageLimit,
	}

	if imsi != "" {
		dailyUsageFilters.IMSI = &imsi
	}

	var dailyUsage []UsagePerDay

	stmt := db.getUsagePerDayStmt
	if !db.appliedSchemaAtLeast(ctx, roamingSchema) {
		if roaming != nil && *roaming {
			return nil, nil
		}

		stmt = db.getUsagePerDayPreV26Stmt
	}

	err := db.conn().Query(ctx, stmt, dailyUsageFilters).GetAll(&dailyUsage)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}

		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return dailyUsage, nil
}

func (db *Database) GetUsagePerSubscriber(ctx context.Context, imsi string, days DayRange, roaming *bool, limit int64) ([]UsagePerSub, error) {
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
		Roaming:   roaming,
		StartDate: days.First,
		EndDate:   days.Last,
		Limit:     limit,
	}

	if imsi != "" {
		dailyUsageFilters.IMSI = &imsi
	}

	var dailyUsage []UsagePerSub

	stmt := db.getUsagePerSubscriberStmt
	if !db.appliedSchemaAtLeast(ctx, roamingSchema) {
		if roaming != nil && *roaming {
			return nil, nil
		}

		stmt = db.getUsagePerSubscriberPreV26Stmt
	}

	err := db.conn().Query(ctx, stmt, dailyUsageFilters).GetAll(&dailyUsage)
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
