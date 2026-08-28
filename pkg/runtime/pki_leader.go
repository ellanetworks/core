// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package runtime

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/pki"
	"go.uber.org/zap"
)

const (
	leaderInitInitialBackoff = time.Second
	leaderInitMaxBackoff     = 30 * time.Second
)

type pkiLeaderCallback struct {
	ctx           context.Context
	state         *pkiState
	dbInstance    *db.Database
	nodeID        int
	binaryVersion string

	needsDRSnapshot bool

	mu           sync.Mutex
	leaderCancel context.CancelFunc
}

func newPKILeaderCallback(ctx context.Context, state *pkiState, dbInstance *db.Database, nodeID int, binaryVersion string, needsDRSnapshot bool) *pkiLeaderCallback {
	return &pkiLeaderCallback{
		ctx:             ctx,
		state:           state,
		dbInstance:      dbInstance,
		nodeID:          nodeID,
		binaryVersion:   binaryVersion,
		needsDRSnapshot: needsDRSnapshot,
	}
}

// OnBecameLeader runs runLeaderInit synchronously on the typical
// success path. On failure it yields leadership and retries in
// background.
func (c *pkiLeaderCallback) OnBecameLeader() {
	leaderCtx := c.beginLeaderTerm()

	if err := c.runLeaderSequence(leaderCtx); err != nil {
		logger.EllaLog.Warn("leader init failed; yielding leadership and scheduling retry",
			zap.Error(err))

		if transferErr := c.yieldLeadership(); transferErr != nil {
			logger.EllaLog.Error("leadership transfer after init failure; staying leader and retrying",
				zap.Error(transferErr))
		}

		go c.retryLeaderInit(leaderCtx)

		return
	}
}

func (c *pkiLeaderCallback) OnLostLeadership() {
	c.mu.Lock()
	if c.leaderCancel != nil {
		c.leaderCancel()
		c.leaderCancel = nil
	}
	c.mu.Unlock()
}

func (c *pkiLeaderCallback) beginLeaderTerm() context.Context {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.leaderCancel != nil {
		c.leaderCancel()
	}

	leaderCtx, cancel := context.WithCancel(c.ctx)
	c.leaderCancel = cancel

	return leaderCtx
}

func (c *pkiLeaderCallback) yieldLeadership() error {
	return c.dbInstance.LeadershipTransfer()
}

func (c *pkiLeaderCallback) runLeaderSequence(ctx context.Context) error {
	c.mu.Lock()
	needsRestore := c.needsDRSnapshot
	c.mu.Unlock()

	if needsRestore {
		if err := c.dbInstance.SelfRestore(ctx); err != nil {
			return fmt.Errorf("post-DR self-restore: %w", err)
		}

		c.mu.Lock()
		c.needsDRSnapshot = false
		c.mu.Unlock()
	}

	return runLeaderInit(ctx, c.state, c.dbInstance, c.nodeID, c.binaryVersion)
}

func (c *pkiLeaderCallback) retryLeaderInit(ctx context.Context) {
	backoff := leaderInitInitialBackoff

	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}

		if !c.dbInstance.IsLeader() {
			logger.EllaLog.Info("leader init retry stopping; no longer leader")

			return
		}

		err := c.runLeaderSequence(ctx)
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
func runLeaderInit(ctx context.Context, pki *pkiState, dbInstance *db.Database, nodeID int, binaryVersion string) error {
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

func setupLeaderPKI(ctx context.Context, p *pkiState, dbInstance *db.Database, nodeID int) error {
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
