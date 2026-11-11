// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// SPDX-FileCopyrightText: Ella Networks Inc.

package db

import (
	"context"
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
	incrementDailyUsageSQL       = "INSERT INTO " + DailyUsageTableName + " (epochDay, imsi, mcc, mnc, bytesUplink, bytesDownlink) VALUES (?, ?, ?, ?, ?, ?) ON CONFLICT(epochDay, imsi) DO UPDATE SET bytesUplink = bytesUplink + excluded.bytesUplink, bytesDownlink = bytesDownlink + excluded.bytesDownlink"
	incrementDailyUsagePreV27SQL = "INSERT INTO " + DailyUsageTableName + " (epoch_day, imsi, bytes_uplink, bytes_downlink) VALUES (?, ?, ?, ?) ON CONFLICT(epoch_day, imsi) DO UPDATE SET bytes_uplink = bytes_uplink + ?, bytes_downlink = bytes_downlink + ?"
	deleteOldDailyUsageSQL       = "DELETE FROM " + DailyUsageTableName + " WHERE epochDay < ?"
	deleteOldDailyUsagePreV27SQL = "DELETE FROM " + DailyUsageTableName + " WHERE epoch_day < ?"
	deleteAllDailyUsageSQL       = "DELETE FROM " + DailyUsageTableName
)

const (
	getUsagePerDaySQL = `
SELECT
    epochDay,
    SUM(bytesUplink),
    SUM(bytesDownlink)
FROM ` + DailyUsageTableName + `
WHERE
    (?1 IS NULL OR imsi == ?1)
    AND (?2 IS NULL OR (mcc IS NOT NULL) == ?2)
    AND epochDay >= ?3
    AND epochDay <= ?4
GROUP BY epochDay
ORDER BY epochDay ASC`

	getUsagePerDayPreV27SQL = `
SELECT
    epoch_day,
    SUM(bytes_uplink),
    SUM(bytes_downlink)
FROM ` + DailyUsageTableName + `
WHERE
    (?1 IS NULL OR imsi == ?1)
    AND epoch_day >= ?3
    AND epoch_day <= ?4
GROUP BY epoch_day
ORDER BY epoch_day ASC`

	usageDaysInRange = `(
    WITH RECURSIVE days(d) AS (
        SELECT ?3 WHERE ?3 <= ?4
        UNION ALL
        SELECT d + 1 FROM days WHERE d < ?4
    )
    SELECT d FROM days)`

	getUsagePerDayByIMSISQL = `
SELECT
    epochDay,
    SUM(bytesUplink),
    SUM(bytesDownlink)
FROM ` + DailyUsageTableName + `
WHERE
    imsi == ?1
    AND (?2 IS NULL OR (mcc IS NOT NULL) == ?2)
    AND epochDay IN ` + usageDaysInRange + `
GROUP BY epochDay
ORDER BY epochDay ASC`

	getUsagePerDayByIMSIPreV27SQL = `
SELECT
    epoch_day,
    SUM(bytes_uplink),
    SUM(bytes_downlink)
FROM ` + DailyUsageTableName + `
WHERE
    imsi == ?1
    AND epoch_day IN ` + usageDaysInRange + `
GROUP BY epoch_day
ORDER BY epoch_day ASC`
)

const (
	getUsagePerSubscriberSQL = `
SELECT
    u.imsi,
    MAX(u.mcc),
    MAX(u.mnc),
    SUM(u.bytesUplink),
    SUM(u.bytesDownlink)
FROM (
    SELECT imsi, mcc, mnc, bytesUplink, bytesDownlink
    FROM ` + DailyUsageTableName + `
    WHERE epochDay >= ?3 AND epochDay <= ?4
    UNION ALL
    SELECT imsi, NULL, NULL, 0, 0
    FROM ` + SubscribersTableName + `
) AS u
WHERE
    (?1 IS NULL OR u.imsi = ?1)
    AND (?2 IS NULL OR (u.mcc IS NOT NULL) == ?2)
GROUP BY u.imsi
ORDER BY SUM(u.bytesUplink) + SUM(u.bytesDownlink) DESC, u.imsi ASC
LIMIT ?5`

	getUsagePerSubscriberPreV27SQL = `
SELECT
    s.imsi,
    COALESCE(u.bytes_uplink, 0),
    COALESCE(u.bytes_downlink, 0)
FROM ` + SubscribersTableName + ` AS s
LEFT JOIN (
    SELECT imsi, SUM(bytes_uplink) AS bytes_uplink, SUM(bytes_downlink) AS bytes_downlink
    FROM ` + DailyUsageTableName + `
    WHERE epoch_day >= ?3 AND epoch_day <= ?4
    GROUP BY imsi
) AS u ON u.imsi = s.imsi
ORDER BY COALESCE(u.bytes_uplink, 0) + COALESCE(u.bytes_downlink, 0) DESC, s.imsi ASC
LIMIT ?5`

	getUsagePerSubscriberByIMSISQL = `
SELECT
    u.imsi,
    MAX(u.mcc),
    MAX(u.mnc),
    SUM(u.bytesUplink),
    SUM(u.bytesDownlink)
FROM (
    SELECT imsi, mcc, mnc, bytesUplink, bytesDownlink
    FROM ` + DailyUsageTableName + `
    WHERE imsi = ?1 AND epochDay IN ` + usageDaysInRange + `
    UNION ALL
    SELECT imsi, NULL, NULL, 0, 0
    FROM ` + SubscribersTableName + `
    WHERE imsi = ?1
) AS u
WHERE ?2 IS NULL OR (u.mcc IS NOT NULL) == ?2
GROUP BY u.imsi
LIMIT ?5`

	getUsagePerSubscriberByIMSIPreV27SQL = `
SELECT
    s.imsi,
    COALESCE(SUM(u.bytes_uplink), 0),
    COALESCE(SUM(u.bytes_downlink), 0)
FROM ` + SubscribersTableName + ` AS s
LEFT JOIN ` + DailyUsageTableName + ` AS u
    ON u.imsi = s.imsi
    AND u.epoch_day IN ` + usageDaysInRange + `
WHERE s.imsi = ?1
GROUP BY s.imsi
LIMIT ?5`
)

