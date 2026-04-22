// Copyright 2026 Ella Networks

package runtime

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ellanetworks/core/internal/cluster/pkiagent"
	"github.com/ellanetworks/core/internal/cluster/pkiissuer"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	ellapki "github.com/ellanetworks/core/internal/pki"
	"go.uber.org/zap"
)

// ErrNoTrustBundleYet signals that no PEM material was available at
// seed time; the runtime treats this as "populate later once raft has
// caught up," not as a hard failure.
var ErrNoTrustBundleYet = errors.New("no trust bundle material available yet")

// pkiState collects the runtime-wide PKI plumbing.
type pkiState struct {
	agent      *pkiagent.Agent
	issuer     *pkiissuer.Service
	revocation *ellapki.RevocationCache

	// bundleCached serves the listener's trust accessor from memory so
	// per-handshake calls don't hit the DB.
	bundleCached atomic.Pointer[ellapki.TrustBundle]

	// bundleRefreshMu serialises concurrent refreshes.
	bundleRefreshMu sync.Mutex
}

func newPKIState(nodeID int, clusterID, dataDir string) *pkiState {
	return &pkiState{
		agent:      pkiagent.NewAgent(nodeID, clusterID, dataDir),
		revocation: ellapki.NewRevocationCache(),
	}
}

func (p *pkiState) LeafFunc() func() *tls.Certificate {
	return func() *tls.Certificate { return p.agent.Leaf() }
}

func (p *pkiState) RevokedFunc() func(*big.Int) bool {
	return p.revocation.IsRevoked
}

// BundleFunc reads through bundleCached. RefreshBundle is responsible
// for keeping that cache current.
func (p *pkiState) BundleFunc() func() *ellapki.TrustBundle {
	return func() *ellapki.TrustBundle { return p.bundleCached.Load() }
}

// RefreshBundle reads the current bundle from the DB and stores it in
// the cache. Safe to call concurrently. On error — including the "no
// active roots yet" race during bootstrap — the previous cache entry
// is preserved.
func (p *pkiState) RefreshBundle(ctx context.Context) error {
	p.bundleRefreshMu.Lock()
	defer p.bundleRefreshMu.Unlock()

	if p.issuer == nil {
		return nil
	}

	b, err := p.issuer.CurrentBundle(ctx)
	if err != nil {
		return err
	}

	prev := p.bundleCached.Load()
	p.bundleCached.Store(b)

	// One-shot breadcrumb when the DB-backed bundle first becomes
	// usable. If this line is absent from a node serving cluster
	// traffic, the listener is running on a seed-only bundle.
	if prev == nil || len(prev.Roots) == 0 {
		logger.EllaLog.Info("cluster PKI trust bundle populated from DB",
			zap.Int("roots", len(b.Roots)),
			zap.Int("intermediates", len(b.Intermediates)))
	}

	return nil
}

// refreshFollowerBundleWithRetry polls RefreshBundle until it succeeds
// or timeout elapses. Covers the window where a joiner's
// WaitForInitialization returns on the operator row before the
// pki_roots rows have replicated.
func refreshFollowerBundleWithRetry(ctx context.Context, pki *pkiState, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	const interval = 200 * time.Millisecond

	var lastErr error

	for {
		lastErr = pki.RefreshBundle(ctx)
		if lastErr == nil {
			return nil
		}

		if time.Now().After(deadline) {
			return lastErr
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
}

// seedBundleFromPEM parses bundlePEM (concatenated CERTIFICATE blocks)
// into a TrustBundle and installs it in the cache. Returns
// ErrNoTrustBundleYet if bundlePEM is empty — the caller decides
// whether that's fatal or tolerable.
func (p *pkiState) seedBundleFromPEM(clusterID string, bundlePEM []byte) error {
	if len(bundlePEM) == 0 {
		return ErrNoTrustBundleYet
	}

	b := &ellapki.TrustBundle{ClusterID: clusterID}

	rest := bundlePEM

	for {
		var block *pem.Block

		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}

		if block.Type != "CERTIFICATE" {
			continue
		}

		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return fmt.Errorf("parse bundle cert: %w", err)
		}

		if bytes.Equal(cert.RawIssuer, cert.RawSubject) {
			b.Roots = append(b.Roots, cert)
		} else {
			b.Intermediates = append(b.Intermediates, cert)
		}
	}

	if len(b.Roots) == 0 {
		return fmt.Errorf("bundle has no root certs")
	}

	p.bundleCached.Store(b)

	return nil
}

