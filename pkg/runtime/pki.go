// Copyright 2026 Ella Networks

package runtime

import (
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

// pkiState collects the runtime-wide PKI plumbing so the rest of the
// bootstrap sequence can reach it by name.
type pkiState struct {
	agent      *pkiagent.Agent
	issuer     *pkiissuer.Service
	revocation *ellapki.RevocationCache

	// bundleCached caches the last-read trust bundle so per-handshake
	// accessor calls don't hit the DB on every hit. Refreshed lazily.
	bundleCached atomic.Pointer[ellapki.TrustBundle]

	// bundleRefreshMu serialises concurrent refreshes so only one DB
	// read is in flight at a time.
	bundleRefreshMu sync.Mutex
}

func newPKIState(nodeID int, clusterID, dataDir string) *pkiState {
	return &pkiState{
		agent:      pkiagent.NewAgent(nodeID, clusterID, dataDir),
		revocation: ellapki.NewRevocationCache(),
	}
}

// LeafFunc returns a listener.LeafFunc-compatible accessor.
func (p *pkiState) LeafFunc() func() *tls.Certificate {
	return func() *tls.Certificate { return p.agent.Leaf() }
}

// RevokedFunc returns a listener.RevokedFunc-compatible accessor.
func (p *pkiState) RevokedFunc() func(*big.Int) bool {
	return p.revocation.IsRevoked
}

// BundleFunc returns a listener.TrustBundleFunc-compatible accessor.
// Reads through a cached pointer; the cache is refreshed by
// RefreshBundle (called after leadership transitions and bootstrap).
func (p *pkiState) BundleFunc() func() *ellapki.TrustBundle {
	return func() *ellapki.TrustBundle { return p.bundleCached.Load() }
}

// RefreshBundle reads the current bundle from the DB and stores it in
// the cache. Safe to call concurrently.
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

	p.bundleCached.Store(b)

	return nil
}

// SeedBundleFromAgentDisk parses the bundle PEM the agent wrote during
// the join flow and installs it in the cache. Needed because the
// listener's trust accessor reads from the cache, but the DB-backed
// RefreshBundle only works after the joiner's Raft log has replicated
// the PKI state — and replication can't happen until the listener can
// mTLS-verify peers. The on-disk bundle carries the same roots and
// intermediates the leader handed us in the join response, so using it
// as the initial trust set is safe.
func (p *pkiState) SeedBundleFromAgentDisk(clusterID string) error {
	bundlePath := p.agent.BundlePath()

	pemBytes, err := os.ReadFile(bundlePath) // #nosec: G304 -- under dataDir
	if err != nil {
		return fmt.Errorf("read bundle %s: %w", bundlePath, err)
	}

	b := &ellapki.TrustBundle{ClusterID: clusterID}

	rest := pemBytes

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

		if bytesEqual(cert.RawIssuer, cert.RawSubject) {
			b.Roots = append(b.Roots, cert)
		} else {
			b.Intermediates = append(b.Intermediates, cert)
		}
	}

	if len(b.Roots) == 0 {
		return fmt.Errorf("bundle %s has no root certs", bundlePath)
	}

	p.bundleCached.Store(b)

	return nil
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
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

// maybeRestoreFromBundle checks for a restore.bundle file under dataDir.
// If present AND there is no existing ella.db, extracts the bundle in
// place and deletes the input bundle. Returns true if a restore
// happened.
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

	if err := db.ExtractForDR(bundlePath, dataDir); err != nil {
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
