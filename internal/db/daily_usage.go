// Copyright 2024 Ella Networks

package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
)

const DailyUsageTableName = "daily_usage"

const QueryCreateDailyUsageTable = `
	CREATE TABLE IF NOT EXISTS %s (
		epoch_day INTEGER NOT NULL,

		imsi TEXT NOT NULL,
		bytes_uplink   INTEGER NOT NULL DEFAULT 0 CHECK (bytes_uplink   >= 0),
    bytes_downlink INTEGER NOT NULL DEFAULT 0 CHECK (bytes_downlink >= 0),

		PRIMARY KEY (epoch_day, imsi),

		FOREIGN KEY (imsi) REFERENCES subscribers(imsi)
)`

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
    imsi AS &UsagePerSub.imsi,
    COALESCE(SUM(bytes_uplink), 0)   AS &UsagePerSub.bytes_uplink,
    COALESCE(SUM(bytes_downlink), 0) AS &UsagePerSub.bytes_downlink
FROM %s
WHERE
    epoch_day >= $UsageFilters.start_date
		AND epoch_day <= $UsageFilters.end_date
		AND ($UsageFilters.imsi IS NULL OR imsi = $UsageFilters.imsi)
GROUP BY imsi
ORDER BY COALESCE(SUM(bytes_uplink), 0) + COALESCE(SUM(bytes_downlink), 0) DESC`
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
}

func DaysSinceEpoch(t time.Time) int64 {
	t = t.UTC()
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC).Unix() / 86400
}

func (d *DailyUsage) SetDay(t time.Time) {
	d.EpochDay = DaysSinceEpoch(t)
}

func (d *DailyUsage) GetDay() time.Time {
	return time.Unix(d.EpochDay*86400, 0).UTC()
}

func (d *UsagePerDay) GetDay() time.Time {
	return time.Unix(d.EpochDay*86400, 0).UTC()
}

func (db *Database) IncrementDailyUsage(ctx context.Context, usage DailyUsage) error {
	operation := "INSERT"
	target := DailyUsageTableName
	spanName := fmt.Sprintf("%s %s", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
	)

	if err := db.conn.Query(ctx, db.incrementDailyUsageStmt, usage).Run(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")
		return err
	}

	span.SetStatus(codes.Ok, "")
	return nil
}

func (db *Database) GetUsagePerDay(ctx context.Context, imsi string, startDate time.Time, endDate time.Time) ([]UsagePerDay, error) {
	dailyUsageFilters := UsageFilters{
		StartDate: DaysSinceEpoch(startDate),
		EndDate:   DaysSinceEpoch(endDate),
	}
	if imsi != "" {
		dailyUsageFilters.IMSI = &imsi
	}

	operation := "SELECT"
	target := DailyUsageTableName
	spanName := fmt.Sprintf("%s %s", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
	)

	var dailyUsage []UsagePerDay

	if err := db.conn.Query(ctx, db.getUsagePerDayStmt, dailyUsageFilters).GetAll(&dailyUsage); err != nil {
		if err == sql.ErrNoRows {
			span.SetStatus(codes.Ok, "no rows")
			return nil, nil
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")
		return nil, fmt.Errorf("couldn't query: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return dailyUsage, nil
}

func (db *Database) GetUsagePerSubscriber(ctx context.Context, imsi string, startDate time.Time, endDate time.Time) ([]UsagePerSub, error) {
	dailyUsageFilters := UsageFilters{
		StartDate: DaysSinceEpoch(startDate),
		EndDate:   DaysSinceEpoch(endDate),
	}
	if imsi != "" {
		dailyUsageFilters.IMSI = &imsi
	}

	operation := "SELECT"
	target := DailyUsageTableName
	spanName := fmt.Sprintf("%s %s", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
	)

	var dailyUsage []UsagePerSub

	if err := db.conn.Query(ctx, db.getUsagePerSubscriberStmt, dailyUsageFilters).GetAll(&dailyUsage); err != nil {
		if err == sql.ErrNoRows {
			span.SetStatus(codes.Ok, "no rows")
			return nil, nil
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")
		return nil, fmt.Errorf("couldn't query: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return dailyUsage, nil
}

func (db *Database) ClearDailyUsage(ctx context.Context) error {
	operation := "DELETE"
	target := DailyUsageTableName
	spanName := fmt.Sprintf("%s %s", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
	)

	if err := db.conn.Query(ctx, db.deleteAllDailyUsageStmt).Run(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")
		return err
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

func (db *Database) DeleteOldDailyUsage(ctx context.Context, days int) error {
	operation := "DELETE"
	target := DailyUsageTableName
	spanName := fmt.Sprintf("%s %s (older than %d days)", operation, target, days)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	now := time.Now().UTC()
	cutoffDay := DaysSinceEpoch(now.AddDate(0, 0, -days))

	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
		attribute.Int("retention.days", days),
		attribute.Int64("retention.cutoff_epoch_day", cutoffDay),
	)

	if err := db.conn.Query(ctx, db.deleteOldDailyUsageStmt, cutoffDaysArgs{CutoffDays: cutoffDay}).Run(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")
		return err
	}

	span.SetStatus(codes.Ok, "")
	return nil
}
