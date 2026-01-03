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
			if err := enforceAuditDataRetention(database); err != nil {
				logger.EllaLog.Error("error enforcing audit log retention", zap.Error(err))
			}

			if err := enforceRadioDataRetention(database); err != nil {
				logger.EllaLog.Error("error enforcing radio log retention", zap.Error(err))
			}

			if err := enforceSubscriberUsageDataRetention(database); err != nil {
				logger.EllaLog.Error("error enforcing subscriber usage data retention", zap.Error(err))
			}

			<-ticker.C
		}
	}()
}

func enforceAuditDataRetention(database *db.Database) error {
	ctx := context.Background()

	days, err := database.GetRetentionPolicy(ctx, db.CategoryAuditLogs)
	if err != nil {
		return err
	}

	if err := database.DeleteOldAuditLogs(ctx, days); err != nil {
		return err
	}

	return nil
}

func enforceRadioDataRetention(database *db.Database) error {
	ctx := context.Background()

	days, err := database.GetRetentionPolicy(ctx, db.CategoryRadioLogs)
	if err != nil {
		return err
	}

	if err := database.DeleteOldRadioEvents(ctx, days); err != nil {
		return err
	}

	return nil
}

func enforceSubscriberUsageDataRetention(database *db.Database) error {
	ctx := context.Background()

	days, err := database.GetRetentionPolicy(ctx, db.CategorySubscriberUsage)
	if err != nil {
		return fmt.Errorf("failed to get subscriber usage retention policy: %v", err)
	}

	if err := database.DeleteOldDailyUsage(ctx, days); err != nil {
		return fmt.Errorf("failed to delete old daily usage data: %v", err)
	}

	return nil
}
