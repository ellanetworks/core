// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package raft

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ellanetworks/core/internal/cluster/listener"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/osutil"
	"github.com/hashicorp/raft"
	autopilot "github.com/hashicorp/raft-autopilot"
	raftboltdb "github.com/hashicorp/raft-boltdb/v2"
	"go.uber.org/zap"
)

var ErrBarrierTimeout = errors.New("raft barrier timed out")

// AutopilotConfig overrides autopilot's timing. A zero field keeps the
// package default.
type AutopilotConfig struct {
	// LastContactThreshold bounds how long a server may go without leader
	// contact before autopilot reports it unhealthy.
	LastContactThreshold time.Duration

	// ServerStabilizationTime is how long a non-voter must stay healthy
	// before autopilot promotes it.
	ServerStabilizationTime time.Duration

	// ReconcileInterval and UpdateInterval pace autopilot's promotion and
	// health passes.
	ReconcileInterval time.Duration
	UpdateInterval    time.Duration
}

// ClusterConfig holds the cluster-related configuration parsed from YAML.
type ClusterConfig struct {
	Enabled          bool
	NodeID           int
	BindAddress      string
	AdvertiseAddress string
	APIAddress       string
	Peers            []string

	// HasJoinToken signals that a join-token was provided in config. A node
	// with a join-token is a joiner and must never solo-bootstrap; a node
	// without one is the founder and solo-bootstraps immediately when no
	// formed peer is reachable.
	HasJoinToken bool

	JoinTimeout       time.Duration
	ProposeTimeout    time.Duration
	SnapshotInterval  time.Duration
	SnapshotThreshold uint64

	PerformanceMultiplier int

	HeartbeatTimeout   time.Duration
	ElectionTimeout    time.Duration
	LeaderLeaseTimeout time.Duration
	CommitTimeout      time.Duration

	Autopilot AutopilotConfig

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

	// BinaryVersion is sent in the join handshake and recorded on the
	// joiner's cluster_members row. Used for operator inventory only;
	// the migration gate reads SchemaVersion live via /cluster/status.
	BinaryVersion string
}

const (
	// defaultPerformanceMultiplier is the per-operator scaling factor
	// applied to the library's default timeouts when running in HA mode.
	defaultPerformanceMultiplier = 5

	standalonePerformanceMultiplier = 1

	// defaultProposeTimeout caps how long a write waits for Raft commit
	// before the API layer returns 503. 5 s is generous for single-server
	// (commit is microseconds) and a reasonable default for HA with healthy
	// replication; operators tune via ClusterConfig.ProposeTimeout.
	defaultProposeTimeout = 5 * time.Second

	// defaultStandaloneNodeID is the node ID a standalone install adopts and
	// persists when neither config, environment, nor a previously written
	// node-id file supplies one.
	defaultStandaloneNodeID = 1

	leaderPollInterval = 25 * time.Millisecond

	leaderBarrierRetryInterval = 1 * time.Second
)

var errShuttingDown = errors.New("raft manager shutting down")

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
	autopilot       *autopilotRunner
	followerTracker *followerTracker
	boltNoSync      bool
	clusterListener *listener.Listener

	leaderClient *leaderHTTPClient

	discoveryPending atomic.Bool
	discoveryFatal   atomic.Pointer[string]

	barrieredTerm atomic.Uint64
	barrierMu     sync.Mutex
	barrier       *barrierAttempt

	leaderBarrier     chan struct{}
	leaderBarrierOnce sync.Once

	shutdownCh   chan struct{}
	shutdownOnce sync.Once
}

type leaderBarrierCallback struct {
	m *Manager
}

func (c leaderBarrierCallback) OnLostLeadership() {}

func (c leaderBarrierCallback) OnBecameLeader() {
	for {
		err := c.m.barrierForLeadership()
		if err == nil {
			c.m.leaderBarrierOnce.Do(func() { close(c.m.leaderBarrier) })
			return
		}

		if errors.Is(err, errShuttingDown) || c.m.raft.State() != raft.Leader {
			logger.RaftLog.Warn("Post-leadership barrier abandoned", zap.Error(err))
			return
		}

		logger.RaftLog.Error("Post-leadership barrier failed; retrying before leader initialization runs",
			zap.Error(err))

		select {
		case <-c.m.shutdownCh:
			return
		case <-time.After(leaderBarrierRetryInterval):
		}
	}
}

