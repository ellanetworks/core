package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/canonical/sqlair"
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
	const operation = "SELECT"
	const target = RetentionPolicyTableName
	spanName := fmt.Sprintf("%s %s", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	stmtStr := fmt.Sprintf(selectRetentionPolicyStmt, RetentionPolicyTableName)
	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(stmtStr),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
		attribute.String("policy.category", string(category)),
	)

	stmt, err := sqlair.Prepare(stmtStr, RetentionPolicy{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "prepare failed")
		return 0, err
	}

	arg := RetentionPolicy{Category: category}
	var row RetentionPolicy
	if err := db.conn.Query(ctx, stmt, arg).Get(&row); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")
		return 0, err
	}

	span.SetStatus(codes.Ok, "")
	return row.Days, nil
}

// Ensure that we have a row for the Audit Log retention policy.
func (db *Database) IsRetentionPolicyInitialized(ctx context.Context, category RetentionCategory) bool {
	operation := "SELECT"
	target := RetentionPolicyTableName
	spanName := fmt.Sprintf("%s %s", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	stmt := fmt.Sprintf(selectRetentionPolicyStmt, db.retentionPoliciesTable)
	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(stmt),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
	)

	row := RetentionPolicy{Category: category}
	q, err := sqlair.Prepare(stmt, RetentionPolicy{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "prepare failed")
		return false
	}

	if err := db.conn.Query(ctx, q, row).Get(&row); err != nil {
		if err == sql.ErrNoRows {
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
	const operation = "UPSERT"
	const target = RetentionPolicyTableName
	spanName := fmt.Sprintf("%s %s", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	stmtStr := fmt.Sprintf(upsertRetentionPolicyStmt, RetentionPolicyTableName)
	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(stmtStr),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
		attribute.String("policy.category", string(policy.Category)),
		attribute.Int("policy.days", policy.Days),
	)

	stmt, err := sqlair.Prepare(stmtStr, RetentionPolicy{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "prepare failed")
		return err
	}

	if err := db.conn.Query(ctx, stmt, *policy).Run(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")
		return err
	}

	span.SetStatus(codes.Ok, "")
	return nil
}
