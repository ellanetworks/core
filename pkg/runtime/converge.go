// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package runtime

import (
	"context"
	"time"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

const (
	leaseReleaseInitialBackoff = 1 * time.Second
	leaseReleaseMaxBackoff     = 30 * time.Second
)

func awaitInitialSettings(ctx context.Context, dbInstance *db.Database, pki *pkiState) {
	logger.EllaLog.Info("Waiting for a raft leader")

	if err := dbInstance.WaitForLeader(ctx); err != nil {
		logger.EllaLog.Info("Stopped waiting for a raft leader", zap.Error(err))
		return
	}

	logger.EllaLog.Info("Waiting for initial settings")

	if err := dbInstance.WaitForInitialization(ctx, 0); err != nil {
		logger.EllaLog.Info("Stopped waiting for initial settings", zap.Error(err))
		return
	}

	logger.EllaLog.Info("Initial settings available")

	releaseStaleLeases(ctx, dbInstance)

	if pki != nil {
		pki.ensureIssuer(dbInstance)
	}
}

func releaseStaleLeases(ctx context.Context, dbInstance *db.Database) {
	backoff := leaseReleaseInitialBackoff

	for {
		err := clearStaleDynamicLeases(ctx, dbInstance)
		if err == nil {
			return
		}

		if ctx.Err() != nil {
			return
		}

		logger.EllaLog.Warn("could not release this node's stale dynamic leases; retrying",
			zap.Error(err),
			zap.Duration("next_backoff", backoff))

		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}

		backoff *= 2
		if backoff > leaseReleaseMaxBackoff {
			backoff = leaseReleaseMaxBackoff
		}
	}
}
