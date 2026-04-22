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

	bootstrapRegistered sync.Once
}

func newPKILeaderCallback(ctx context.Context, state *pkiState, dbInstance *db.Database, ln *listener.Listener, nodeID int) *pkiLeaderCallback {
	return &pkiLeaderCallback{
		ctx:        ctx,
		state:      state,
		dbInstance: dbInstance,
		clusterLn:  ln,
		nodeID:     nodeID,
	}
}

// OnBecameLeader loads or bootstraps PKI material on this node and
// publishes the issuer so HTTP handlers can reach it. Best-effort: a
// failure logs but does not panic; issuance requests return 503 until
// the next leadership transition retries.
func (c *pkiLeaderCallback) OnBecameLeader() {
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

// setupLeaderPKI runs on the leader after PostInitClusterSetup. It
// bootstraps the issuer (first leader) or loads keys (subsequent
// elections), issues a leaf for this node if we don't have one yet, and
// publishes the issuer + revocation cache so HTTP handlers can reach
// them.
func setupLeaderPKI(ctx context.Context, pki *pkiState, dbInstance *db.Database, nodeID int) error {
	if pki.issuer == nil {
		pki.issuer = pkiissuer.New(dbInstance)
	}

	// First-leader bootstrap (idempotent).
	if err := pki.issuer.Bootstrap(ctx); err != nil {
		return fmt.Errorf("issuer bootstrap: %w", err)
	}

	// Reload keys from the replicated DB every time leadership is
	// acquired. A non-founding voter may only now have the keys via raft
	// replication; a voter that was promoted before replication
	// completed will leave LoadKeys in a degraded state and recover
	// at the next election after the worker finishes. LoadKeys is
	// non-fatal on missing files so Ready() stays false without
	// crashing — Issue / MintJoinToken will return 503 until a
	// subsequent promotion succeeds.
	if err := pki.issuer.LoadKeys(ctx); err != nil {
		return fmt.Errorf("issuer load keys: %w", err)
	}

	if err := pki.RefreshBundle(ctx); err != nil {
		return fmt.Errorf("refresh bundle: %w", err)
	}

	if err := pki.RefreshRevocations(ctx, dbInstance); err != nil {
		logger.EllaLog.Warn("refresh revocations", zap.Error(err))
	}

	// If this node has no leaf yet (first-boot-first-leader) AND we
	// are able to issue (Ready), self-issue one. If keys are missing
	// we can't self-issue; the key-transfer worker will keep trying
	// and subsequent leadership events will re-run this path.
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
