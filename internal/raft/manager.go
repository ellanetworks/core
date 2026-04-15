// Copyright 2026 Ella Networks

package raft

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
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

	// SchemaVersion is the shared-DB migration version this binary expects.
	// Included in the join handshake so version-skewed nodes are rejected.
	SchemaVersion int

	// InitialSuffrage controls whether this node joins as "voter" or
	// "nonvoter". Set to "nonvoter" during rolling upgrade re-joins.
	InitialSuffrage string
}

const (
	// defaultPerformanceMultiplier is the per-operator scaling factor
	// applied to the library's default timeouts when running in HA mode.
	// Matches Vault.
	defaultPerformanceMultiplier = 5

	// standaloneHeartbeatTimeout and friends govern the single-server bootstrap
	// path. Aggressive values are safe because there are no peers to time
	// out against — the timeout is a ceiling, not a floor, and the real
	// election completes in microseconds over the loopback listener.
	standaloneHeartbeatTimeout   = 50 * time.Millisecond
	standaloneElectionTimeout    = 50 * time.Millisecond
	standaloneLeaderLeaseTimeout = 50 * time.Millisecond
	standaloneCommitTimeout      = 5 * time.Millisecond

	// defaultProposeTimeout caps how long a write waits for Raft commit
	// before the API layer returns 503. 5 s is generous for single-server
	// (commit is microseconds) and a reasonable default for HA with healthy
	// replication; operators tune via ClusterConfig.ProposeTimeout.
	defaultProposeTimeout = 5 * time.Second
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
	raft            *raft.Raft
	fsm             *FSM
	transport       raft.Transport
	logStore        raft.LogStore
	snaps           raft.SnapshotStore
	config          ClusterConfig
	nodeID          int
	dataDir         string
	observer        *LeaderObserver
	needsDiscovery  bool
	autopilot       *autopilotRunner
	followerTracker *followerTracker
	boltNoSync      bool
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
// fast timeouts, auto-bootstrap on fresh state, and a synchronous wait for
// self-election. This is the shipping standalone mode. Tests that want an
// in-memory transport should use NewTestManager instead.
//
// When cfg.Enabled is true, the manager runs in HA mode: library default
// timeouts scaled by PerformanceMultiplier, and no auto-bootstrap (operators
// drive cluster formation explicitly).
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

	// In single-server mode the raft log is auxiliary: ella.db (fsynced on
	// COMMIT) is the canonical FSM state, there are no peers to replicate to,
	// and losing trailing raft-log entries on crash is harmless — the FSM
	// state on disk is already authoritative. Skipping per-entry fsync halves
	// write latency (one fsync per COMMIT instead of two) and restores the
	// pre-HA throughput the API layer depends on for batched mutations.
	// HA mode keeps fsync enabled: replicas derive truth from the log.
	boltStore, err := raftboltdb.New(raftboltdb.Options{
		Path:   boltPath,
		NoSync: singleServer,
	})
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

	observer := NewLeaderObserver()

	m := &Manager{
		raft:           r,
		fsm:            fsm,
		transport:      transport,
		logStore:       boltStore,
		snaps:          snapshotStore,
		config:         cfg,
		nodeID:         nodeID,
		dataDir:        dataDir,
		observer:       observer,
		needsDiscovery: !singleServer && !hasState && !recovered,
		boltNoSync:     singleServer,
	}

	if !singleServer {
		ft := newFollowerTracker(r)
		m.followerTracker = ft
		m.autopilot = newAutopilotRunner(r, m)

		observer.Register(ft.asLeaderCallback(raft.ServerID(strconv.Itoa(nodeID))))
		observer.Register(m.autopilot)
	}

	if singleServer {
		if err := m.waitForLeader(ctx); err != nil {
			_ = r.Shutdown().Error()

			closeTransport(transport)

			_ = boltStore.Close()

			return nil, err
		}
	}

	go observer.Run(r)
	go runMetricsLoop(r, observer.stopCh)

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
	if singleServer || freshBoot {
		// Single-server and fresh HA nodes both start with fast timeouts.
		// For single-server the timeouts are permanent. For fresh HA nodes
		// they allow the bootstrapper to self-elect in milliseconds;
		// restoreHATimeouts upgrades HeartbeatTimeout and ElectionTimeout
		// to HA values after the cluster forms. LeaderLeaseTimeout is not
		// runtime-reloadable so it stays at the standalone value, which is
		// always smaller than the HA HeartbeatTimeout.
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
}

// restoreHATimeouts reloads the steady-state HA timeouts (performance
// multiplier only, no freshBoot inflation). Called after the bootstrapper's
// fast self-election completes so that subsequent elections with real peers
// use properly scaled timeouts.
func (m *Manager) restoreHATimeouts() {
	multiplier := m.config.PerformanceMultiplier
	if multiplier <= 0 {
		multiplier = defaultPerformanceMultiplier
	}

	base := raft.DefaultConfig()

	rc := m.raft.ReloadableConfig()
	rc.HeartbeatTimeout = base.HeartbeatTimeout * time.Duration(multiplier)
	rc.ElectionTimeout = base.ElectionTimeout * time.Duration(multiplier)

	if err := m.raft.ReloadConfig(rc); err != nil {
		logger.RaftLog.Warn("Failed to restore HA timeouts", zap.Error(err))
	}
}

// Propose serializes a command and applies it through Raft consensus.
// Only the leader can propose; followers receive ErrNotLeader.
// ProposeResult holds the FSM response and the raft log index.
type ProposeResult struct {
	Value any
	Index uint64
}

func (m *Manager) Propose(cmd *Command, timeout time.Duration) (*ProposeResult, error) {
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

	return &ProposeResult{Value: resp, Index: future.Index()}, nil
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

// RaftAddress returns the transport-local Raft address this node is reachable
// at (post-bind, so includes any ephemeral port assigned by the kernel).
func (m *Manager) RaftAddress() string {
	return string(m.transport.LocalAddr())
}

// APIAddress returns the advertised API address for this node.
func (m *Manager) APIAddress() string {
	return m.config.AdvertiseAPIAddress
}

// ProposeTimeout returns the configured maximum wait for a Raft commit, or
// defaultProposeTimeout when ClusterConfig left it unset.
func (m *Manager) ProposeTimeout() time.Duration {
	if m.config.ProposeTimeout > 0 {
		return m.config.ProposeTimeout
	}

	return defaultProposeTimeout
}

// AppliedIndex returns the last applied Raft log index.
func (m *Manager) AppliedIndex() uint64 {
	return m.fsm.AppliedIndex()
}

// Barrier blocks until all preceding log entries are applied to the FSM,
// ensuring subsequent reads reflect every committed write.
func (m *Manager) Barrier(timeout time.Duration) error {
	return m.raft.Barrier(timeout).Error()
}

// Snapshot triggers a user-requested Raft snapshot and blocks until it
// completes. Callers use this to force log truncation after large log
// entries so followers don't carry large blobs in their log indefinitely.
func (m *Manager) Snapshot() error {
	future := m.raft.Snapshot()
	if err := future.Error(); err != nil {
		return fmt.Errorf("raft snapshot: %w", err)
	}

	return nil
}

// UserRestore feeds an external snapshot (e.g. a user-uploaded backup) into
// the Raft cluster. The leader consumes the reader as a snapshot, bumps the
// index past commitIndex, and replicates to followers via InstallSnapshot.
// Each node's FSM.Restore is called exactly once. Must be called on the leader.
func (m *Manager) UserRestore(reader io.Reader, size int64, timeout time.Duration) error {
	meta := raft.SnapshotMeta{
		Version: raft.SnapshotVersionMax,
		Size:    size,
	}

	if err := m.raft.Restore(&meta, reader, timeout); err != nil {
		return fmt.Errorf("raft user restore: %w", err)
	}

	return nil
}

// State returns the current Raft state (Leader, Follower, Candidate, Shutdown).
func (m *Manager) State() raft.RaftState {
	return m.raft.State()
}

// Stats returns the Raft stats map (wraps raft.Stats()).
func (m *Manager) Stats() map[string]string {
	return m.raft.Stats()
}

// LeaderObserver returns the manager's leadership observer. Callers register
// LeaderCallback implementations before the observer's Run loop fires the
// initial state; in practice, registration happens between NewDatabase and
// the background-worker launch in runtime.go.
func (m *Manager) LeaderObserver() *LeaderObserver {
	return m.observer
}

// AddVoter adds a new node to the Raft cluster as a voting member. Only the
// leader can add nodes. The nodeID and address identify the new server; if the
// node already exists with a different address, it is updated.
func (m *Manager) AddVoter(nodeID int, address string) error {
	serverID := raft.ServerID(fmt.Sprintf("%d", nodeID))
	serverAddr := raft.ServerAddress(address)

	future := m.raft.AddVoter(serverID, serverAddr, 0, 0)
	if err := future.Error(); err != nil {
		return fmt.Errorf("add voter %d at %s: %w", nodeID, address, err)
	}

	return nil
}

// RemoveServer removes a node from the Raft cluster. Only the leader can
// remove nodes. After removal the target node will revert to follower state
// and stop receiving replication.
func (m *Manager) RemoveServer(nodeID int) error {
	serverID := raft.ServerID(fmt.Sprintf("%d", nodeID))

	future := m.raft.RemoveServer(serverID, 0, 0)
	if err := future.Error(); err != nil {
		return fmt.Errorf("remove server %d: %w", nodeID, err)
	}

	return nil
}

// LeaderAPIAddress returns the advertise-api-address of the current leader by
// looking up the leader's Raft transport address in the cluster members table
// via the provided resolver function. Returns empty string if this node is the
// leader or if the leader is unknown.
func (m *Manager) LeaderAPIAddress(resolver func(raftAddr string) string) string {
	if m.IsLeader() {
		return ""
	}

	addr := m.LeaderAddress()
	if addr == "" {
		return ""
	}

	return resolver(addr)
}

// ClusterEnabled returns whether the manager was started in HA mode.
func (m *Manager) ClusterEnabled() bool {
	return m.config.Enabled
}

// BoltNoSync reports whether the raft log store was opened with fsync
// disabled. Single-server nodes skip fsync because ella.db is the canonical
// FSM state and is itself fsynced on COMMIT; HA nodes keep fsync enabled
// because the raft log is the replicated source of truth.
func (m *Manager) BoltNoSync() bool {
	return m.boltNoSync
}

// LeadershipTransfer triggers a leadership transfer to another node.
func (m *Manager) LeadershipTransfer() error {
	return m.raft.LeadershipTransfer().Error()
}

// LeadershipTransferToServer triggers a leadership transfer to a specific
// target node. Returns an error if the target is not a voter or the transfer
// fails.
func (m *Manager) LeadershipTransferToServer(nodeID int, raftAddress string) error {
	serverID := raft.ServerID(fmt.Sprintf("%d", nodeID))
	serverAddr := raft.ServerAddress(raftAddress)

	future := m.raft.LeadershipTransferToServer(serverID, serverAddr)
	if err := future.Error(); err != nil {
		return fmt.Errorf("leadership transfer to %d: %w", nodeID, err)
	}

	return nil
}

// VoterIDs returns the server IDs of all voting members in the current Raft
// configuration. Returns nil on error (e.g. no quorum).
func (m *Manager) VoterIDs() []int {
	future := m.raft.GetConfiguration()
	if err := future.Error(); err != nil {
		return nil
	}

	var ids []int

	for _, srv := range future.Configuration().Servers {
		if srv.Suffrage != raft.Voter {
			continue
		}

		var id int
		if _, err := fmt.Sscanf(string(srv.ID), "%d", &id); err != nil {
			continue
		}

		ids = append(ids, id)
	}

	return ids
}

// Shutdown gracefully shuts down the Raft node.
func (m *Manager) Shutdown() error {
	if m.observer != nil {
		m.observer.Stop()
	}

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

// waitForLeader blocks until the cluster has an elected leader or ctx is
// cancelled. On the bootstrapper LeaderCh fires quickly; on joiners we
// poll LeaderWithID because LeaderCh only signals this node's own
// leadership transitions.
func (m *Manager) waitForLeader(ctx context.Context) error {
	if addr, _ := m.raft.LeaderWithID(); addr != "" {
		return nil
	}

	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case isLeader := <-m.raft.LeaderCh():
			if isLeader {
				return nil
			}
		case <-ticker.C:
			if addr, _ := m.raft.LeaderWithID(); addr != "" {
				return nil
			}
		}
	}
}

// AddNonvoter adds a new node to the Raft cluster as a non-voting member.
// Non-voters receive log replication but do not participate in elections
// or commit quorum. Used during rolling upgrades for catch-up before promotion.
func (m *Manager) AddNonvoter(nodeID int, address string) error {
	serverID := raft.ServerID(fmt.Sprintf("%d", nodeID))
	serverAddr := raft.ServerAddress(address)

	future := m.raft.AddNonvoter(serverID, serverAddr, 0, 0)
	if err := future.Error(); err != nil {
		return fmt.Errorf("add nonvoter %d at %s: %w", nodeID, address, err)
	}

	return nil
}
