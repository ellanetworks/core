// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package runtime

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/pki"
	"go.uber.org/zap"
)

var (
	leaderInitInitialBackoff = time.Second
	leaderInitMaxBackoff     = 30 * time.Second
)

var errDRRestorePending = errors.New("post-DR self-restore pending")

type leaderDB interface {
	LeadershipTransfer() error
	SelfRestore(ctx context.Context) error
}

type pkiLeader struct {
	db      leaderDB
	runInit func(context.Context) error

	needsDRSnapshot atomic.Bool
}

func newPKILeader(state *pkiState, dbInstance *db.Database, nodeID string, binaryVersion string, needsDRSnapshot bool) *pkiLeader {
	l := &pkiLeader{
		db: dbInstance,
		runInit: func(leaderCtx context.Context) error {
			return runLeaderInit(leaderCtx, state, dbInstance, nodeID, binaryVersion)
		},
	}

	l.needsDRSnapshot.Store(needsDRSnapshot)

	return l
}

func (l *pkiLeader) run(ctx context.Context) {
	err := l.runLeaderSequence(ctx)
	if err == nil {
		return
	}

	if errors.Is(err, errDRRestorePending) {
		logger.EllaLog.Error("post-DR self-restore failed; keeping leadership and retrying, the restored state is not the cluster baseline yet",
			zap.Error(err))

		l.retryLeaderInit(ctx)

		return
	}

	logger.EllaLog.Warn("leader init failed; yielding leadership", zap.Error(err))

	if transferErr := l.db.LeadershipTransfer(); transferErr != nil {
		logger.EllaLog.Error("leadership transfer after init failure; staying leader and retrying",
			zap.Error(transferErr))

		l.retryLeaderInit(ctx)
	}
}

func (l *pkiLeader) runLeaderSequence(ctx context.Context) error {
	if l.needsDRSnapshot.Load() {
		if err := l.db.SelfRestore(ctx); err != nil {
			return fmt.Errorf("%w: %w", errDRRestorePending, err)
		}

		l.needsDRSnapshot.Store(false)
	}

	return l.runInit(ctx)
}

func (l *pkiLeader) retryLeaderInit(ctx context.Context) {
	backoff := leaderInitInitialBackoff

	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}

		err := l.runLeaderSequence(ctx)
		if err == nil {
			logger.EllaLog.Info("leader init recovered after retry")

			return
		}

		backoff *= 2
		if backoff > leaderInitMaxBackoff {
			backoff = leaderInitMaxBackoff
		}

		logger.EllaLog.Warn("leader init retry failed",
			zap.Error(err),
			zap.Duration("next_backoff", backoff))
	}
}

// runLeaderInit is idempotent.
func runLeaderInit(ctx context.Context, pki *pkiState, dbInstance *db.Database, nodeID string, binaryVersion string) error {
	if err := dbInstance.Initialize(ctx); err != nil {
		return fmt.Errorf("initialize: %w", err)
	}

	if err := dbInstance.PostInitClusterSetup(ctx, binaryVersion); err != nil {
		return fmt.Errorf("post-init cluster setup: %w", err)
	}

	if err := clearStaleDynamicLeases(ctx, dbInstance); err != nil {
		return fmt.Errorf("delete dynamic leases: %w", err)
	}

	if pki != nil {
		if err := setupLeaderPKI(ctx, pki, dbInstance, nodeID); err != nil {
			return fmt.Errorf("setup pki: %w", err)
		}
	}

	return nil
}

func setupLeaderPKI(ctx context.Context, p *pkiState, dbInstance *db.Database, nodeID string) error {
	// Step 1: ensure this node's self-signed cert exists. On a fresh
	// first-leader boot the cert was not created by JoinFlow, so we
	// generate one here. The clusterID is now populated by
	// PostInitClusterSetup.
	if !p.agent.HaveLeafOnDisk() {
		op, err := dbInstance.GetOperator(ctx)
		if err != nil {
			return fmt.Errorf("get operator: %w", err)
		}

		if op.ClusterID == "" {
			return fmt.Errorf("clusterID still empty after PostInitClusterSetup")
		}

		p.agent.ClusterID = op.ClusterID

		if err := p.agent.GenerateAndPersist(); err != nil {
			return fmt.Errorf("generate self-signed cert: %w", err)
		}
	} else if p.agent.Leaf() == nil {
		if err := p.agent.Load(); err != nil {
			return fmt.Errorf("load existing cert: %w", err)
		}
	}

	// Step 2: install the issuer so the leader can mint join tokens
	// and accept register requests.
	p.ensureIssuer(dbInstance)

	if err := p.issuer.Bootstrap(ctx); err != nil {
		return fmt.Errorf("issuer bootstrap: %w", err)
	}

	// Step 3: pin the leader's own cert in cluster_node_certs (if not
	// already there). This is what lets MintJoinToken later embed
	// the leader's pin in tokens.
	leaf := p.agent.Leaf()
	if leaf != nil && leaf.Leaf != nil {
		certPEM := pki.EncodeCertPEM(leaf.Leaf)
		if _, _, err := p.issuer.RegisterCert(ctx, nodeID, certPEM); err != nil {
			return fmt.Errorf("register leader cert: %w", err)
		}
	}

	// Step 4: refresh the in-memory pin map so the listener sees the
	// just-registered leader pin and any others the new leader's
	// snapshot loaded.
	if err := p.RefreshPins(ctx, dbInstance); err != nil {
		return fmt.Errorf("refresh pin cache: %w", err)
	}

	return nil
}
