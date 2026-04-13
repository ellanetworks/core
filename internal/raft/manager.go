// Copyright 2026 Ella Networks

package raft

import (
	"context"
	"fmt"
	"io"
	"net"
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

// NewManager creates and starts a Raft node. The applier is called by the FSM
// for every committed log entry.
func NewManager(ctx context.Context, cfg ClusterConfig, applier Applier, dataDir string) (*Manager, error) {
	nodeID, err := ResolveNodeID(cfg.NodeID, dataDir)
	if err != nil {
		return nil, fmt.Errorf("resolve node ID: %w", err)
	}

	raftDir := filepath.Join(dataDir, "raft")
	if err := os.MkdirAll(raftDir, 0o700); err != nil {
		return nil, fmt.Errorf("create raft directory: %w", err)
	}

	fsm := NewFSM(applier, dataDir)
	idCounters := NewIDCounters()

	raftConfig := raft.DefaultConfig()
	raftConfig.LocalID = raft.ServerID(fmt.Sprintf("%d", nodeID))
	raftConfig.SnapshotInterval = cfg.SnapshotInterval
	raftConfig.SnapshotThreshold = cfg.SnapshotThreshold
	raftConfig.Logger = newZapRaftLogger()

	// BoltDB for LogStore and StableStore.
	boltPath := filepath.Join(raftDir, "raft.db")

	boltStore, err := raftboltdb.NewBoltStore(boltPath)
	if err != nil {
		return nil, fmt.Errorf("create bolt store at %s: %w", boltPath, err)
	}

	snapshotStore, err := raft.NewFileSnapshotStore(raftDir, 3, os.Stderr)
	if err != nil {
		_ = boltStore.Close()
		return nil, fmt.Errorf("create snapshot store: %w", err)
	}

	bindAddr, err := net.ResolveTCPAddr("tcp", cfg.BindAddress)
	if err != nil {
		_ = boltStore.Close()
		return nil, fmt.Errorf("resolve bind address %s: %w", cfg.BindAddress, err)
	}

	transport, err := raft.NewTCPTransport(cfg.BindAddress, bindAddr, 3, 10*time.Second, os.Stderr)
	if err != nil {
		_ = boltStore.Close()
		return nil, fmt.Errorf("create TCP transport: %w", err)
	}

	r, err := raft.NewRaft(raftConfig, fsm, boltStore, boltStore, snapshotStore, transport)
	if err != nil {
		_ = transport.Close()
		_ = boltStore.Close()

		return nil, fmt.Errorf("create raft instance: %w", err)
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

	// Watch for leadership changes to seed ID counters.
	go m.watchLeadership(ctx, applier)

	return m, nil
}

// watchLeadership monitors raft.LeaderCh() and seeds ID counters on promotion.
func (m *Manager) watchLeadership(ctx context.Context, applier Applier) {
	for {
		select {
		case <-ctx.Done():
			return
		case isLeader := <-m.raft.LeaderCh():
			if isLeader {
				logger.DBLog.Info("Raft: this node is now the leader",
					zap.Int("nodeID", m.nodeID))

				if err := m.idCounters.SeedFromDB(ctx, applier.SharedPlainDB()); err != nil {
					logger.DBLog.Error("Raft: failed to seed ID counters on leader promotion",
						zap.Error(err))
				}
			} else {
				logger.DBLog.Info("Raft: this node lost leadership",
					zap.Int("nodeID", m.nodeID))
			}
		}
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
