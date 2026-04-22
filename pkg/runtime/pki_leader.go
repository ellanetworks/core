// Copyright 2026 Ella Networks

package runtime

import (
	"context"
	"fmt"
	"sync"

	"github.com/ellanetworks/core/internal/api/server"
	"github.com/ellanetworks/core/internal/cluster/listener"
	"github.com/ellanetworks/core/internal/cluster/pkiissuer"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	ellapki "github.com/ellanetworks/core/internal/pki"
	"go.uber.org/zap"
)

// pkiLeaderCallback runs setupLeaderPKI each time this node becomes the
// Raft leader, and unloads in-memory keys when leadership is lost.
// Implements raft.LeaderCallback via duck-typing.
type pkiLeaderCallback struct {
	ctx        context.Context
	state      *pkiState
	dbInstance *db.Database
	clusterLn  *listener.Listener
	nodeID     int

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
}

func newPKILeaderCallback(ctx context.Context, state *pkiState, dbInstance *db.Database, ln *listener.Listener, nodeID int, needsDRSnapshot bool) *pkiLeaderCallback {
	return &pkiLeaderCallback{
		ctx:             ctx,
		state:           state,
		dbInstance:      dbInstance,
		clusterLn:       ln,
		nodeID:          nodeID,
		needsDRSnapshot: needsDRSnapshot,
	}
}

// OnBecameLeader loads or bootstraps PKI material on this node and
// publishes the issuer so HTTP handlers can reach it. Best-effort: a
// failure logs but does not panic; issuance requests return 503 until
// the next leadership transition retries.
func (c *pkiLeaderCallback) OnBecameLeader() {
	if c.needsDRSnapshot {
		if err := c.dbInstance.SelfRestore(c.ctx); err != nil {
			logger.EllaLog.Warn("post-DR self-restore failed", zap.Error(err))
			// Skip the rest for this transition; the next leader
			// callback fires a retry.
			return
		}

		c.needsDRSnapshot = false
	}

	if err := setupLeaderPKI(c.ctx, c.state, c.dbInstance, c.nodeID); err != nil {
		logger.EllaLog.Warn("setupLeaderPKI on leader transition failed", zap.Error(err))
		return
	}

	// Register the bootstrap ALPN the first time setup succeeds.
	// listener.Register panics on duplicate ALPN, so we guard with a
	// once that only fires after a successful setupLeaderPKI.
	if c.clusterLn != nil && c.state.issuer != nil {
		c.bootstrapRegistered.Do(func() {
			server.RegisterBootstrapALPN(c.clusterLn, c.state.issuer)
		})
	}
}

// OnLostLeadership zeroes the in-memory keys. The issuer stays
// registered with the HTTP handlers so follower-side bundle accessors
// still work, but Issue / MintJoinToken return "not leader" until we
// regain leadership.
func (c *pkiLeaderCallback) OnLostLeadership() {
	if c.state.issuer != nil {
		c.state.issuer.UnloadKeys()
	}
}

// setupLeaderPKI runs after PostInitClusterSetup. It bootstraps the
// issuer on the first leader, loads signing keys from the replicated
// DB on every leadership transition, self-issues a leaf if this node
// doesn't have one, and publishes the issuer so HTTP handlers can
// reach it.
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

// selfIssueLeaf generates a local CSR, has the issuer sign it, and
// stores the result via the agent.
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
