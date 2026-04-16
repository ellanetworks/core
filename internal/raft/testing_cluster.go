// Copyright 2026 Ella Networks

package raft

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb/v2"
)

// TestCluster is a multi-node in-memory Raft cluster for HA unit tests.
type TestCluster struct {
	Nodes    []*Manager
	Appliers []Applier
	t        testing.TB
	cleanup  sync.Once
}

// SetupTestCluster starts n Raft nodes over in-memory transports and bootstraps
// them into a single cluster. The first node bootstraps; subsequent nodes join
// via AddVoter. Returns a cluster whose Nodes[0] is the initial leader.
func SetupTestCluster(t testing.TB, n int, applier Applier) *TestCluster {
	return SetupTestClusterWithAppliers(t, n, func() Applier { return applier })
}

// SetupTestClusterWithAppliers is like SetupTestCluster but calls newApplier
// once per node, giving each its own Applier (and thus its own SQLite database).
// Use this when testing FSM state comparison across nodes.
func SetupTestClusterWithAppliers(t testing.TB, n int, newApplier func() Applier) *TestCluster {
	t.Helper()

	if n < 1 {
		t.Fatal("cluster size must be >= 1")
	}

	type nodeInfo struct {
		mgr       *Manager
		addr      raft.ServerAddress
		transport *raft.InmemTransport
	}

	nodes := make([]nodeInfo, 0, n)
	appliers := make([]Applier, 0, n)

	// Create all nodes first.
	for i := range n {
		nodeID := i + 1
		a := newApplier()
		appliers = append(appliers, a)

		m, addr, transport := createTestNode(t, nodeID, a)
		nodes = append(nodes, nodeInfo{mgr: m, addr: addr, transport: transport})
	}

	// Wire up all transports so they can talk to each other.
	for i := range nodes {
		for j := range nodes {
			if i != j {
				nodes[i].transport.Connect(nodes[j].addr, nodes[j].transport)
			}
		}
	}

	// Bootstrap node 0 with the full server list.
	servers := make([]raft.Server, 0, n)
	for _, ni := range nodes {
		servers = append(servers, raft.Server{
			ID:      raft.ServerID(fmt.Sprintf("%d", ni.mgr.nodeID)),
			Address: ni.addr,
		})
	}

	bootCfg := raft.Configuration{Servers: servers}
	if err := nodes[0].mgr.raft.BootstrapCluster(bootCfg).Error(); err != nil {
		for _, ni := range nodes {
			_ = ni.mgr.Shutdown()
		}

		t.Fatalf("bootstrap: %v", err)
	}

	// Wait for node 0 to become leader.
	if err := waitForLeaderTest(t, nodes[0].mgr); err != nil {
		for _, ni := range nodes {
			_ = ni.mgr.Shutdown()
		}

		t.Fatalf("wait for leader: %v", err)
	}

	// Ensure the leader's configuration entry is replicated to all followers
	// before returning. Without this, followers may not yet know the cluster
	// membership and would never start elections if the leader is partitioned.
	if err := nodes[0].mgr.raft.Barrier(5 * time.Second).Error(); err != nil {
		for _, ni := range nodes {
			_ = ni.mgr.Shutdown()
		}

		t.Fatalf("barrier: %v", err)
	}

	// Start observers.
	managers := make([]*Manager, 0, n)

	for _, ni := range nodes {
		go ni.mgr.observer.Run(ni.mgr.raft)

		managers = append(managers, ni.mgr)
	}

	tc := &TestCluster{Nodes: managers, Appliers: appliers, t: t}
	t.Cleanup(tc.Close)

	return tc
}

// Leader returns the current leader node, or nil if none.
func (tc *TestCluster) Leader() *Manager {
	for _, m := range tc.Nodes {
		if m.IsLeader() {
			return m
		}
	}

	return nil
}

// WaitForConvergence polls until every node's AppliedIndex reaches at least
// minIndex. Returns an error if the timeout expires first.
func (tc *TestCluster) WaitForConvergence(minIndex uint64, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		converged := true

		for _, m := range tc.Nodes {
			if m.AppliedIndex() < minIndex {
				converged = false
				break
			}
		}

		if converged {
			return nil
		}

		time.Sleep(5 * time.Millisecond)
	}

	return fmt.Errorf("not all nodes converged to index %d within %v", minIndex, timeout)
}

// Close shuts down all nodes in the cluster.
func (tc *TestCluster) Close() {
	tc.cleanup.Do(func() {
		for _, m := range tc.Nodes {
			if err := m.Shutdown(); err != nil && !errors.Is(err, raft.ErrRaftShutdown) {
				tc.t.Errorf("shutdown node %d: %v", m.nodeID, err)
			}
		}
	})
}

func createTestNode(t testing.TB, nodeID int, applier Applier) (*Manager, raft.ServerAddress, *raft.InmemTransport) {
	t.Helper()

	dataDir := t.TempDir()

	raftDir := filepath.Join(dataDir, "raft")
	if err := os.MkdirAll(raftDir, 0o700); err != nil {
		t.Fatalf("create raft directory for node %d: %v", nodeID, err)
	}

	fsm := NewFSM(applier, dataDir)

	cfg := raft.DefaultConfig()
	cfg.LocalID = raft.ServerID(fmt.Sprintf("%d", nodeID))
	cfg.Logger = newZapRaftLogger()
	cfg.HeartbeatTimeout = 50 * time.Millisecond
	cfg.ElectionTimeout = 50 * time.Millisecond
	cfg.LeaderLeaseTimeout = 50 * time.Millisecond
	cfg.CommitTimeout = 5 * time.Millisecond
	cfg.TrailingLogs = 100
	cfg.SnapshotThreshold = 100
	cfg.SnapshotInterval = time.Second

	boltPath := filepath.Join(raftDir, "raft.db")

	boltStore, err := raftboltdb.NewBoltStore(boltPath)
	if err != nil {
		t.Fatalf("create bolt store for node %d: %v", nodeID, err)
	}

	snapshots, err := raft.NewFileSnapshotStore(raftDir, 3, newZapIOWriter("snapshot"))
	if err != nil {
		_ = boltStore.Close()

		t.Fatalf("create snapshot store for node %d: %v", nodeID, err)
	}

	logCache, err := raft.NewLogCache(raftLogCacheSize, boltStore)
	if err != nil {
		_ = boltStore.Close()

		t.Fatalf("create log cache for node %d: %v", nodeID, err)
	}

	addr, transport := raft.NewInmemTransport("")

	r, err := raft.NewRaft(cfg, fsm, logCache, boltStore, snapshots, transport)
	if err != nil {
		_ = boltStore.Close()

		t.Fatalf("create raft for node %d: %v", nodeID, err)
	}

	observer := NewLeaderObserver()

	m := &Manager{
		raft:      r,
		fsm:       fsm,
		transport: transport,
		logStore:  boltStore,
		snaps:     snapshots,
		config:    ClusterConfig{Enabled: true, BindAddress: string(addr)},
		nodeID:    nodeID,
		dataDir:   dataDir,
		observer:  observer,
	}

	return m, addr, transport
}
