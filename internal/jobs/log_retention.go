package jobs

import (
	"context"
	"time"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

func StartLogRetentionWorker(database *db.Database) {
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()

		for {
			if err := enforceAuditLogRetention(database); err != nil {
				logger.EllaLog.Error("error enforcing audit log retention", zap.Error(err))
			}
			if err := enforceSubscriberLogRetention(database); err != nil {
				logger.EllaLog.Error("error enforcing subscriber log retention", zap.Error(err))
			}
			if err := enforceRadioLogRetention(database); err != nil {
				logger.EllaLog.Error("error enforcing radio log retention", zap.Error(err))
			}

			<-ticker.C
		}
	}()
}

func enforceAuditLogRetention(database *db.Database) error {
	ctx := context.Background()

	days, err := database.GetLogRetentionPolicy(ctx, db.CategoryAuditLogs)
	if err != nil {
		return err
	}

	if err := database.DeleteOldAuditLogs(ctx, days); err != nil {
		return err
	}

	return nil
}

func enforceSubscriberLogRetention(database *db.Database) error {
	ctx := context.Background()

	days, err := database.GetLogRetentionPolicy(ctx, db.CategorySubscriberLogs)
	if err != nil {
		return err
	}

	if err := database.DeleteOldSubscriberLogs(ctx, days); err != nil {
		return err
	}

	return nil
}

func enforceRadioLogRetention(database *db.Database) error {
	ctx := context.Background()

	days, err := database.GetLogRetentionPolicy(ctx, db.CategoryRadioLogs)
	if err != nil {
		return err
	}

	if err := database.DeleteOldRadioLogs(ctx, days); err != nil {
		return err
	}

	return nil
}
