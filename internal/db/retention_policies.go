// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
)

const RetentionPolicyTableName = "retention_policies"

const (
	selectRetentionPolicyStmt = "SELECT retention_days FROM " + RetentionPolicyTableName + " WHERE category = ?"
	upsertRetentionPolicyStmt = `
INSERT INTO ` + RetentionPolicyTableName + ` (id, category, retention_days)
VALUES (?, ?, ?)
ON CONFLICT(category) DO UPDATE SET retention_days = excluded.retention_days
`
)

type RetentionCategory string

const (
	CategoryAuditLogs       RetentionCategory = "audit"
	CategoryRadioLogs       RetentionCategory = "radio"
	CategorySubscriberUsage RetentionCategory = "usage"
	CategoryFlowReports     RetentionCategory = "flow_reports"
)

type RetentionPolicy struct {
	ID       string // UUIDv7
	Category RetentionCategory
	Days     int
}

func (db *Database) GetRetentionPolicy(ctx context.Context, category RetentionCategory) (int, error) {
	ctx, span, timer := startDBQuery(ctx, RetentionPolicyTableName, "SELECT", "select", attribute.String("policy.category", string(category)))
	defer span.End()
	defer timer.ObserveDuration()

	var days int

	err := db.reader().QueryRowContext(ctx, selectRetentionPolicyStmt, string(category)).Scan(&days)
	if err != nil {
		recordSpanError(span, err)

		return 0, fmt.Errorf("query failed: %w", err)
	}

	return days, nil
}

// Ensure that we have a row for the Audit Log retention policy.
func (db *Database) IsRetentionPolicyInitialized(ctx context.Context, category RetentionCategory) bool {
	ctx, span, timer := startDBQuery(ctx, RetentionPolicyTableName, "SELECT", "select", attribute.String("policy.category", string(category)))
	defer span.End()
	defer timer.ObserveDuration()

	var days int

	err := db.reader().QueryRowContext(ctx, selectRetentionPolicyStmt, string(category)).Scan(&days)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false
		}

		recordSpanError(span, err)

		return false
	}

	return true
}

// SetRetentionPolicy upserts the retention policy for a category.
// If policy.ID is empty (new row, not an update of an existing one),
// a UUIDv7 is generated. The id is ignored on UPDATE conflict.
func (db *Database) SetRetentionPolicy(ctx context.Context, policy *RetentionPolicy) error {
	ctx, span, timer := startDBQuery(ctx, RetentionPolicyTableName, "UPSERT", "insert", attribute.String("policy.category", string(policy.Category)), attribute.Int("policy.days", policy.Days))
	defer span.End()
	defer timer.ObserveDuration()

	if policy.ID == "" {
		id, err := uuid.NewV7()
		if err != nil {
			recordSpanError(span, err)

			return fmt.Errorf("generate retention policy id: %w", err)
		}

		policy.ID = id.String()
	}

	_, err := opSetRetentionPolicy.Invoke(ctx, db, policy)
	if err != nil {
		recordSpanError(span, err)

		return err
	}

	return nil
}
