package jobs

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

// LeaderGuard tracks leadership state for the data retention worker.
// Implements raft.LeaderCallback.
type LeaderGuard struct {
	isLeader atomic.Bool
}

func NewLeaderGuard() *LeaderGuard {
	return &LeaderGuard{}
}

func (g *LeaderGuard) OnBecameLeader()   { g.isLeader.Store(true) }
func (g *LeaderGuard) OnLostLeadership() { g.isLeader.Store(false) }
func (g *LeaderGuard) IsLeader() bool    { return g.isLeader.Load() }

// RunDataRetentionWorker runs the data retention loop. It blocks until ctx
// is cancelled, so callers should invoke it in a goroutine.
func RunDataRetentionWorker(ctx context.Context, database *db.Database, guard *LeaderGuard) {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.EllaLog.Info("Data retention worker stopped")
			return
		case <-ticker.C:
		}

		if !guard.IsLeader() {
			continue
		}

		if err := enforceAuditDataRetention(ctx, database); err != nil {
			logger.EllaLog.Error("error enforcing audit log retention", zap.Error(err))
		}

		if err := enforceRadioDataRetention(ctx, database); err != nil {
			logger.EllaLog.Error("error enforcing radio log retention", zap.Error(err))
		}

		if err := enforceSubscriberUsageDataRetention(ctx, database); err != nil {
			logger.EllaLog.Error("error enforcing subscriber usage data retention", zap.Error(err))
		}

		if err := enforceFlowReportsDataRetention(ctx, database); err != nil {
			logger.EllaLog.Error("error enforcing flow reports retention", zap.Error(err))
		}
	}
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
