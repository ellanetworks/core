// Copyright 2024 Ella Networks

package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/canonical/sqlair"
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
	incrementDailyUsageStmt  = "INSERT INTO %s (epoch_day, imsi, bytes_uplink, bytes_downlink) VALUES ($DailyUsage.epoch_day, $DailyUsage.imsi, $DailyUsage.bytes_uplink, $DailyUsage.bytes_downlink) ON CONFLICT(epoch_day, imsi) DO UPDATE SET bytes_uplink = bytes_uplink + $DailyUsage.bytes_uplink, bytes_downlink = bytes_downlink + $DailyUsage.bytes_downlink"
	getDailyUsageStmt        = "SELECT &DailyUsage.* FROM %s WHERE epoch_day==$DailyUsage.epoch_day AND imsi==$DailyUsage.imsi"
	getTotalUsageForIMSIStmt = "SELECT SUM(bytes_uplink) AS &TotalUsage.bytes_uplink, SUM(bytes_downlink) AS &TotalUsage.bytes_downlink FROM %s WHERE imsi==$TotalUsage.imsi"
	getUsageForPeriodStmt    = "SELECT SUM(bytes_uplink) AS &TotalUsage.bytes_uplink, SUM(bytes_downlink) AS &TotalUsage.bytes_downlink FROM %s WHERE imsi==$TotalUsage.imsi AND epoch_day >= $TotalUsage.epoch_day_from AND epoch_day <= $TotalUsage.epoch_day_to"
	deleteOldDailyUsageStmt  = "DELETE FROM %s WHERE epoch_day < $cutoffDaysArgs.cutoff_days"
	deleteAllDailyUsageStmt  = "DELETE FROM %s"
)

const (
	getDailyUsageForPeriodStmt = `
SELECT
    epoch_day AS &TotalDailyUsage.epoch_day,
    SUM(bytes_uplink)   AS &TotalDailyUsage.bytes_uplink,
    SUM(bytes_downlink) AS &TotalDailyUsage.bytes_downlink
FROM %s
WHERE
    ($DailyUsageFilters.imsi IS NULL OR imsi == $DailyUsageFilters.imsi)
    AND epoch_day >= $DailyUsageFilters.start_date
    AND epoch_day <= $DailyUsageFilters.end_date
GROUP BY epoch_day
ORDER BY epoch_day ASC`
)

type TotalDailyUsage struct {
	EpochDay      int64 `db:"epoch_day"`
	BytesUplink   int64 `db:"bytes_uplink"`
	BytesDownlink int64 `db:"bytes_downlink"`
}

type DailyUsage struct {
	EpochDay      int64  `db:"epoch_day"`
	IMSI          string `db:"imsi"`
	BytesUplink   int64  `db:"bytes_uplink"`
	BytesDownlink int64  `db:"bytes_downlink"`
}

type DailyUsageFilters struct {
	IMSI      *string `db:"imsi"` // exact match
	StartDate int64   `db:"start_date"`
	EndDate   int64   `db:"end_date"`
}