type UsagePerDay struct {
	EpochDay      int64
	BytesUplink   int64
	BytesDownlink int64
}

type UsagePerSub struct {
	IMSI          string
	Mcc           *string
	Mnc           *string
	BytesUplink   int64
	BytesDownlink int64
}

type DailyUsage struct {
	EpochDay      int64
	IMSI          string
	Mcc           *string
	Mnc           *string
	BytesUplink   int64
	BytesDownlink int64
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
	IMSI      *string // exact match
	Roaming   *bool
	StartDate int64
	EndDate   int64
	Limit     int64 // SQLite treats a negative limit as unbounded
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
	ctx, span, timer := startDBQuery(ctx, DailyUsageTableName, "INSERT", "insert")
	defer span.End()
	defer timer.ObserveDuration()

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
	ctx, span, timer := startDBQuery(ctx, DailyUsageTableName, "SELECT", "select")
	defer span.End()
	defer timer.ObserveDuration()

	dailyUsageFilters := UsageFilters{
		Roaming:   roaming,
		StartDate: days.First,
		EndDate:   days.Last,
		Limit:     NoUsageLimit,
	}

	if imsi != "" {
		dailyUsageFilters.IMSI = &imsi
	}

	query := getUsagePerDaySQL
	if imsi != "" {
		query = getUsagePerDayByIMSISQL
	}

	if !db.appliedSchemaAtLeast(ctx, roamingSchema) {
		if roaming != nil && *roaming {
			return nil, nil
		}

		query = getUsagePerDayPreV27SQL
		if imsi != "" {
			query = getUsagePerDayByIMSIPreV27SQL
		}
	}

	dailyUsage, err := queryAll(ctx, db.reader(), query, scanUsagePerDay, dailyUsageFilters.IMSI, dailyUsageFilters.Roaming, dailyUsageFilters.StartDate, dailyUsageFilters.EndDate)
	if err != nil {
		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return dailyUsage, nil
}

func (db *Database) GetUsagePerSubscriber(ctx context.Context, imsi string, days DayRange, roaming *bool, limit int64) ([]UsagePerSub, error) {
	ctx, span, timer := startDBQuery(ctx, DailyUsageTableName, "SELECT", "select")
	defer span.End()
	defer timer.ObserveDuration()

	dailyUsageFilters := UsageFilters{
		Roaming:   roaming,
		StartDate: days.First,
		EndDate:   days.Last,
		Limit:     limit,
	}

	if imsi != "" {
		dailyUsageFilters.IMSI = &imsi
	}

	query, scan := getUsagePerSubscriberSQL, scanUsagePerSub
	if imsi != "" {
		query = getUsagePerSubscriberByIMSISQL
	}

	if !db.appliedSchemaAtLeast(ctx, roamingSchema) {
		if roaming != nil && *roaming {
			return nil, nil
		}

		query, scan = getUsagePerSubscriberPreV27SQL, scanUsagePerSubPreV27
		if imsi != "" {
			query = getUsagePerSubscriberByIMSIPreV27SQL
		}
	}

	dailyUsage, err := queryAll(ctx, db.reader(), query, scan, dailyUsageFilters.IMSI, dailyUsageFilters.Roaming, dailyUsageFilters.StartDate, dailyUsageFilters.EndDate, dailyUsageFilters.Limit)
	if err != nil {
		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return dailyUsage, nil
}

func (db *Database) ClearDailyUsage(ctx context.Context) error {
	ctx, span, timer := startDBQuery(ctx, DailyUsageTableName, "DELETE", "delete")
	defer span.End()
	defer timer.ObserveDuration()

	_, err := opClearDailyUsage.Invoke(ctx, db, &emptyPayload{})
	if err != nil {
		recordSpanError(span, err)

		return err
	}

	return nil
}

func (db *Database) DeleteOldDailyUsage(ctx context.Context, days int) error {
	ctx, span, timer := startDBQueryNamed(ctx, "DELETE "+DailyUsageTableName+" (retention)", DailyUsageTableName, "DELETE", "delete", attribute.Int("retention.days", days))
	defer span.End()
	defer timer.ObserveDuration()

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

func scanUsagePerDay(row scanner) (UsagePerDay, error) {
	var v UsagePerDay

	if err := row.Scan(&v.EpochDay, &v.BytesUplink, &v.BytesDownlink); err != nil {
		return UsagePerDay{}, err
	}

	return v, nil
}

func scanUsagePerSub(row scanner) (UsagePerSub, error) {
	var v UsagePerSub

	if err := row.Scan(&v.IMSI, &v.Mcc, &v.Mnc, &v.BytesUplink, &v.BytesDownlink); err != nil {
		return UsagePerSub{}, err
	}

	return v, nil
}

func scanUsagePerSubPreV27(row scanner) (UsagePerSub, error) {
	var v UsagePerSub

	if err := row.Scan(&v.IMSI, &v.BytesUplink, &v.BytesDownlink); err != nil {
		return UsagePerSub{}, err
	}

	return v, nil
}
