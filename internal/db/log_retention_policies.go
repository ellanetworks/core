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

const LogRetentionPolicyTableName = "log_retention_policies"

const QueryCreateLogRetentionPolicyTable = `
	CREATE TABLE IF NOT EXISTS %s (
		id              INTEGER PRIMARY KEY AUTOINCREMENT,
		category        TEXT NOT NULL UNIQUE,
		retention_days  INTEGER NOT NULL CHECK (retention_days >= 1)
);`

const (
	selectRetentionPolicyStmt = "SELECT &LogRetentionPolicy.* FROM %s WHERE category = $LogRetentionPolicy.category"
	upsertRetentionPolicyStmt = `
INSERT INTO %s (category, retention_days)
VALUES ($LogRetentionPolicy.category, $LogRetentionPolicy.retention_days)
ON CONFLICT(category) DO UPDATE SET retention_days = excluded.retention_days
`
)

type LogCategory string

const (
	CategoryAuditLogs   LogCategory = "audit"
	CategoryNetworkLogs LogCategory = "network"
)

type LogRetentionPolicy struct {
	ID       int         `db:"id"`
	Category LogCategory `db:"category"`
	Days     int         `db:"retention_days"`
}

func (db *Database) GetLogRetentionPolicy(ctx context.Context, category LogCategory) (int, error) {
	const operation = "SELECT"
	const target = LogRetentionPolicyTableName
	spanName := fmt.Sprintf("%s %s", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	stmtStr := fmt.Sprintf(selectRetentionPolicyStmt, LogRetentionPolicyTableName)
	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(stmtStr),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
		attribute.String("policy.category", string(category)),
	)

	stmt, err := sqlair.Prepare(stmtStr, LogRetentionPolicy{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "prepare failed")
		return 0, err
	}

	arg := LogRetentionPolicy{Category: category}
	var row LogRetentionPolicy
	if err := db.conn.Query(ctx, stmt, arg).Get(&row); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")
		return 0, err
	}

	span.SetStatus(codes.Ok, "")
	return row.Days, nil
}

// Ensure that we have a row for the Audit Log retention policy.
func (db *Database) IsLogRetentionPolicyInitialized(ctx context.Context, category LogCategory) bool {
	operation := "SELECT"
	target := LogRetentionPolicyTableName
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

	row := LogRetentionPolicy{Category: category}
	q, err := sqlair.Prepare(stmt, LogRetentionPolicy{})
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

// SetLogRetentionPolicy upserts the retention policy for a category.
func (db *Database) SetLogRetentionPolicy(ctx context.Context, policy *LogRetentionPolicy) error {
	const operation = "UPSERT"
	const target = LogRetentionPolicyTableName
	spanName := fmt.Sprintf("%s %s", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	stmtStr := fmt.Sprintf(upsertRetentionPolicyStmt, LogRetentionPolicyTableName)
	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(stmtStr),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
		attribute.String("policy.category", string(policy.Category)),
		attribute.Int("policy.days", policy.Days),
	)

	stmt, err := sqlair.Prepare(stmtStr, LogRetentionPolicy{})
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
