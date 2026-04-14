// Copyright 2026 Ella Networks

package raft

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb/v2"
	"go.uber.org/zap"
)

// ClusterConfig holds the cluster-related configuration parsed from YAML.
type ClusterConfig struct {
	Enabled             bool
	NodeID              int
	BindAddress         string
	AdvertiseAPIAddress string
	BootstrapExpect     int
	Peers               []string
	JoinToken           string
	JoinTimeout         time.Duration
	ProposeTimeout      time.Duration
	SnapshotInterval    time.Duration
	SnapshotThreshold   uint64

	// PerformanceMultiplier scales heartbeat/election/leader-lease timeouts
	// in HA mode. Default 5 matches Vault's integrated storage, trading a
	// slower election for tolerance of real-network jitter. Ignored in
	// single-server mode, which uses fixed fast timeouts.
	PerformanceMultiplier int

	// TrailingLogs bounds the number of Raft log entries retained after a
	// snapshot. Lower values shrink BoltDB at the cost of forcing full
	// snapshots to followers that lag. Zero keeps the hashicorp/raft
	// default (10240).
	TrailingLogs uint64
}

const (
	// defaultPerformanceMultiplier is the per-operator scaling factor
	// applied to the library's default timeouts when running in HA mode.
	// Matches Vault.
	defaultPerformanceMultiplier = 5

	// initialTimeoutMultiplier slows down heartbeat/election once on first
	// boot so a newly joined HA node doesn't contest leadership before the
	// cluster has stabilised. Also from Vault.
	initialTimeoutMultiplier = 3

	// standaloneHeartbeatTimeout and friends govern the single-server bootstrap
	// path. Aggressive values are safe because there are no peers to time
	// out against — the timeout is a ceiling, not a floor, and the real
	// election completes in microseconds over the loopback listener.
	standaloneHeartbeatTimeout   = 50 * time.Millisecond
	standaloneElectionTimeout    = 50 * time.Millisecond
	standaloneLeaderLeaseTimeout = 50 * time.Millisecond
	standaloneCommitTimeout      = 5 * time.Millisecond
)

// closeTransport best-effort closes a raft.Transport. The interface itself
// has no Close method, but concrete transports (TCP, in-mem) implement
// io.Closer. Used on error paths in NewManager and Shutdown.
func closeTransport(t raft.Transport) {
	if c, ok := t.(io.Closer); ok {
		_ = c.Close()
	}
}

// Manager wraps a hashicorp/raft instance and its supporting infrastructure.
type Manager struct {
	raft        *raft.Raft
	fsm         *FSM
	transport   raft.Transport
	logStore    raft.LogStore
	stableStore raft.StableStore
	snaps       raft.SnapshotStore
	config      ClusterConfig
	idCounters  *IDCounters
	nodeID      int
	dataDir     string
}

// defaultStandaloneBindAddress is the bind address used when ClusterConfig
// leaves BindAddress empty. Port 0 asks the kernel for an ephemeral port, so
// standalone processes (and concurrent test processes) never compete for a
// fixed port. The actual bound address is surfaced via transport.LocalAddr()
// and used as the sole entry in the single-server bootstrap configuration.
const defaultStandaloneBindAddress = "127.0.0.1:0"

