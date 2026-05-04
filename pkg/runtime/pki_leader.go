// Copyright 2026 Ella Networks

package runtime

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ellanetworks/core/internal/api/server"
	"github.com/ellanetworks/core/internal/cluster/listener"
	"github.com/ellanetworks/core/internal/cluster/pkiissuer"
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
	clusterLn     *listener.Listener
	nodeID        int
	binaryVersion string

	needsDRSnapshot bool

	bootstrapRegistered sync.Once

	mu           sync.Mutex
	leaderCancel context.CancelFunc
}

func newPKILeaderCallback(ctx context.Context, state *pkiState, dbInstance *db.Database, ln *listener.Listener, nodeID int, binaryVersion string, needsDRSnapshot bool) *pkiLeaderCallback {
	return &pkiLeaderCallback{
		ctx:             ctx,
		state:           state,
		dbInstance:      dbInstance,
		clusterLn:       ln,
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

	if c.needsDRSnapshot {
		if err := c.dbInstance.SelfRestore(leaderCtx); err != nil {
			logger.EllaLog.Warn("post-DR self-restore failed", zap.Error(err))
			return
		}

		c.needsDRSnapshot = false
	}

	if err := runLeaderInit(leaderCtx, c.state, c.dbInstance, c.nodeID, c.binaryVersion); err != nil {
		logger.EllaLog.Warn("leader init failed; yielding leadership and scheduling retry",
			zap.Error(err))

		c.yieldLeadership()

		go c.retryLeaderInit(leaderCtx)

		return
	}

	c.onLeaderInitSuccess()
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

func (c *pkiLeaderCallback) yieldLeadership() {
	if err := c.dbInstance.LeadershipTransfer(); err != nil {
		logger.EllaLog.Debug("leadership transfer after init failure",
			zap.Error(err))
	}
}

func (c *pkiLeaderCallback) retryLeaderInit(ctx context.Context) {
	backoff := leaderInitInitialBackoff

	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}

		err := runLeaderInit(ctx, c.state, c.dbInstance, c.nodeID, c.binaryVersion)
		if err == nil {
			logger.EllaLog.Info("leader init recovered after retry")
			c.onLeaderInitSuccess()

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

func (c *pkiLeaderCallback) onLeaderInitSuccess() {
	if c.clusterLn != nil && c.state != nil && c.state.issuer != nil {
		c.bootstrapRegistered.Do(func() {
			server.RegisterBootstrapALPN(c.clusterLn, c.state.issuer)
		})
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

	if err := dbInstance.DeleteAllDynamicLeases(ctx); err != nil {
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
	if p.issuer == nil {
		p.issuer = pkiissuer.New(dbInstance)
	}

	if err := p.issuer.Bootstrap(ctx); err != nil {
		return fmt.Errorf("issuer bootstrap: %w", err)
	}

	// Step 3: pin the leader's own cert in cluster_node_certs (if not
	// already there). This is what lets MintJoinToken later embed
	// the leader's pin in tokens.
	leaf := p.agent.Leaf()
	if leaf != nil && leaf.Leaf != nil {
		certPEM := pki.EncodeCertPEM(leaf.Leaf)
		if _, err := p.issuer.RegisterCert(ctx, nodeID, certPEM); err != nil {
			return fmt.Errorf("register leader cert: %w", err)
		}
	}

	// Step 4: refresh the in-memory pin map so the listener sees the
	// just-registered leader pin and any others the new leader's
	// snapshot loaded.
	if err := p.RefreshPins(ctx, dbInstance); err != nil {
		return fmt.Errorf("refresh pin cache: %w", err)
	}

	server.SetPKIIssuer(p.issuer)

	return nil
}
