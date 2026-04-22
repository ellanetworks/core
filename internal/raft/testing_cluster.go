// Copyright 2026 Ella Networks

package raft

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/cluster/listener"
	"github.com/ellanetworks/core/internal/cluster/listener/testutil"
	ellapki "github.com/ellanetworks/core/internal/pki"
	hraft "github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb/v2"
)

// TestCluster is a multi-node mTLS Raft cluster for HA unit tests.
type TestCluster struct {
	Nodes     []*Manager
	Listeners []*listener.Listener
	Appliers  []Applier
	t         testing.TB
	cancel    context.CancelFunc
	cleanup   sync.Once
}

// SetupTestCluster starts n Raft nodes over mTLS transports and bootstraps
// them into a single cluster. Each node gets its own cluster listener with
// a shared test CA. The first node bootstraps; the full server list is
// committed in a single configuration. Returns a cluster whose Nodes[0]
// is the initial leader.
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

	nodeIDs := make([]int, n)
	for i := range n {
		nodeIDs[i] = i + 1
	}

	pki := testutil.GenTestPKI(t, nodeIDs)

	type nodeInfo struct {
		mgr *Manager
		ln  *listener.Listener
	}

	nodes := make([]nodeInfo, 0, n)
	appliers := make([]Applier, 0, n)

	for i := range n {
		nodeID := i + 1
		a := newApplier()
		appliers = append(appliers, a)

		m, ln := createTestNode(t, nodeID, pki, a)
		nodes = append(nodes, nodeInfo{mgr: m, ln: ln})
	}

	// Start all cluster listeners so nodes can communicate.
	ctx, cancel := context.WithCancel(context.Background())

	for _, ni := range nodes {
		if err := ni.ln.Start(ctx); err != nil {
			t.Fatalf("start cluster listener: %v", err)
		}
	}

	// Bootstrap node 0 with the full server list.
	servers := make([]hraft.Server, 0, n)

	for _, ni := range nodes {
		servers = append(servers, hraft.Server{
			ID:      hraft.ServerID(fmt.Sprintf("%d", ni.mgr.nodeID)),
			Address: ni.mgr.transport.LocalAddr(),
		})
	}

	bootCfg := hraft.Configuration{Servers: servers}
	if err := nodes[0].mgr.raft.BootstrapCluster(bootCfg).Error(); err != nil {
		cancel()

		for _, ni := range nodes {
			_ = ni.mgr.Shutdown()
			ni.ln.Stop()
		}

		t.Fatalf("bootstrap: %v", err)
	}

	// Wait for node 0 to become leader.
	if err := waitForLeaderTest(t, nodes[0].mgr); err != nil {
		cancel()

		for _, ni := range nodes {
			_ = ni.mgr.Shutdown()
			ni.ln.Stop()
		}

		t.Fatalf("wait for leader: %v", err)
	}

	// Ensure the leader's configuration entry is replicated to all followers
	// before returning. Without this, followers may not yet know the cluster
	// membership and would never start elections if the leader is partitioned.
	if err := nodes[0].mgr.raft.Barrier(5 * time.Second).Error(); err != nil {
		cancel()

		for _, ni := range nodes {
			_ = ni.mgr.Shutdown()
			ni.ln.Stop()
		}

		t.Fatalf("barrier: %v", err)
	}

	managers := make([]*Manager, 0, n)
	listeners := make([]*listener.Listener, 0, n)

	for _, ni := range nodes {
		go ni.mgr.observer.Run(ni.mgr.raft)

		managers = append(managers, ni.mgr)
		listeners = append(listeners, ni.ln)
	}

	tc := &TestCluster{
		Nodes:     managers,
		Listeners: listeners,
		Appliers:  appliers,
		t:         t,
		cancel:    cancel,
	}

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

// Close shuts down all nodes and listeners in the cluster.
func (tc *TestCluster) Close() {
	tc.cleanup.Do(func() {
		for _, m := range tc.Nodes {
			if err := m.Shutdown(); err != nil && !errors.Is(err, hraft.ErrRaftShutdown) {
				tc.t.Errorf("shutdown node %d: %v", m.nodeID, err)
			}
		}

		for _, ln := range tc.Listeners {
			ln.Stop()
		}

		if tc.cancel != nil {
			tc.cancel()
		}
	})
}

func createTestNode(t testing.TB, nodeID int, pki *testutil.PKI, applier Applier) (*Manager, *listener.Listener) {
	t.Helper()

	dataDir := t.TempDir()

	raftDir := filepath.Join(dataDir, "raft")
	if err := os.MkdirAll(raftDir, 0o700); err != nil {
		t.Fatalf("create raft directory for node %d: %v", nodeID, err)
	}

	fsm := NewFSM(applier, dataDir)

	port := freePort(t)
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	bundle := pki.Bundle()
	leaf := pki.LeafFunc(nodeID)

	ln := listener.New(listener.Config{
		BindAddress:      addr,
		AdvertiseAddress: addr,
		NodeID:           nodeID,
		TrustBundle:      func() *ellapki.TrustBundle { return bundle },
		Leaf:             leaf,
		Revoked:          func(*big.Int) bool { return false },
	})

	sl, err := newRaftStreamLayer(ln, addr)
	if err != nil {
		t.Fatalf("create stream layer for node %d: %v", nodeID, err)
	}

	transport := hraft.NewNetworkTransport(sl, 3, 10*time.Second, newZapIOWriter("transport"))

	cfg := hraft.DefaultConfig()
	cfg.LocalID = hraft.ServerID(fmt.Sprintf("%d", nodeID))
	cfg.Logger = newZapRaftLogger()
	cfg.HeartbeatTimeout = 50 * time.Millisecond
	cfg.ElectionTimeout = 50 * time.Millisecond
	cfg.LeaderLeaseTimeout = 50 * time.Millisecond
	cfg.CommitTimeout = 5 * time.Millisecond
	cfg.TrailingLogs = 100
	cfg.SnapshotThreshold = 100
	cfg.SnapshotInterval = time.Second

	boltPath := filepath.Join(raftDir, "raft.db")

	var (
		boltStore *raftboltdb.BoltStore
		snapshots hraft.SnapshotStore
	)

	if err := withTightUmask(func() error {
		var bsErr error

		boltStore, bsErr = raftboltdb.NewBoltStore(boltPath)
		if bsErr != nil {
			return fmt.Errorf("create bolt store for node %d: %w", nodeID, bsErr)
		}

		var ssErr error

		snapshots, ssErr = hraft.NewFileSnapshotStore(raftDir, 3, newZapIOWriter("snapshot"))
		if ssErr != nil {
			_ = boltStore.Close()
			return fmt.Errorf("create snapshot store for node %d: %w", nodeID, ssErr)
		}

		return nil
	}); err != nil {
		t.Fatalf("%v", err)
	}

	logCache, err := hraft.NewLogCache(raftLogCacheSize, boltStore)
	if err != nil {
		_ = boltStore.Close()

		t.Fatalf("create log cache for node %d: %v", nodeID, err)
	}

	r, err := hraft.NewRaft(cfg, fsm, logCache, boltStore, snapshots, transport)
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
		config:    ClusterConfig{Enabled: true, BindAddress: addr, AdvertiseAddress: addr},
		nodeID:    nodeID,
		dataDir:   dataDir,
		observer:  observer,
	}

	return m, ln
}

func freePort(t testing.TB) int {
	t.Helper()

	lc := net.ListenConfig{}

	l, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("find free port: %v", err)
	}

	port := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()

	return port
}