// NewManager creates and starts a Raft node over a real TCP transport. The
// applier is called by the FSM for every committed log entry.
//
// When cfg.Enabled is false, the manager runs as a single-server cluster:
// fast timeouts, auto-bootstrap on fresh state, synchronous wait for
// self-election, and synchronous ID-counter seeding. This is the shipping
// standalone mode. Tests that want an in-memory transport should use
// NewTestManager instead.
//
// When cfg.Enabled is true, the manager runs in HA mode: library default
// timeouts, no auto-bootstrap (operators call Bootstrap on the designated
// node), and async leadership-change handling that re-seeds ID counters on
// promotion.
func NewManager(ctx context.Context, cfg ClusterConfig, applier Applier, dataDir string, opts ...ManagerOption) (*Manager, error) {
	options := managerOptions{transportFactory: tcpTransportFactory}
	for _, opt := range opts {
		opt(&options)
	}

	singleServer := !cfg.Enabled

	nodeID, err := resolveNodeIDForMode(cfg, singleServer, dataDir)
	if err != nil {
		return nil, err
	}

	raftDir := filepath.Join(dataDir, "raft")
	if err := os.MkdirAll(raftDir, 0o700); err != nil {
		return nil, fmt.Errorf("create raft directory: %w", err)
	}

	fsm := NewFSM(applier, dataDir)
	idCounters := NewIDCounters()

	raftConfig := raft.DefaultConfig()
	raftConfig.LocalID = raft.ServerID(fmt.Sprintf("%d", nodeID))
	raftConfig.Logger = newZapRaftLogger()

	if cfg.SnapshotInterval > 0 {
		raftConfig.SnapshotInterval = cfg.SnapshotInterval
	}

	if cfg.SnapshotThreshold > 0 {
		raftConfig.SnapshotThreshold = cfg.SnapshotThreshold
	}

	if cfg.TrailingLogs > 0 {
		raftConfig.TrailingLogs = cfg.TrailingLogs
	}

	boltPath := filepath.Join(raftDir, "raft.db")

	boltStore, err := raftboltdb.NewBoltStore(boltPath)
	if err != nil {
		return nil, fmt.Errorf("create bolt store at %s: %w", boltPath, err)
	}

	snapshotStore, err := raft.NewFileSnapshotStore(raftDir, 3, newZapIOWriter("snapshot"))
	if err != nil {
		_ = boltStore.Close()
		return nil, fmt.Errorf("create snapshot store: %w", err)
	}

	logCache, err := raft.NewLogCache(raftLogCacheSize, boltStore)
	if err != nil {
		_ = boltStore.Close()
		return nil, fmt.Errorf("create log cache: %w", err)
	}

	transport, err := options.transportFactory(cfg)
	if err != nil {
		_ = boltStore.Close()
		return nil, err
	}

	// HasExistingState must run before NewRaft, which may write an initial
	// term. Bootstrap detection (below) keys off this pre-state.
	hasState, err := raft.HasExistingState(logCache, boltStore, snapshotStore)
	if err != nil {
		closeTransport(transport)

		_ = boltStore.Close()

		return nil, fmt.Errorf("check existing raft state: %w", err)
	}

	recovered, err := maybeRecoverCluster(raftDir, raftConfig, fsm, logCache, boltStore, snapshotStore, transport)
	if err != nil {
		closeTransport(transport)

		_ = boltStore.Close()

		return nil, err
	}

	// Timeouts depend on (hasState || recovered) — only apply once both are
	// known, but before NewRaft spins up the internal loop that consumes them.
	// RecoverCluster above only uses LocalID from raftConfig, so the order is
	// safe.
	applyTimeouts(raftConfig, cfg, singleServer, !hasState && !recovered)

	r, err := raft.NewRaft(raftConfig, fsm, logCache, boltStore, snapshotStore, transport)
	if err != nil {
		closeTransport(transport)

		_ = boltStore.Close()

		return nil, fmt.Errorf("create raft instance: %w", err)
	}

	if singleServer && !hasState && !recovered {
		bootCfg := raft.Configuration{
			Servers: []raft.Server{{
				ID:      raftConfig.LocalID,
				Address: transport.LocalAddr(),
			}},
		}

		if err := r.BootstrapCluster(bootCfg).Error(); err != nil {
			_ = r.Shutdown().Error()

			closeTransport(transport)

			_ = boltStore.Close()

			return nil, fmt.Errorf("bootstrap standalone raft: %w", err)
		}
	}

	m := &Manager{
		raft:        r,
		fsm:         fsm,
		transport:   transport,
		logStore:    boltStore,
		stableStore: boltStore,
		snaps:       snapshotStore,
		config:      cfg,
		idCounters:  idCounters,
		nodeID:      nodeID,
		dataDir:     dataDir,
	}

	if singleServer {
		if err := m.waitForLeader(ctx); err != nil {
			_ = r.Shutdown().Error()

			closeTransport(transport)

			_ = boltStore.Close()

			return nil, err
		}

		if err := idCounters.SeedFromDB(ctx, applier.SharedPlainDB()); err != nil {
			_ = r.Shutdown().Error()

			closeTransport(transport)

			_ = boltStore.Close()

			return nil, fmt.Errorf("seed ID counters: %w", err)
		}

		return m, nil
	}

	go m.watchLeadership(ctx, applier)

	return m, nil
}

// resolveNodeIDForMode picks the Raft server ID. In single-server mode the
// node is alone in its configuration, so ID 1 is sufficient and doesn't
// require operators to provision cluster.node-id on every standalone install.
// HA mode goes through ResolveNodeID, which enforces config/env/file
// consistency and rejects mismatches that would invalidate issued GUTIs.
func resolveNodeIDForMode(cfg ClusterConfig, singleServer bool, dataDir string) (int, error) {
	if singleServer && cfg.NodeID == 0 {
		return 1, nil
	}

	id, err := ResolveNodeID(cfg.NodeID, dataDir)
	if err != nil {
		return 0, fmt.Errorf("resolve node ID: %w", err)
	}

	return id, nil
}

