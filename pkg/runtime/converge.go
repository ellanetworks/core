// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package runtime

import (
	"context"
	"errors"
	"time"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

const (
	leaseReleaseInitialBackoff = 1 * time.Second
	leaseReleaseMaxBackoff     = 30 * time.Second
)

var (
	attributePublishInitialBackoff = 1 * time.Second
	attributePublishMaxBackoff     = 30 * time.Second
)

type memberAttributePublisher interface {
	PublishClusterMemberAttributes(ctx context.Context, binaryVersion string) error
}

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

func publishClusterMemberAttributes(ctx context.Context, publisher memberAttributePublisher, binaryVersion string) {
	backoff := attributePublishInitialBackoff

	for {
		err := publisher.PublishClusterMemberAttributes(ctx, binaryVersion)
		if err == nil {
			return
		}

		if ctx.Err() != nil {
			return
		}

		if errors.Is(err, db.ErrRemovedFromCluster) {
			logger.EllaLog.Warn("this node is no longer a cluster member; not publishing its address and version",
				zap.Error(err))

			return
		}

		if errors.Is(err, db.ErrForwardRejected) {
			logger.EllaLog.Info("leader does not support publishing member attributes yet; retrying",
				zap.Error(err),
				zap.Duration("next_backoff", backoff))
		} else {
			logger.EllaLog.Warn("could not publish this node's address and version; retrying",
				zap.Error(err),
				zap.Duration("next_backoff", backoff))
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}

		backoff *= 2
		if backoff > attributePublishMaxBackoff {
			backoff = attributePublishMaxBackoff
		}
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
