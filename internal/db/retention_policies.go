// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
)

const RetentionPolicyTableName = "retention_policies"

const (
	selectRetentionPolicySQL = "SELECT retention_days FROM " + RetentionPolicyTableName + " WHERE category = ?"
	upsertRetentionPolicySQL = `
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

func (db *Database) GetRetentionPolicy(ctx context.Context, category RetentionCategory) (_ int, err error) {
	ctx, finish := startDBQuery(ctx, RetentionPolicyTableName, "SELECT", attribute.String("policy.category", string(category)))
	defer func() { finish(err) }()

	var days int

	err = db.reader().QueryRowContext(ctx, selectRetentionPolicySQL, string(category)).Scan(&days)
	if err != nil {
		return 0, fmt.Errorf("query failed: %w", err)
	}

	return days, nil
}

// Ensure that we have a row for the Audit Log retention policy.
func (db *Database) IsRetentionPolicyInitialized(ctx context.Context, category RetentionCategory) bool {
	ctx, finish := startDBQuery(ctx, RetentionPolicyTableName, "SELECT", attribute.String("policy.category", string(category)))

	var days int

	err := db.reader().QueryRowContext(ctx, selectRetentionPolicySQL, string(category)).Scan(&days)
	finish(err)

	return err == nil
}

// SetRetentionPolicy upserts the retention policy for a category.
// If policy.ID is empty (new row, not an update of an existing one),
// a UUIDv7 is generated. The id is ignored on UPDATE conflict.
func (db *Database) SetRetentionPolicy(ctx context.Context, policy *RetentionPolicy) (err error) {
	ctx, finish := startDBQuery(ctx, RetentionPolicyTableName, "UPSERT", attribute.String("policy.category", string(policy.Category)), attribute.Int("policy.days", policy.Days))
	defer func() { finish(err) }()

	if policy.ID == "" {
		id, err := uuid.NewV7()
		if err != nil {
			return fmt.Errorf("generate retention policy id: %w", err)
		}

		policy.ID = id.String()
	}

	_, err = opSetRetentionPolicy.Invoke(ctx, db, policy)
	if err != nil {
		return err
	}

	return nil
}
