package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
)

const RetentionPolicyTableName = "retention_policies"

const QueryCreateRetentionPolicyTable = `
	CREATE TABLE IF NOT EXISTS %s (
		id              INTEGER PRIMARY KEY AUTOINCREMENT,
		category        TEXT NOT NULL UNIQUE,
		retention_days  INTEGER NOT NULL CHECK (retention_days >= 1)
);`

const (
	selectRetentionPolicyStmt = "SELECT &RetentionPolicy.* FROM %s WHERE category = $RetentionPolicy.category"
	upsertRetentionPolicyStmt = `
INSERT INTO %s (category, retention_days)
VALUES ($RetentionPolicy.category, $RetentionPolicy.retention_days)
ON CONFLICT(category) DO UPDATE SET retention_days = excluded.retention_days
`
)

type RetentionCategory string

const (
	CategoryAuditLogs       RetentionCategory = "audit"
	CategoryRadioLogs       RetentionCategory = "radio"
	CategorySubscriberUsage RetentionCategory = "usage"
)

type RetentionPolicy struct {
	ID       int               `db:"id"`
	Category RetentionCategory `db:"category"`
	Days     int               `db:"retention_days"`
}

func (db *Database) GetRetentionPolicy(ctx context.Context, category RetentionCategory) (int, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "SELECT", RetentionPolicyTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("SELECT"),
			attribute.String("db.collection", RetentionPolicyTableName),
			attribute.String("policy.category", string(category)),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(RetentionPolicyTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(RetentionPolicyTableName, "select").Inc()

	arg := RetentionPolicy{Category: category}

	var row RetentionPolicy

	err := db.conn.Query(ctx, db.selectRetentionPolicyStmt, arg).Get(&row)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return 0, fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return row.Days, nil
}

// Ensure that we have a row for the Audit Log retention policy.
func (db *Database) IsRetentionPolicyInitialized(ctx context.Context, category RetentionCategory) bool {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "SELECT", RetentionPolicyTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("SELECT"),
			attribute.String("db.collection", RetentionPolicyTableName),
			attribute.String("policy.category", string(category)),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(RetentionPolicyTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(RetentionPolicyTableName, "select").Inc()

	row := RetentionPolicy{Category: category}

	err := db.conn.Query(ctx, db.selectRetentionPolicyStmt, row).Get(&row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			span.SetStatus(codes.Ok, "no rows")
			return false
		}

		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return false
	}

	span.SetStatus(codes.Ok, "")

	return true
}

// SetRetentionPolicy upserts the retention policy for a category.
func (db *Database) SetRetentionPolicy(ctx context.Context, policy *RetentionPolicy) error {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "UPSERT", RetentionPolicyTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("UPSERT"),
			attribute.String("db.collection", RetentionPolicyTableName),
			attribute.String("policy.category", string(policy.Category)),
			attribute.Int("policy.days", policy.Days),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(RetentionPolicyTableName, "insert"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(RetentionPolicyTableName, "insert").Inc()

	err := db.conn.Query(ctx, db.upsertRetentionPolicyStmt, *policy).Run()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return nil
}
