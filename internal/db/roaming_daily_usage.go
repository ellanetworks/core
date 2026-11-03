// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
)

const RoamingDailyUsageTableName = "roaming_daily_usage"

const (
	incrementRoamingDailyUsageStmt = "INSERT INTO %s (epochDay, imsi, mcc, mnc, bytesUplink, bytesDownlink) VALUES ($RoamingDailyUsage.epochDay, $RoamingDailyUsage.imsi, $RoamingDailyUsage.mcc, $RoamingDailyUsage.mnc, $RoamingDailyUsage.bytesUplink, $RoamingDailyUsage.bytesDownlink) ON CONFLICT(epochDay, imsi) DO UPDATE SET bytesUplink = bytesUplink + excluded.bytesUplink, bytesDownlink = bytesDownlink + excluded.bytesDownlink"
	deleteOldRoamingDailyUsageStmt = "DELETE FROM %s WHERE epochDay < $cutoffDaysArgs.cutoff_days"
	listRoamingUsageStmt           = `
SELECT
    epochDay           AS &RoamingUsage.epochDay,
    mcc                AS &RoamingUsage.mcc,
    mnc                AS &RoamingUsage.mnc,
    SUM(bytesUplink)   AS &RoamingUsage.bytesUplink,
    SUM(bytesDownlink) AS &RoamingUsage.bytesDownlink
FROM %s
WHERE
    ($roamingUsageFilters.imsi IS NULL OR imsi == $roamingUsageFilters.imsi)
    AND epochDay >= $roamingUsageFilters.firstDay
    AND epochDay <= $roamingUsageFilters.lastDay
GROUP BY epochDay, mcc, mnc
ORDER BY epochDay ASC, mcc ASC, mnc ASC`
)

type RoamingDailyUsage struct {
	EpochDay      int64  `db:"epochDay"`
	IMSI          string `db:"imsi"`
	Mcc           string `db:"mcc"`
	Mnc           string `db:"mnc"`
	BytesUplink   int64  `db:"bytesUplink"`
	BytesDownlink int64  `db:"bytesDownlink"`
}

type RoamingDailyUsageBatch struct {
	Rows []RoamingDailyUsage
}

func (u RoamingDailyUsage) Validate() error {
	plmn := RoamingPLMN{Mcc: u.Mcc, Mnc: u.Mnc}

	if !isDigits(u.Mcc, 3, 3) || !isDigits(u.Mnc, 2, 3) {
		return fmt.Errorf("roaming usage of IMSI %s has an invalid PLMN %s", u.IMSI, plmn)
	}

	if !isDigits(u.IMSI, 6, 15) || !strings.HasPrefix(u.IMSI, u.Mcc+u.Mnc) {
		return fmt.Errorf("roaming usage IMSI %q is not in PLMN %s", u.IMSI, plmn)
	}

	if u.BytesUplink < 0 || u.BytesDownlink < 0 {
		return fmt.Errorf("roaming usage of IMSI %s has negative bytes", u.IMSI)
	}

	return nil
}

type RoamingUsage struct {
	EpochDay      int64  `db:"epochDay"`
	Mcc           string `db:"mcc"`
	Mnc           string `db:"mnc"`
	BytesUplink   int64  `db:"bytesUplink"`
	BytesDownlink int64  `db:"bytesDownlink"`
}

type roamingUsageFilters struct {
	IMSI     *string `db:"imsi"`
	FirstDay int64   `db:"firstDay"`
	LastDay  int64   `db:"lastDay"`
}

func (db *Database) IncrementRoamingDailyUsageBatch(ctx context.Context, usages []RoamingDailyUsage) error {
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

	querySummary := fmt.Sprintf("%s %s", operation, RoamingDailyUsageTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName(operation),
			semconv.DBCollectionName(RoamingDailyUsageTableName),
		),
		trace.WithAttributes(batchAttrs...),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(RoamingDailyUsageTableName, "batch_insert"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(RoamingDailyUsageTableName, "batch_insert").Inc()

	if _, err := opIncrementRoamingDailyUsageBatch.Invoke(ctx, db, &RoamingDailyUsageBatch{Rows: usages}); err != nil {
		recordSpanError(span, err)

		return err
	}

	return nil
}

func (db *Database) ListRoamingUsage(ctx context.Context, imsi string, days DayRange) ([]RoamingUsage, error) {
	querySummary := fmt.Sprintf("%s %s", "SELECT", RoamingDailyUsageTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			semconv.DBCollectionName(RoamingDailyUsageTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(RoamingDailyUsageTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(RoamingDailyUsageTableName, "select").Inc()

	if !db.appliedSchemaAtLeast(ctx, roamingSchema) {
		return nil, nil
	}

	filters := roamingUsageFilters{FirstDay: days.First, LastDay: days.Last}

	if imsi != "" {
		filters.IMSI = &imsi
	}

	var usage []RoamingUsage

	if err := db.conn().Query(ctx, db.listRoamingUsageStmt, filters).GetAll(&usage); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}

		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return usage, nil
}

func (db *Database) applyIncrementRoamingDailyUsageBatch(ctx context.Context, batch *RoamingDailyUsageBatch) (any, error) {
	for i := range batch.Rows {
		if err := db.runner(ctx).Query(ctx, db.incrementRoamingDailyUsageStmt, &batch.Rows[i]).Run(); err != nil {
			return nil, fmt.Errorf("query failed: %w", err)
		}
	}

	return nil, nil
}

func (db *Database) applyDeleteOldRoamingDailyUsage(ctx context.Context, cutoffDay int64) error {
	if !db.appliedSchemaAtLeast(ctx, roamingSchema) {
		return nil
	}

	if err := db.runner(ctx).Query(ctx, db.deleteOldRoamingDailyUsageStmt, cutoffDaysArgs{CutoffDays: cutoffDay}).Run(); err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	return nil
}
