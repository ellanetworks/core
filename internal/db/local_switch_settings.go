// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
)

const (
	LocalSwitchDefaultEnabled = false
)

const LocalSwitchSettingsTableName = "local_switch_settings"

const upsertLocalSwitchSettingsStmt = `
INSERT INTO %s (singleton, enabled) VALUES (TRUE, $LocalSwitchSettings.enabled)
ON CONFLICT(singleton) DO UPDATE SET enabled=$LocalSwitchSettings.enabled;
`

const getLocalSwitchSettingsStmt = `SELECT &LocalSwitchSettings.* FROM %s WHERE singleton=TRUE;`

type LocalSwitchSettings struct {
	Enabled bool `db:"enabled"`
}

func (db *Database) InitializeLocalSwitchSettings(ctx context.Context) error {
	_, err := db.IsLocalSwitchEnabled(ctx)
	if err == nil {
		return nil
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("failed to check local switch settings: %w", err)
	}

	return db.UpdateLocalSwitchSettings(ctx, LocalSwitchDefaultEnabled)
}

func (db *Database) IsLocalSwitchEnabled(ctx context.Context) (bool, error) {
	querySummary := fmt.Sprintf("%s %s", "SELECT", LocalSwitchSettingsTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			semconv.DBCollectionName(LocalSwitchSettingsTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(LocalSwitchSettingsTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(LocalSwitchSettingsTableName, "select").Inc()

	var localSwitchSettings LocalSwitchSettings

	err := db.conn().Query(ctx, db.getLocalSwitchSettingsStmt).Get(&localSwitchSettings)
	if err != nil {
		recordSpanError(span, err)

		return false, fmt.Errorf("query failed: %w", err)
	}

	return localSwitchSettings.Enabled, nil
}

func (db *Database) UpdateLocalSwitchSettings(ctx context.Context, enabled bool) error {
	querySummary := fmt.Sprintf("%s %s", "UPSERT", LocalSwitchSettingsTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("UPSERT"),
			semconv.DBCollectionName(LocalSwitchSettingsTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(LocalSwitchSettingsTableName, "update"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(LocalSwitchSettingsTableName, "update").Inc()

	_, err := db.applyUpdateLocalSwitchSettings(ctx, &boolPayload{Value: enabled})
	if err != nil {
		recordSpanError(span, err)

		return err
	}

	db.publishOpTopics([]Topic{TopicLocalSwitchSettings})

	return nil
}
