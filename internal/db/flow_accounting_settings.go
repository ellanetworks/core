package db

import (
	"context"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
)

const (
	FlowAccountingDefaultEnabled = true
)

const FlowAccountingSettingsTableName = "flow_accounting_settings"

const QueryCreateFlowAccountingSettingsTable = `
CREATE TABLE IF NOT EXISTS %s (
  singleton BOOLEAN PRIMARY KEY DEFAULT TRUE,
  enabled   BOOLEAN NOT NULL DEFAULT TRUE,
  CHECK (singleton)
);
`

const insertDefaultFlowAccountingSettingsStmt = `INSERT OR IGNORE INTO %s (singleton, enabled) VALUES (TRUE, $FlowAccountingSettings.enabled);`

const upsertFlowAccountingSettingsStmt = `
INSERT INTO %s (singleton, enabled) VALUES (TRUE, $FlowAccountingSettings.enabled)
ON CONFLICT(singleton) DO UPDATE SET enabled=$FlowAccountingSettings.enabled;
`

const getFlowAccountingSettingsStmt = `SELECT &FlowAccountingSettings.* FROM %s WHERE singleton=TRUE;`

type FlowAccountingSettings struct {
	Enabled bool `db:"enabled"`
}

func (db *Database) InitializeFlowAccountingSettings(ctx context.Context) error {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "INSERT", FlowAccountingSettingsTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("INSERT"),
			attribute.String("db.collection", FlowAccountingSettingsTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(FlowAccountingSettingsTableName, "insert"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(FlowAccountingSettingsTableName, "insert").Inc()

	flowAccountingSettings := FlowAccountingSettings{Enabled: FlowAccountingDefaultEnabled}

	err := db.conn.Query(ctx, db.insertDefaultFlowAccountingSettingsStmt, flowAccountingSettings).Run()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

func (db *Database) IsFlowAccountingEnabled(ctx context.Context) (bool, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "SELECT", FlowAccountingSettingsTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("SELECT"),
			attribute.String("db.collection", FlowAccountingSettingsTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(FlowAccountingSettingsTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(FlowAccountingSettingsTableName, "select").Inc()

	var flowAccountingSettings FlowAccountingSettings

	err := db.conn.Query(ctx, db.getFlowAccountingSettingsStmt).Get(&flowAccountingSettings)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return false, fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return flowAccountingSettings.Enabled, nil
}

func (db *Database) UpdateFlowAccountingSettings(ctx context.Context, enabled bool) error {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "UPSERT", FlowAccountingSettingsTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("UPSERT"),
			attribute.String("db.collection", FlowAccountingSettingsTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(FlowAccountingSettingsTableName, "update"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(FlowAccountingSettingsTableName, "update").Inc()

	arg := FlowAccountingSettings{Enabled: enabled}

	err := db.conn.Query(ctx, db.upsertFlowAccountingSettingsStmt, arg).Run()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return nil
}
