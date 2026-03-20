package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

func StartDataRetentionWorker(ctx context.Context, database *db.Database) {
	go func() { // #nosec: G118 -- Background context is intentional — retention operations must not be cancelled by shutdown
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()

		for {
			bgCtx := context.Background()

			if err := enforceAuditDataRetention(bgCtx, database); err != nil {
				logger.EllaLog.Error("error enforcing audit log retention", zap.Error(err))
			}

			if err := enforceRadioDataRetention(bgCtx, database); err != nil {
				logger.EllaLog.Error("error enforcing radio log retention", zap.Error(err))
			}

			if err := enforceSubscriberUsageDataRetention(bgCtx, database); err != nil {
				logger.EllaLog.Error("error enforcing subscriber usage data retention", zap.Error(err))
			}

			if err := enforceFlowReportsDataRetention(bgCtx, database); err != nil {
				logger.EllaLog.Error("error enforcing flow reports retention", zap.Error(err))
			}

			select {
			case <-ctx.Done():
				logger.EllaLog.Info("Data retention worker stopped")
				return
			case <-ticker.C:
			}
		}
	}()
}

func enforceAuditDataRetention(ctx context.Context, database *db.Database) error {
	days, err := database.GetRetentionPolicy(ctx, db.CategoryAuditLogs)
	if err != nil {
		return err
	}

	if err := database.DeleteOldAuditLogs(ctx, days); err != nil {
		return err
	}

	return nil
}

func enforceRadioDataRetention(ctx context.Context, database *db.Database) error {
	days, err := database.GetRetentionPolicy(ctx, db.CategoryRadioLogs)
	if err != nil {
		return err
	}

	if err := database.DeleteOldRadioEvents(ctx, days); err != nil {
		return err
	}

	return nil
}

func enforceSubscriberUsageDataRetention(ctx context.Context, database *db.Database) error {
	days, err := database.GetRetentionPolicy(ctx, db.CategorySubscriberUsage)
	if err != nil {
		return fmt.Errorf("failed to get subscriber usage retention policy: %v", err)
	}

	if err := database.DeleteOldDailyUsage(ctx, days); err != nil {
		return fmt.Errorf("failed to delete old daily usage data: %v", err)
	}

	return nil
}

func enforceFlowReportsDataRetention(ctx context.Context, database *db.Database) error {
	days, err := database.GetRetentionPolicy(ctx, db.CategoryFlowReports)
	if err != nil {
		return fmt.Errorf("failed to get flow reports retention policy: %v", err)
	}

	if err := database.DeleteOldFlowReports(ctx, days); err != nil {
		return fmt.Errorf("failed to delete old flow reports: %v", err)
	}

	return nil
}