type TotalUsage struct {
	IMSI          string `db:"imsi"`
	EpochDayFrom  int64  `db:"epoch_day_from"`
	EpochDayTo    int64  `db:"epoch_day_to"`
	BytesUplink   int64  `db:"bytes_uplink"`
	BytesDownlink int64  `db:"bytes_downlink"`
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

func (d *TotalDailyUsage) GetDay() time.Time {
	return time.Unix(d.EpochDay*86400, 0).UTC()
}

func (db *Database) IncrementDailyUsage(ctx context.Context, usage DailyUsage) error {
	operation := "INSERT"
	target := DailyUsageTableName
	spanName := fmt.Sprintf("%s %s", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	stmt := fmt.Sprintf(incrementDailyUsageStmt, db.dailyUsageTable)
	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(stmt),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
	)

	q, err := sqlair.Prepare(stmt, DailyUsage{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "prepare failed")
		return err
	}
	if err := db.conn.Query(ctx, q, usage).Run(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")
		return err
	}

	span.SetStatus(codes.Ok, "")
	return nil
}

func (db *Database) GetDailyUsage(ctx context.Context, date time.Time, imsi string) (*DailyUsage, error) {
	operation := "SELECT"
	target := DailyUsageTableName
	spanName := fmt.Sprintf("%s %s", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	stmt := fmt.Sprintf(getDailyUsageStmt, db.dailyUsageTable)
	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(stmt),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
	)

	row := DailyUsage{EpochDay: DaysSinceEpoch(date), IMSI: imsi}
	q, err := sqlair.Prepare(stmt, DailyUsage{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "prepare failed")
		return nil, err
	}

	if err := db.conn.Query(ctx, q, row).Get(&row); err != nil {
		if err == sqlair.ErrNoRows {
			return nil, nil
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")
		return nil, err
	}

	span.SetStatus(codes.Ok, "")
	return &row, nil
}

func (db *Database) GetTotalUsageForIMSI(ctx context.Context, imsi string) (*TotalUsage, error) {
	operation := "SELECT"
	target := DailyUsageTableName
	spanName := fmt.Sprintf("%s %s", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	stmt := fmt.Sprintf(getTotalUsageForIMSIStmt, db.dailyUsageTable)
	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(stmt),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
	)

	row := TotalUsage{IMSI: imsi}
	q, err := sqlair.Prepare(stmt, TotalUsage{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "prepare failed")
		return nil, err
	}

	if err := db.conn.Query(ctx, q, row).Get(&row); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")
		return nil, err
	}

	span.SetStatus(codes.Ok, "")

	return &row, nil
}

func (db *Database) GetDailyUsageForPeriod(ctx context.Context, imsi string, startDate time.Time, endDate time.Time) ([]TotalDailyUsage, error) {
	dailyUsageFilters := DailyUsageFilters{
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

	stmtStr := fmt.Sprintf(getDailyUsageForPeriodStmt, db.dailyUsageTable)

	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(stmtStr),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
	)

	var dailyUsage []TotalDailyUsage

	q, err := sqlair.Prepare(stmtStr, DailyUsageFilters{}, TotalDailyUsage{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "prepare failed")
		return nil, fmt.Errorf("couldn't prepare statement: %w", err)
	}

	if err := db.conn.Query(ctx, q, dailyUsageFilters).GetAll(&dailyUsage); err != nil {
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

func (db *Database) GetUsageForPeriod(ctx context.Context, imsi string, dateFrom time.Time, dateTo time.Time) (*TotalUsage, error) {
	operation := "SELECT"
	target := DailyUsageTableName
	spanName := fmt.Sprintf("%s %s", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	stmt := fmt.Sprintf(getUsageForPeriodStmt, db.dailyUsageTable)

	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(stmt),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
	)

	row := TotalUsage{IMSI: imsi, EpochDayFrom: DaysSinceEpoch(dateFrom), EpochDayTo: DaysSinceEpoch(dateTo)}

	q, err := sqlair.Prepare(stmt, TotalUsage{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "prepare failed")
		return nil, err
	}

	if err := db.conn.Query(ctx, q, row).Get(&row); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")
		return nil, err
	}

	span.SetStatus(codes.Ok, "")

	return &row, nil
}

func (db *Database) ClearDailyUsage(ctx context.Context) error {
	operation := "DELETE"
	target := DailyUsageTableName
	spanName := fmt.Sprintf("%s %s", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	stmt := fmt.Sprintf(deleteAllDailyUsageStmt, db.dailyUsageTable)
	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(stmt),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
	)

	q, err := sqlair.Prepare(stmt)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "prepare failed")
		return err
	}

	if err := db.conn.Query(ctx, q).Run(); err != nil {
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

	stmtStr := fmt.Sprintf(deleteOldDailyUsageStmt, db.dailyUsageTable)
	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(stmtStr),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
		attribute.Int("retention.days", days),
		attribute.Int64("retention.cutoff_epoch_day", cutoffDay),
	)

	q, err := sqlair.Prepare(stmtStr, cutoffDaysArgs{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "prepare failed")
		return err
	}

	if err := db.conn.Query(ctx, q, cutoffDaysArgs{CutoffDays: cutoffDay}).Run(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")
		return err
	}

	span.SetStatus(codes.Ok, "")
	return nil
}