// watchLeadership monitors raft.LeaderCh() and seeds ID counters on promotion.
func (m *Manager) watchLeadership(ctx context.Context, applier Applier) {
	for {
		select {
		case <-ctx.Done():
			return
		case isLeader := <-m.raft.LeaderCh():
			if isLeader {
				logger.RaftLog.Info("Raft: this node is now the leader",
					zap.Int("nodeID", m.nodeID))

				if err := m.idCounters.SeedFromDB(ctx, applier.SharedPlainDB()); err != nil {
					logger.RaftLog.Error("Raft: failed to seed ID counters on leader promotion",
						zap.Error(err))
				}
			} else {
				logger.RaftLog.Info("Raft: this node lost leadership",
					zap.Int("nodeID", m.nodeID))
			}
		}
	}
}

// applyTimeouts configures heartbeat / election / leader-lease / commit
// timeouts based on whether the manager is single-server or HA, and whether
// this is a fresh boot with no prior state.
//
// Single-server mode uses fixed 50 ms timeouts: with no peers to negotiate
// with the library defaults are pure dead time during bootstrap.
//
// HA mode scales the library defaults by PerformanceMultiplier (default 5,
// matching Vault) to tolerate real-network jitter. On a fresh boot the
// heartbeat and election timeouts are further multiplied by
// initialTimeoutMultiplier so a slow first election on a newly joined node
// doesn't trigger spurious leadership contests before the cluster stabilises.
func applyTimeouts(rc *raft.Config, cfg ClusterConfig, singleServer, freshBoot bool) {
	if singleServer {
		rc.HeartbeatTimeout = standaloneHeartbeatTimeout
		rc.ElectionTimeout = standaloneElectionTimeout
		rc.LeaderLeaseTimeout = standaloneLeaderLeaseTimeout
		rc.CommitTimeout = standaloneCommitTimeout

		return
	}

	multiplier := cfg.PerformanceMultiplier
	if multiplier <= 0 {
		multiplier = defaultPerformanceMultiplier
	}

	rc.HeartbeatTimeout *= time.Duration(multiplier)
	rc.ElectionTimeout *= time.Duration(multiplier)
	rc.LeaderLeaseTimeout *= time.Duration(multiplier)

	if freshBoot {
		rc.HeartbeatTimeout *= initialTimeoutMultiplier
		rc.ElectionTimeout *= initialTimeoutMultiplier
	}
}

// Propose serializes a command and applies it through Raft consensus.
// Only the leader can propose; followers receive ErrNotLeader.
func (m *Manager) Propose(cmd *Command, timeout time.Duration) (any, error) {
	data, err := cmd.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("marshal command: %w", err)
	}

	future := m.raft.Apply(data, timeout)
	if err := future.Error(); err != nil {
		return nil, err
	}

	resp := future.Response()

	// If the FSM returned an error, propagate it.
	if err, ok := resp.(error); ok {
		return nil, err
	}

	return resp, nil
}

// IsLeader returns true if this node is the current Raft leader.
func (m *Manager) IsLeader() bool {
	return m.raft.State() == raft.Leader
}

// LeaderAddress returns the Raft transport address of the current leader.
// Returns empty string if there is no leader.
func (m *Manager) LeaderAddress() string {
	addr, _ := m.raft.LeaderWithID()
	return string(addr)
}

// NodeID returns this node's cluster ID.
func (m *Manager) NodeID() int {
	return m.nodeID
}

// IDCounters returns the leader ID counters for deterministic ID assignment.
func (m *Manager) IDCounters() *IDCounters {
	return m.idCounters
}

// AppliedIndex returns the last applied Raft log index.
func (m *Manager) AppliedIndex() uint64 {
	return m.fsm.AppliedIndex()
}

// State returns the current Raft state (Leader, Follower, Candidate, Shutdown).
func (m *Manager) State() raft.RaftState {
	return m.raft.State()
}

// Shutdown gracefully shuts down the Raft node.
func (m *Manager) Shutdown() error {
	future := m.raft.Shutdown()
	if err := future.Error(); err != nil {
		return fmt.Errorf("raft shutdown: %w", err)
	}

	if closer, ok := m.logStore.(interface{ Close() error }); ok {
		if err := closer.Close(); err != nil {
			return fmt.Errorf("close log store: %w", err)
		}
	}

	if tc, ok := m.transport.(io.Closer); ok {
		if err := tc.Close(); err != nil {
			return fmt.Errorf("close transport: %w", err)
		}
	}

	return nil
}

// Bootstrap bootstraps a new single-node cluster. Only called once during
// initial cluster formation.
func (m *Manager) Bootstrap() error {
	cfg := raft.Configuration{
		Servers: []raft.Server{
			{
				ID:      raft.ServerID(fmt.Sprintf("%d", m.nodeID)),
				Address: raft.ServerAddress(m.config.BindAddress),
			},
		},
	}

	future := m.raft.BootstrapCluster(cfg)

	return future.Error()
}

// waitForLeader blocks until this node becomes the Raft leader or ctx is cancelled.
func (m *Manager) waitForLeader(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case isLeader := <-m.raft.LeaderCh():
			if isLeader {
				return nil
			}
		}
	}
}