// RefreshRevocations rebuilds the revocation cache from the DB.
func (p *pkiState) RefreshRevocations(ctx context.Context, dbInstance *db.Database) error {
	rows, err := dbInstance.ListRevokedCerts(ctx)
	if err != nil {
		return err
	}

	serials := make([]uint64, 0, len(rows))
	for _, r := range rows {
		if r.Serial >= 0 {
			serials = append(serials, uint64(r.Serial))
		}
	}

	p.revocation.Replace(serials)

	return nil
}

// revocationRefreshInterval is the upper bound on how long the in-memory
// revocation cache can lag the replicated state. Revocations are rare
// (only on cluster-member removal), so polling is cheap and the bound
// does not need to be tight.
const revocationRefreshInterval = 30 * time.Second

// runRevocationRefresher periodically rebuilds the revocation cache from
// the DB so followers pick up new revocations applied via Raft. Blocks
// until ctx is cancelled.
func runRevocationRefresher(ctx context.Context, pki *pkiState, dbInstance *db.Database) {
	t := time.NewTicker(revocationRefreshInterval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := pki.RefreshRevocations(ctx, dbInstance); err != nil {
				logger.EllaLog.Warn("periodic revocation refresh failed", zap.Error(err))
			}
		}
	}
}

// maybeRestoreFromBundle extracts restore.bundle under dataDir when
// present and ella.db does not yet exist. Returns true if a restore
// happened. No-op otherwise.
func maybeRestoreFromBundle(dataDir string) (bool, error) {
	bundlePath := filepath.Join(dataDir, "restore.bundle")

	if _, err := os.Stat(bundlePath); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}

		return false, fmt.Errorf("stat restore.bundle: %w", err)
	}

	dbPath := filepath.Join(dataDir, db.DBFilename)
	if _, err := os.Stat(dbPath); err == nil {
		logger.EllaLog.Warn("ella.db already exists; ignoring restore.bundle",
			zap.String("bundle", bundlePath))

		return false, nil
	}

	if err := db.ExtractForRestore(bundlePath, dataDir); err != nil {
		return false, fmt.Errorf("extract restore bundle: %w", err)
	}

	if err := os.Remove(bundlePath); err != nil {
		logger.EllaLog.Warn("failed to remove restore bundle after extract",
			zap.String("bundle", bundlePath), zap.Error(err))
	}

	logger.EllaLog.Info("restored cluster state from bundle",
		zap.String("bundle", bundlePath))

	return true, nil
}

// runJoinFlow dials each peer in turn presenting token until one
// accepts and issues a leaf. No-op if token is empty.
func runJoinFlow(ctx context.Context, agent *pkiagent.Agent, peers []string, token string) error {
	if token == "" {
		return nil
	}

	if len(peers) == 0 {
		return errors.New("cluster.join-token is set but cluster.peers is empty")
	}

	var lastErr error

	for _, addr := range peers {
		joinCtx, cancel := context.WithTimeout(ctx, 10*time.Second)

		if err := agent.JoinFlow(joinCtx, addr, token); err != nil {
			lastErr = fmt.Errorf("peer %s: %w", addr, err)

			cancel()

			continue
		}

		cancel()

		logger.EllaLog.Info("join flow completed; leaf installed",
			zap.String("peer", addr))

		return nil
	}

	return fmt.Errorf("all peers rejected the join: %w", lastErr)
}
