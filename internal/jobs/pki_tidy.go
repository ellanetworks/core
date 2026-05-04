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
// cluster_join_tokens. Runs on the leader only (gated by guard);
// mirrors the RunDataRetentionWorker pattern.
//
// In the post-v12 PKI design there are no issued-cert or revocation
// rows to tidy — the only growable table is cluster_join_tokens.
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
