// Copyright 2026 Ella Networks

package runtime

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/ellanetworks/core/internal/cluster/listener"
	"github.com/ellanetworks/core/internal/cluster/pkiagent"
	"github.com/ellanetworks/core/internal/cluster/pkiissuer"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/pki"
	"go.uber.org/zap"
)

// pkiState collects the runtime-wide PKI plumbing.
//
// The cluster TLS transport is a fingerprint-pinning system rather
// than a CA hierarchy: each node owns a long-lived self-signed cert
// and trust is established by the replicated cluster_node_certs
// table. The runtime caches that table in memory (pins) and refreshes
// it on a tick so handshakes do not hit SQLite per dial.
type pkiState struct {
	agent  *pkiagent.Agent
	issuer *pkiissuer.Service

	pins atomic.Pointer[map[string]int]
}

func newPKIState(nodeID int, clusterID, dataDir string) *pkiState {
	return &pkiState{
		agent: pkiagent.NewAgent(nodeID, clusterID, dataDir),
	}
}

// LeafFunc returns the listener-compatible accessor for this node's
// self-signed cert.
func (p *pkiState) LeafFunc() listener.LeafFunc {
	return func() *tls.Certificate { return p.agent.Leaf() }
}

// PinFunc returns the listener-compatible accessor that resolves a
// peer's fingerprint via the cached pin map.
func (p *pkiState) PinFunc() listener.PinFunc {
	return func(fingerprint string) listener.PinResult {
		m := p.pins.Load()
		if m == nil {
			return listener.PinResult{}
		}

		nid, ok := (*m)[fingerprint]

		return listener.PinResult{Found: ok, NodeID: nid}
	}
}

// SeedPinsFromAgentDisk installs an early pin map containing only
// the local cert. Used at startup so the listener has at least its
// own pin to check before Raft replication of cluster_node_certs has
// caught up. RefreshPins replaces the cache with the full table once
// available.
func (p *pkiState) SeedPinsFromAgentDisk() {
	leaf := p.agent.Leaf()
	if leaf == nil || leaf.Leaf == nil {
		return
	}

	m := map[string]int{
		pki.Fingerprint(leaf.Leaf): p.agent.NodeID,
	}

	p.pins.Store(&m)
}

// RefreshPins reads cluster_node_certs and replaces the cache.
func (p *pkiState) RefreshPins(ctx context.Context, dbInstance *db.Database) error {
	rows, err := dbInstance.ListClusterNodeCerts(ctx)
	if err != nil {
		return err
	}

	m := make(map[string]int, len(rows))
	for _, r := range rows {
		m[r.Fingerprint] = r.NodeID
	}

	prev := p.pins.Load()

	p.pins.Store(&m)

	if prev == nil || len(*prev) == 0 {
		logger.EllaLog.Info("cluster pin cache populated from DB",
			zap.Int("pins", len(m)))
	}

	return nil
}

// pinRefreshInterval bounds how stale the in-memory cache can be.
// Pin updates are rare (member add/remove or rotation), so polling
// is cheap.
const pinRefreshInterval = 30 * time.Second

func runPinRefresher(ctx context.Context, pki *pkiState, dbInstance *db.Database) {
	t := time.NewTicker(pinRefreshInterval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := pki.RefreshPins(ctx, dbInstance); err != nil {
				logger.EllaLog.Warn("periodic pin refresh failed", zap.Error(err))
			}
		}
	}
}

// rotateInterval is how often a node optionally re-generates its
// self-signed cert and re-pins it. Failure is harmless: the existing
// pin remains valid until the new one commits, so the cluster
// liveness path does not depend on rotation succeeding.
const rotateInterval = 90 * 24 * time.Hour

func runRotator(ctx context.Context, p *pkiState, ln *listener.Listener, dbInstance *db.Database) {
	t := time.NewTicker(rotateInterval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			leaderAddr, leaderID := dbInstance.LeaderAddressAndID()
			if leaderAddr == "" || leaderID == 0 {
				logger.EllaLog.Info("skip rotation: no leader yet")
				continue
			}

			rotCtx, cancel := context.WithTimeout(ctx, 30*time.Second)

			if err := p.agent.Rotate(rotCtx, ln, leaderAddr, leaderID); err != nil {
				logger.EllaLog.Warn("cluster cert rotation failed; existing pin remains valid",
					zap.Error(err))
			} else {
				logger.EllaLog.Info("cluster cert rotated successfully")
			}

			cancel()
		}
	}
}

// maybeRestoreFromBundle extracts restore.bundle under dataDir when
// present and ella.db does not yet exist.
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

// runJoinFlow dials each peer presenting token until one accepts and
// registers this node's cert. No-op if token is empty.
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

		logger.EllaLog.Info("join flow completed; cert registered",
			zap.String("peer", addr))

		return nil
	}

	return fmt.Errorf("all peers rejected the join: %w", lastErr)
}

// refreshFollowerPinsWithRetry polls RefreshPins until it returns at
// least one pin or timeout elapses. Covers the window where a
// joiner's WaitForInitialization returns on the operator row before
// cluster_node_certs has replicated.
func refreshFollowerPinsWithRetry(ctx context.Context, p *pkiState, dbInstance *db.Database, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	const interval = 200 * time.Millisecond

	var lastErr error

	for {
		lastErr = p.RefreshPins(ctx, dbInstance)
		if lastErr == nil {
			m := p.pins.Load()
			if m != nil && len(*m) > 0 {
				return nil
			}
		}

		if time.Now().After(deadline) {
			if lastErr != nil {
				return lastErr
			}

			return fmt.Errorf("no pins replicated within %s", timeout)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
}

// _ keeps pkiissuer in the import set without a build-time hit even
// when the only direct use site is pki_leader.go.
var _ = pkiissuer.New
