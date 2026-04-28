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
	ellapki "github.com/ellanetworks/core/internal/pki"
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

	// needsDRSnapshot is true when this node was bootstrapped from a
	// restore bundle. Its FSM carries state that wasn't built up by
	// replicated log entries, so a fresh joiner replaying the log
	// from index 1 would hit changeset conflicts on the UPDATE
	// changesets produced by the leader's post-bootstrap work (PKI
	// mint etc). On the first leader transition we re-inject the
	// current DB as a raft user snapshot, which truncates the log
	// and forces joiners through InstallSnapshot. Cleared after a
	// successful SelfRestore; OnBecameLeader is single-threaded from
	// the observer so no mutex is needed.
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
// success path so the leader's init completes before observer.Register
// returns. On failure it yields leadership and retries in the
// background under a leadership-scoped context.
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

	if c.state != nil && c.state.issuer != nil {
		c.state.issuer.UnloadKeys()
	}
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

// yieldLeadership best-effort transfers leadership to a follower.
// Fails (and is logged at Debug) on a single-node cluster — the retry
// loop covers that case.
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
	// listener.Register panics on duplicate ALPN, so guard with Once.
	if c.clusterLn != nil && c.state != nil && c.state.issuer != nil {
		c.bootstrapRegistered.Do(func() {
			server.RegisterBootstrapALPN(c.clusterLn, c.state.issuer)
		})
	}
}

// runLeaderInit is idempotent; each step's invariant is checked
// before any write.
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

func setupLeaderPKI(ctx context.Context, pki *pkiState, dbInstance *db.Database, nodeID int) error {
	if pki.issuer == nil {
		pki.issuer = pkiissuer.New(dbInstance)
	}

	if err := pki.issuer.Bootstrap(ctx); err != nil {
		return fmt.Errorf("issuer bootstrap: %w", err)
	}

	// A voter promoted before raft replicated the CA tables will leave
	// LoadKeys with no active rows to load; Ready stays false and the
	// next election re-runs this path.
	if err := pki.issuer.LoadKeys(ctx); err != nil {
		return fmt.Errorf("issuer load keys: %w", err)
	}

	if err := pki.RefreshBundle(ctx); err != nil {
		return fmt.Errorf("refresh bundle: %w", err)
	}

	if err := pki.RefreshRevocations(ctx, dbInstance); err != nil {
		logger.EllaLog.Warn("refresh revocations", zap.Error(err))
	}

	if pki.agent.Leaf() == nil && pki.issuer.Ready() {
		if err := selfIssueLeaf(ctx, pki, nodeID); err != nil {
			return fmt.Errorf("self-issue leaf: %w", err)
		}
	}

	server.SetPKIIssuer(pki.issuer)

	return nil
}

func selfIssueLeaf(ctx context.Context, pki *pkiState, nodeID int) error {
	// Re-read clusterID so the CSR's URI SAN matches.
	bundle := pki.bundleCached.Load()
	if bundle == nil {
		return fmt.Errorf("no bundle available for self-issue")
	}

	pki.agent.ClusterID = bundle.ClusterID

	keyPEM, csrPEM, err := ellapki.GenerateKeyAndCSR(nodeID, bundle.ClusterID)
	if err != nil {
		return err
	}

	csr, err := ellapki.ParseCSRPEM(csrPEM)
	if err != nil {
		return err
	}

	leafPEM, err := pki.issuer.Issue(ctx, csr, nodeID, ellapki.DefaultLeafTTL)
	if err != nil {
		return err
	}

	var bundlePEM []byte

	for _, r := range bundle.Roots {
		bundlePEM = append(bundlePEM, ellapki.EncodeCertPEM(r)...)
	}

	for _, i := range bundle.Intermediates {
		bundlePEM = append(bundlePEM, ellapki.EncodeCertPEM(i)...)
	}

	return pki.agent.StoreLeaf(leafPEM, keyPEM, bundlePEM)
}
