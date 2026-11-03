// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	listVisitorRegistrationsStmt = "SELECT &UERegistration.* FROM %s WHERE purged==0 AND imsi IN (SELECT imsi FROM %s WHERE roamingPartnerID IS NOT NULL) ORDER BY imsi, type"
	listRoamingUsageOfDayStmt    = "SELECT &RoamingDailyUsage.* FROM %s WHERE epochDay==$roamingUsageFilters.firstDay ORDER BY imsi"
)

func (db *Database) ListVisitorRegistrations(ctx context.Context) ([]UERegistration, error) {
	ctx, span := startVisitorsSpan(ctx, "SELECT", "visitor registrations")
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(UERegistrationsTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(UERegistrationsTableName, "select").Inc()

	if !db.appliedSchemaAtLeast(ctx, roamingSchema) {
		return nil, nil
	}

	var regs []UERegistration

	if err := db.conn().Query(ctx, db.listVisitorRegistrationsStmt).GetAll(&regs); err != nil && !errors.Is(err, sql.ErrNoRows) {
		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return regs, nil
}

func (db *Database) ListRoamingUsageOfDay(ctx context.Context, epochDay int64) ([]RoamingDailyUsage, error) {
	ctx, span := startVisitorsSpan(ctx, "SELECT", "roaming usage of a day")
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(RoamingDailyUsageTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(RoamingDailyUsageTableName, "select").Inc()

	if !db.appliedSchemaAtLeast(ctx, roamingSchema) {
		return nil, nil
	}

	var rows []RoamingDailyUsage

	if err := db.conn().Query(ctx, db.listRoamingUsageOfDayStmt, roamingUsageFilters{FirstDay: epochDay}).GetAll(&rows); err != nil && !errors.Is(err, sql.ErrNoRows) {
		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return rows, nil
}
