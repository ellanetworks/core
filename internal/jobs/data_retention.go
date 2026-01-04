package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

func StartDataRetentionWorker(database *db.Database) {
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()

		for {
			ctx := context.Background()

			if err := enforceAuditDataRetention(ctx, database); err != nil {
				logger.EllaLog.Error("error enforcing audit log retention", zap.Error(err))
			}

			if err := enforceRadioDataRetention(ctx, database); err != nil {
				logger.EllaLog.Error("error enforcing radio log retention", zap.Error(err))
			}

			if err := enforceSubscriberUsageDataRetention(ctx, database); err != nil {
				logger.EllaLog.Error("error enforcing subscriber usage data retention", zap.Error(err))
			}

			<-ticker.C
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
