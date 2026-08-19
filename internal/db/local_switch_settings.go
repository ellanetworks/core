// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
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
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "SELECT", LocalSwitchSettingsTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			attribute.String("db.collection", LocalSwitchSettingsTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(LocalSwitchSettingsTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(LocalSwitchSettingsTableName, "select").Inc()

	var localSwitchSettings LocalSwitchSettings

	err := db.conn().Query(ctx, db.getLocalSwitchSettingsStmt).Get(&localSwitchSettings)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return false, fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return localSwitchSettings.Enabled, nil
}

func (db *Database) UpdateLocalSwitchSettings(ctx context.Context, enabled bool) error {
	_, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "UPSERT", LocalSwitchSettingsTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("UPSERT"),
			attribute.String("db.collection", LocalSwitchSettingsTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(LocalSwitchSettingsTableName, "update"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(LocalSwitchSettingsTableName, "update").Inc()

	_, err := db.applyUpdateLocalSwitchSettings(ctx, &boolPayload{Value: enabled})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return err
	}

	db.publishOpTopics([]Topic{TopicLocalSwitchSettings}, 0)
	span.SetStatus(codes.Ok, "")

	return nil
}