func (m *Manager) barrierForLeadership() error {
	term := m.raft.CurrentTerm()
	if term != 0 && m.barrieredTerm.Load() == term {
		return nil
	}

	if m.raft.State() != raft.Leader {
		return raft.ErrNotLeader
	}

	att := m.barrierFor(term)

	select {
	case <-att.done:
		return att.err
	case <-m.shutdownCh:
		return errShuttingDown
	}
}

func (m *Manager) WaitForLeaderBarrier(ctx context.Context) error {
	select {
	case <-m.leaderBarrier:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type barrierAttempt struct {
	term uint64
	done chan struct{}
	err  error
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
// Tests that want an in-memory transport should use NewTestManager instead.
func NewManager(_ context.Context, cfg ClusterConfig, applier Applier, dataDir string, opts ...ManagerOption) (*Manager, error) {
	options := managerOptions{}
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

	var (
		boltStore     *raftboltdb.BoltStore
		snapshotStore raft.SnapshotStore
	)

	err = osutil.WithTightUmask(func() error {
		var bsErr error

		boltStore, bsErr = raftboltdb.New(raftboltdb.Options{
			Path:   boltPath,
			NoSync: false,
		})
		if bsErr != nil {
			return fmt.Errorf("create bolt store at %s: %w", boltPath, bsErr)
		}

		var ssErr error

		snapshotStore, ssErr = raft.NewFileSnapshotStore(raftDir, 3, newZapIOWriter("snapshot"))
		if ssErr != nil {
			_ = boltStore.Close()
			return fmt.Errorf("create snapshot store: %w", ssErr)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	logCache, err := raft.NewLogCache(raftLogCacheSize, boltStore)
	if err != nil {
		_ = boltStore.Close()
		return nil, fmt.Errorf("create log cache: %w", err)
	}

	var transport raft.Transport

	if !singleServer && options.clusterListener != nil {
		transport, err = clusterTransportFactory(options.clusterListener, cfg)
	} else {
		transport, err = tcpTransportFactory(cfg)
	}

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
	applyTimeouts(raftConfig, cfg, singleServer)

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
		raft:          r,
		fsm:           fsm,
		transport:     transport,
		logStore:      boltStore,
		snaps:         snapshotStore,
		config:        cfg,
		nodeID:        nodeID,
		dataDir:       dataDir,
		observer:      observer,
		boltNoSync:    false,
		leaderBarrier: make(chan struct{}),
		shutdownCh:    make(chan struct{}),
	}

	observer.Register(leaderBarrierCallback{m: m})

	m.discoveryPending.Store(!singleServer && !hasState && !recovered)

	m.attachClusterListener(options.clusterListener)

	if !singleServer {
		ft := newFollowerTracker(r)
		m.followerTracker = ft
		m.autopilot = newAutopilotRunner(r, m)

		observer.Register(ft.asLeaderCallback(raft.ServerID(strconv.Itoa(nodeID))))
		observer.Register(m.autopilot)
	}

	if singleServer {
		warnOnMultiServerStandaloneState(r, nodeID, raftDir)
	}

	go observer.Run(r)

	return m, nil
}

func describeServers(r *raft.Raft) string {
	future := r.GetConfiguration()
	if err := future.Error(); err != nil {
		return fmt.Sprintf("unreadable: %v", err)
	}

	servers := future.Configuration().Servers
	if len(servers) == 0 {
		return "none"
	}

	ids := make([]string, 0, len(servers))
	for _, s := range servers {
		ids = append(ids, string(s.ID))
	}

	return strings.Join(ids, ", ")
}

func warnOnMultiServerStandaloneState(r *raft.Raft, nodeID int, raftDir string) {
	future := r.GetConfiguration()
	if err := future.Error(); err != nil {
		return
	}

	if len(future.Configuration().Servers) <= 1 {
		return
	}

	logger.RaftLog.Error("Standalone node cannot elect itself: on-disk raft state lists multiple servers",
		zap.Int("node_id", nodeID),
		zap.String("raft_dir", raftDir),
		zap.String("servers", describeServers(r)),
		zap.String("remedy", "write a peers.json recovery file into the raft directory to reset the server configuration"),
	)
}

// resolveNodeIDForMode picks the Raft server ID. Both modes go through the
// same config/env/file chain, which persists the ID on first boot and
// rejects later mismatches that would invalidate issued GUTIs. Single-server
// mode additionally falls back to defaultStandaloneNodeID when no source
// supplies one, so standalone installs need not provision cluster.node-id.
func resolveNodeIDForMode(cfg ClusterConfig, singleServer bool, dataDir string) (int, error) {
	fallback := 0
	if singleServer {
		fallback = defaultStandaloneNodeID
	}

	id, err := resolveNodeID(cfg.NodeID, dataDir, fallback)
	if err != nil {
		return 0, fmt.Errorf("resolve node ID: %w", err)
	}

	return id, nil
}

// applyTimeouts configures heartbeat / election / leader-lease / commit
// timeouts.
func applyTimeouts(rc *raft.Config, cfg ClusterConfig, singleServer bool) {
	multiplier := cfg.PerformanceMultiplier
	if multiplier <= 0 {
		multiplier = defaultPerformanceMultiplier
		if singleServer {
			multiplier = standalonePerformanceMultiplier
		}
	}

	rc.HeartbeatTimeout *= time.Duration(multiplier)
	rc.ElectionTimeout *= time.Duration(multiplier)
	rc.LeaderLeaseTimeout *= time.Duration(multiplier)

	if cfg.HeartbeatTimeout > 0 {
		rc.HeartbeatTimeout = cfg.HeartbeatTimeout
	}

	if cfg.ElectionTimeout > 0 {
		rc.ElectionTimeout = cfg.ElectionTimeout
	}

	if cfg.LeaderLeaseTimeout > 0 {
		rc.LeaderLeaseTimeout = cfg.LeaderLeaseTimeout
	}

	if cfg.CommitTimeout > 0 {
		rc.CommitTimeout = cfg.CommitTimeout
	}
}

// Propose serializes a command and applies it through Raft consensus.
// Only the leader can propose; followers receive ErrNotLeader.
// ProposeResult holds the FSM response and the raft log index.
type ProposeResult struct {
	Value any
	Index uint64
}

// Propose marshals and applies a command through Raft. Callers must be
// the leader; followers receive raft.ErrNotLeader. Follower-side
// forwarding is handled by the typed-op dispatch layer (internal/db)
// through Manager.ForwardOperation.
func (m *Manager) Propose(cmd *Command, timeout time.Duration) (*ProposeResult, error) {
	data, err := cmd.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("marshal command: %w", err)
	}

	return m.ApplyBytes(data, timeout)
}

// ApplyBytes applies pre-marshalled Command bytes through Raft. Unlike
// Propose it does not branch on leadership: the caller must already be
// the leader, and receives raft.ErrNotLeader / raft.ErrLeadershipLost on
// the failure paths. Used by the /cluster/internal/propose handler to
// commit a command forwarded from a follower, and by Propose itself as
// the local fast path on the leader.
func (m *Manager) ApplyBytes(data []byte, timeout time.Duration) (*ProposeResult, error) {
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

// AutopilotState returns the current autopilot state snapshot.
//
// Autopilot only runs on the leader, so this returns nil on followers and
// in single-server mode. On the leader, it may still return nil during
// the cold-start window immediately after leadership acquisition, before
// the first autopilot tick has completed.
func (m *Manager) AutopilotState() *autopilot.State {
	if m.autopilot == nil {
		return nil
	}

	return m.autopilot.State()
}

// LeaderAddress returns the Raft transport address of the current leader.
// Returns empty string if there is no leader.
func (m *Manager) LeaderAddress() string {
	addr, _ := m.raft.LeaderWithID()
	return string(addr)
}

// LeaderAddressAndID returns the leader's Raft transport address together
// with the integer node-id parsed from the leader's Raft ServerID.
// Callers dialing the leader over mTLS use the ID to enforce the
// expected-peer check. A zero ID indicates the server-id did not parse
// as an integer (should not happen given bootstrap writes node-id as a
// decimal string, but the caller should still guard).
func (m *Manager) LeaderAddressAndID() (string, int) {
	addr, id := m.raft.LeaderWithID()
	if addr == "" {
		return "", 0
	}

	n, err := strconv.Atoi(string(id))
	if err != nil {
		return string(addr), 0
	}

	return string(addr), n
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

// AdvertiseAddress returns the cluster address peers use to reach this node.
func (m *Manager) AdvertiseAddress() string {
	return m.config.AdvertiseAddress
}

// APIAddress returns the operator-facing API URL for this node.
func (m *Manager) APIAddress() string {
	return m.config.APIAddress
}

func (m *Manager) attachClusterListener(ln *listener.Listener) {
	m.clusterListener = ln

	if ln == nil {
		return
	}

	m.leaderClient = newLeaderHTTPClient(func(ctx context.Context, addr string, peerID int) (net.Conn, error) {
		return ln.Dial(ctx, addr, peerID, listener.ALPNHTTP, dialTimeout)
	})
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

// WriteBarrier blocks until the FSM has applied every entry committed before
// this call. raft.State() reports Leader while entries from the previous term
// are still queued for the FSM, so a changeset captured in that window carries
// pre-images those entries invalidate. Once per term is enough: everything
// this node appends afterwards is ordered behind the barrier.
func (m *Manager) WriteBarrier(timeout time.Duration) error {
	term := m.raft.CurrentTerm()
	if term != 0 && m.barrieredTerm.Load() == term {
		return nil
	}

	if m.raft.State() != raft.Leader {
		return raft.ErrNotLeader
	}

	att := m.barrierFor(term)

	select {
	case <-att.done:
		return att.err
	case <-time.After(timeout):
		return ErrBarrierTimeout
	}
}

// barrierFor shares one in-flight barrier per term: a caller that gives up
// leaves it running, so repeated timeouts still cost the log a single entry.
func (m *Manager) barrierFor(term uint64) *barrierAttempt {
	m.barrierMu.Lock()
	defer m.barrierMu.Unlock()

	if m.barrier != nil && m.barrier.term == term {
		return m.barrier
	}

	att := &barrierAttempt{term: term, done: make(chan struct{})}
	m.barrier = att

	go func() {
		att.err = m.raft.Barrier(0).Error()

		if att.err == nil {
			m.barrieredTerm.Store(term)
		}

		m.barrierMu.Lock()
		if m.barrier == att {
			m.barrier = nil
		}
		m.barrierMu.Unlock()

		close(att.done)
	}()

	return att
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

// ClusterEnabled returns whether the manager was started in HA mode.
func (m *Manager) ClusterEnabled() bool {
	return m.config.Enabled
}

// BoltNoSync reports whether the raft log store was opened with fsync
// disabled.
func (m *Manager) BoltNoSync() bool {
	return m.boltNoSync
}

// LeadershipTransfer triggers a leadership transfer to another node.
func (m *Manager) LeadershipTransfer() error {
	return m.raft.LeadershipTransfer().Error()
}

// MemberIDs returns the current Raft configuration, nonvoters included: they
// apply committed entries too. Nil on error.
func (m *Manager) MemberIDs() []int {
	future := m.raft.GetConfiguration()
	if err := future.Error(); err != nil {
		return nil
	}

	var ids []int

	for _, srv := range future.Configuration().Servers {
		id, err := strconv.Atoi(string(srv.ID))
		if err != nil {
			continue
		}

		ids = append(ids, id)
	}

	return ids
}

// Shutdown gracefully shuts down the Raft node.
func (m *Manager) Shutdown() error {
	m.shutdownOnce.Do(func() { close(m.shutdownCh) })

	if m.observer != nil {
		m.observer.Stop()
	}

	if m.autopilot != nil {
		<-m.autopilot.ap.Stop()
	}

	if m.followerTracker != nil {
		m.followerTracker.stop()
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

	if m.leaderClient != nil {
		m.leaderClient.close()
	}

	return nil
}

func (m *Manager) HasLeader() bool {
	addr, _ := m.raft.LeaderWithID()
	return addr != ""
}

func (m *Manager) WaitForLeader(ctx context.Context) error {
	return m.waitForLeader(ctx)
}

func (m *Manager) HoldForLeader(ctx context.Context, timeout time.Duration) error {
	if m.HasLeader() {
		return nil
	}

	holdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	return m.waitForLeader(holdCtx)
}

// waitForLeader blocks until the cluster has an elected leader or ctx is
// cancelled. It polls LeaderWithID rather than selecting on LeaderCh, which
// only reports this node's own transitions and delivers each value to exactly
// one receiver, so LeaderObserver must stay its sole consumer.
func (m *Manager) waitForLeader(ctx context.Context) error {
	if addr, _ := m.raft.LeaderWithID(); addr != "" {
		return nil
	}

	ticker := time.NewTicker(leaderPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
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
