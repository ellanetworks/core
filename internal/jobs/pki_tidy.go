// Copyright 2026 Ella Networks

package jobs

import (
	"context"
	"time"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

// RunJoinTokenTidyWorker prunes expired and old-consumed rows from
// cluster_join_tokens hourly. Runs on the leader only (gated by
// guard); follows the RunDataRetentionWorker pattern.
func RunJoinTokenTidyWorker(ctx context.Context, database *db.Database, guard *LeaderGuard) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.EllaLog.Info("join-token tidy worker stopped")
			return
		case <-ticker.C:
		}

		if !guard.IsLeader() {
			continue
		}

		if err := database.DeleteStaleJoinTokens(ctx, time.Now()); err != nil {
			logger.EllaLog.Warn("join-token tidy: delete stale tokens failed", zap.Error(err))
		}
	}
}
