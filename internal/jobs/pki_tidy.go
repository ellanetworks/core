// Copyright 2026 Ella Networks

package jobs

import (
	"context"
	"time"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

// RunPKITidyWorker prunes expired rows from the cluster_revoked_certs,
// cluster_issued_certs, and cluster_join_tokens tables. Runs on the
// leader only (gated by guard); mirrors the RunDataRetentionWorker
// pattern.
func RunPKITidyWorker(ctx context.Context, database *db.Database, guard *LeaderGuard) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.EllaLog.Info("PKI tidy worker stopped")
			return
		case <-ticker.C:
		}

		if !guard.IsLeader() {
			continue
		}

		runPKITidyOnce(ctx, database)
	}
}

func runPKITidyOnce(ctx context.Context, database *db.Database) {
	now := time.Now()

	if err := database.DeletePurgedRevocations(ctx, now); err != nil {
		logger.EllaLog.Warn("pki tidy: delete purged revocations failed", zap.Error(err))
	}

	if err := database.DeleteExpiredIssuedCerts(ctx, now.Add(-time.Hour)); err != nil {
		logger.EllaLog.Warn("pki tidy: delete expired issued certs failed", zap.Error(err))
	}

	if err := database.DeleteStaleJoinTokens(ctx, now); err != nil {
		logger.EllaLog.Warn("pki tidy: delete stale join tokens failed", zap.Error(err))
	}
}
