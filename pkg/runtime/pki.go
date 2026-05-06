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

// pkiState collects the runtime-wide PKI plumbing: the local
// per-node self-signed certificate (via Agent), the leader-side
// register/mint service (via Service, populated on leader
// promotion), and an in-memory pin map mirroring
// cluster_node_certs that the listener consults at handshake time.
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

func (p *pkiState) LeafFunc() listener.LeafFunc {
	return func() *tls.Certificate { return p.agent.Leaf() }
}

// PinFunc returns the listener accessor that resolves a peer's
// fingerprint against the cached pin map.
func (p *pkiState) PinFunc() listener.PinFunc {
	return func(fingerprint string) listener.PinResult {
		m := p.pins.Load()
		if m == nil {
			return listener.PinResult{}
		}

		nid, ok := (*m)[fingerprint]

		known := make([]int, 0, len(*m))
		for _, n := range *m {
			known = append(known, n)
		}

		return listener.PinResult{
			Found:        ok,
			NodeID:       nid,
			CacheSize:    len(*m),
			KnownNodeIDs: known,
		}
	}
}

// SeedPinsFromAgentDisk installs the disk-resident pin map (the
// local node's own pin plus the peer-pins.json snapshot saved by
// the most recent JoinFlow / register call) so the listener can
// verify peers during the startup window before the first
// RefreshPins reads the replicated table.
func (p *pkiState) SeedPinsFromAgentDisk() {
	m, err := p.agent.LoadPeerPins()
	if err != nil {
		logger.EllaLog.Warn("seed pins: load peer-pins.json", zap.Error(err))

		m = map[string]int{}
	}

	if leaf := p.agent.Leaf(); leaf != nil && leaf.Leaf != nil {
		m[pki.Fingerprint(leaf.Leaf)] = p.agent.NodeID
	}

	if len(m) == 0 {
		return
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

		return nil
	}

	added, removed := pinDelta(*prev, m)

	logger.EllaLog.Debug("cluster pin cache refreshed",
		zap.Int("size", len(m)),
		zap.Ints("added", added),
		zap.Ints("removed", removed))

	return nil
}

// pinDelta returns the nodeIDs that were added in next vs prev and
// the nodeIDs that were removed.
func pinDelta(prev, next map[string]int) (added, removed []int) {
	for fp, nid := range next {
		if _, ok := prev[fp]; !ok {
			added = append(added, nid)
		}
	}

	for fp, nid := range prev {
		if _, ok := next[fp]; !ok {
			removed = append(removed, nid)
		}
	}

	return added, removed
}

// pinRefreshInterval is a slow drift backstop. The hot path is the
// changefeed subscription in runPinSubscriber, which reacts the
// instant a cluster_node_certs apply lands; this tick only covers
// the unlikely case where the in-memory cache and the replicated
// table diverge for some other reason.
const pinRefreshInterval = 5 * time.Minute

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

// runPinSubscriber rebuilds the in-memory pin map every time the FSM
// applies a cluster_node_certs upsert or delete on this node. Because
// every node — leader and followers — applies replicated entries,
// every node's pin cache stays in lockstep with the replicated table
// without depending on HTTP-handler nudges or the slow tick.
func runPinSubscriber(ctx context.Context, pki *pkiState, dbInstance *db.Database) {
	wakeup, stop := dbInstance.Changefeed().Wakeup(db.TopicClusterNodeCerts)
	defer stop()

	if err := pki.RefreshPins(ctx, dbInstance); err != nil {
		logger.EllaLog.Warn("initial pin refresh failed", zap.Error(err))
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-wakeup:
			if err := pki.RefreshPins(ctx, dbInstance); err != nil {
				logger.EllaLog.Warn("pin refresh on changefeed wakeup failed", zap.Error(err))
			}
		}
	}
}

// rotateInterval is how often a node re-generates its self-signed
// cert and re-pins it. The previous pin remains valid until the
// new one commits, so a failed rotation is safe to retry.
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
