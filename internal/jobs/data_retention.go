// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package jobs

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

const retentionInterval = 24 * time.Hour

func RunDataRetentionWorker(ctx context.Context, database *db.Database) {
	ticker := time.NewTicker(retentionInterval)
	defer ticker.Stop()

	runRetentionPass(ctx, database, database.IsLeader())

	for {
		select {
		case <-ctx.Done():
			logger.EllaLog.Info("Data retention worker stopped")
			return
		case <-ticker.C:
		}

		runRetentionPass(ctx, database, database.IsLeader())
	}
}

func runRetentionPass(ctx context.Context, database *db.Database, isLeader bool) {
	if err := enforceRadioDataRetention(ctx, database); err != nil {
		logger.EllaLog.Error("error enforcing radio log retention", zap.Error(err))
	}

	if err := enforceFlowReportsDataRetention(ctx, database); err != nil {
		logger.EllaLog.Error("error enforcing flow reports retention", zap.Error(err))
	}

	if err := enforceAuditDataRetention(ctx, database); err != nil {
		logger.EllaLog.Error("error enforcing audit log retention", zap.Error(err))
	}

	if !isLeader {
		return
	}

	if err := enforceSubscriberUsageDataRetention(ctx, database); err != nil {
		logger.EllaLog.Error("error enforcing subscriber usage data retention", zap.Error(err))
	}
}

func retentionDays(ctx context.Context, database *db.Database, category db.RetentionCategory) (int, bool, error) {
	days, err := database.GetRetentionPolicy(ctx, category)
	if errors.Is(err, sql.ErrNoRows) {
		logger.EllaLog.Warn("no retention policy configured; skipping retention for this category",
			zap.String("category", string(category)))

		return 0, false, nil
	}

	if err != nil {
		return 0, false, err
	}

	return days, true, nil
}

func enforceAuditDataRetention(ctx context.Context, database *db.Database) error {
	days, ok, err := retentionDays(ctx, database, db.CategoryAuditLogs)
	if err != nil {
		return err
	}

	if !ok {
		return nil
	}

	return database.DeleteOldAuditLogs(ctx, days)
}

func enforceRadioDataRetention(ctx context.Context, database *db.Database) error {
	days, ok, err := retentionDays(ctx, database, db.CategoryRadioLogs)
	if err != nil {
		return err
	}

	if !ok {
		return nil
	}

	return database.DeleteOldRadioEvents(ctx, days)
}

func enforceSubscriberUsageDataRetention(ctx context.Context, database *db.Database) error {
	days, ok, err := retentionDays(ctx, database, db.CategorySubscriberUsage)
	if err != nil {
		return fmt.Errorf("failed to get subscriber usage retention policy: %w", err)
	}

	if !ok {
		return nil
	}

	if err := database.DeleteOldDailyUsage(ctx, days); err != nil {
		return fmt.Errorf("failed to delete old daily usage data: %w", err)
	}

	return nil
}

func enforceFlowReportsDataRetention(ctx context.Context, database *db.Database) error {
	days, ok, err := retentionDays(ctx, database, db.CategoryFlowReports)
	if err != nil {
		return fmt.Errorf("failed to get flow reports retention policy: %w", err)
	}

	if !ok {
		return nil
	}

	if err := database.DeleteOldFlowReports(ctx, days); err != nil {
		return fmt.Errorf("failed to delete old flow reports: %w", err)
	}

	return nil
}
