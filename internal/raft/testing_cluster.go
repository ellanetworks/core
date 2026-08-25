// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package raft

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/cluster/listener"
	"github.com/ellanetworks/core/internal/cluster/listener/testutil"
	"github.com/ellanetworks/core/internal/pki"
	hraft "github.com/hashicorp/raft"
)

// peerGate is one node's view of which peers it can reach. A blocked peer
// is dropped from this node's pin lookup, so the mTLS handshake fails in
// both directions — the same rejection a node gets once its pin row is
// deleted, and the closest a test can get to a partition without touching
// the production dial and accept paths.
type peerGate struct {
	mu      sync.RWMutex
	blocked map[int]struct{}
}

func newPeerGate() *peerGate {
	return &peerGate{blocked: make(map[int]struct{})}
}

func (g *peerGate) block(nodeID int) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.blocked[nodeID] = struct{}{}
}

func (g *peerGate) reset() {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.blocked = make(map[int]struct{})
}

func (g *peerGate) reachable(nodeID int) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()

	_, blocked := g.blocked[nodeID]

	return !blocked
}

type testNode struct {
	nodeID  int
	addr    string
	dataDir string
	applier Applier
	gate    *peerGate
	mgr     *Manager
	ln      *listener.Listener
	running bool
}

// ClusterWiring registers cluster-port handlers for one node. The harness
// runs it at setup and again after RestartNode, so the listener always
// dispatches to the node's current Manager.
type ClusterWiring func(idx int, m *Manager, ln *listener.Listener)

type clusterWiring struct {
	alpns   []string
	install ClusterWiring
}

// TestCluster is a multi-node mTLS Raft cluster for HA unit tests.
type TestCluster struct {
	Nodes     []*Manager
	Listeners []*listener.Listener
	Appliers  []Applier

	nodes   []*testNode
	wirings []clusterWiring
	pki     *testutil.PKI
	t       testing.TB
	ctx     context.Context
	cancel  context.CancelFunc
	cleanup sync.Once
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

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	tc := &TestCluster{
		pki:    testutil.GenTestPKI(t, nodeIDs),
		t:      t,
		ctx:    ctx,
		cancel: cancel,
	}

	t.Cleanup(tc.Close)

	ports, releasePorts := reservePorts(t, n)

	for i := range n {
		tc.nodes = append(tc.nodes, &testNode{
			nodeID:  i + 1,
			addr:    fmt.Sprintf("127.0.0.1:%d", ports[i]),
			dataDir: t.TempDir(),
			applier: newApplier(),
			gate:    newPeerGate(),
		})
	}

	for _, node := range tc.nodes {
		tc.newListener(node)
		tc.startNode(node)
	}

	releasePorts()

	for _, node := range tc.nodes {
		tc.startListener(node)
	}

	tc.refresh()

	servers := make([]hraft.Server, 0, n)
	for _, node := range tc.nodes {
		servers = append(servers, hraft.Server{
			ID:      hraft.ServerID(fmt.Sprintf("%d", node.nodeID)),
			Address: node.mgr.transport.LocalAddr(),
		})
	}

	if err := tc.nodes[0].mgr.raft.BootstrapCluster(hraft.Configuration{Servers: servers}).Error(); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	if err := waitForLeaderTest(t, tc.nodes[0].mgr); err != nil {
		t.Fatalf("wait for leader: %v", err)
	}

	if err := tc.nodes[0].mgr.raft.Barrier(5 * time.Second).Error(); err != nil {
		t.Fatalf("barrier: %v", err)
	}

	return tc
}

func (tc *TestCluster) startNode(node *testNode) {
	tc.t.Helper()

	cfg := ClusterConfig{
		Enabled:            true,
		NodeID:             node.nodeID,
		BindAddress:        node.addr,
		AdvertiseAddress:   node.addr,
		HeartbeatTimeout:   50 * time.Millisecond,
		ElectionTimeout:    50 * time.Millisecond,
		LeaderLeaseTimeout: 50 * time.Millisecond,
		CommitTimeout:      5 * time.Millisecond,
		SnapshotInterval:   time.Second,
		SnapshotThreshold:  100,
		TrailingLogs:       100,

		Autopilot: AutopilotConfig{
			LastContactThreshold:    200 * time.Millisecond,
			ServerStabilizationTime: 100 * time.Millisecond,
			ReconcileInterval:       100 * time.Millisecond,
			UpdateInterval:          50 * time.Millisecond,
		},
	}

	m, err := NewManager(tc.ctx, cfg, node.applier, node.dataDir, WithClusterListener(node.ln))
	if err != nil {
		tc.t.Fatalf("create node %d: %v", node.nodeID, err)
	}

	node.mgr = m
	node.running = true
}

func (tc *TestCluster) newListener(node *testNode) {
	node.ln = listener.New(listener.Config{
		BindAddress:      node.addr,
		AdvertiseAddress: node.addr,
		NodeID:           node.nodeID,
		Pin:              tc.pinFunc(node),
		Leaf:             tc.pki.LeafFunc(node.nodeID),
	})
}

// pinFunc resolves fingerprints against the test PKI, minus the peers this
// node's gate has blocked.
func (tc *TestCluster) pinFunc(node *testNode) listener.PinFunc {
	base := tc.pki.PinFunc()

	return func(fingerprint string) listener.PinResult {
		res := base(fingerprint)
		if res.Found && !node.gate.reachable(res.NodeID) {
			res.Found = false
			res.NodeID = 0
		}

		return res
	}
}

// WireHandlers installs fn on every running node now, and again on a node
// after RestartNode. alpns names the protocols fn registers; StopNode
// deregisters them, so a stopped node's still-bound cluster port stops
// serving handlers that close over the Manager it just shut down.
func (tc *TestCluster) WireHandlers(alpns []string, fn ClusterWiring) {
	tc.t.Helper()

	tc.wirings = append(tc.wirings, clusterWiring{alpns: alpns, install: fn})

	for i, node := range tc.nodes {
		if node.running {
			fn(i, node.mgr, node.ln)
		}
	}
}

func (tc *TestCluster) startListener(node *testNode) {
	tc.t.Helper()

	if err := node.ln.Start(tc.ctx); err != nil {
		tc.t.Fatalf("start cluster listener for node %d: %v", node.nodeID, err)
	}
}

func (tc *TestCluster) refresh() {
	tc.Nodes = tc.Nodes[:0]
	tc.Listeners = tc.Listeners[:0]
	tc.Appliers = tc.Appliers[:0]

	for _, node := range tc.nodes {
		tc.Nodes = append(tc.Nodes, node.mgr)
		tc.Listeners = append(tc.Listeners, node.ln)
		tc.Appliers = append(tc.Appliers, node.applier)
	}
}

// Leader returns the current leader node, or nil if none.
func (tc *TestCluster) Leader() *Manager {
	for _, node := range tc.nodes {
		if node.running && node.mgr.IsLeader() {
			return node.mgr
		}
	}

	return nil
}

func (tc *TestCluster) LeaderIndex() int {
	for i, node := range tc.nodes {
		if node.running && node.mgr.IsLeader() {
			return i
		}
	}

	return -1
}

func (tc *TestCluster) WaitForLeader(timeout time.Duration) *Manager {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		if m := tc.Leader(); m != nil {
			return m
		}

		time.Sleep(5 * time.Millisecond)
	}

	return nil
}

// StopNode shuts a node's Manager down. The listener stays bound so the
// node keeps its port across a restart, but every handler wired to the
// shut-down Manager is deregistered along with the Manager itself.
func (tc *TestCluster) StopNode(idx int) {
	tc.t.Helper()

	node := tc.nodes[idx]
	if !node.running {
		return
	}

	if err := node.mgr.Shutdown(); err != nil && !errors.Is(err, hraft.ErrRaftShutdown) {
		tc.t.Errorf("shutdown node %d: %v", node.nodeID, err)
	}

	for _, w := range tc.wirings {
		for _, alpn := range w.alpns {
			node.ln.Deregister(alpn)
		}
	}

	node.running = false
}

func (tc *TestCluster) RestartNode(idx int) {
	tc.t.Helper()

	tc.StopNode(idx)
	tc.startNode(tc.nodes[idx])
	tc.refresh()

	for _, w := range tc.wirings {
		w.install(idx, tc.nodes[idx].mgr, tc.nodes[idx].ln)
	}
}

func (tc *TestCluster) Partition(far []int) {
	tc.t.Helper()

	isFar := make(map[int]bool, len(far))
	for _, idx := range far {
		isFar[idx] = true
	}

	for i, a := range tc.nodes {
		for j, b := range tc.nodes {
			if i == j || isFar[i] == isFar[j] {
				continue
			}

			a.gate.block(b.nodeID)
		}
	}

	tc.dropPooledConnections()
	tc.closeGatedConnections()
}

// closeGatedConnections tears down the mTLS sessions that span the
// partition. A handshake that completed before the gate closed is never
// re-verified, so those connections have to be closed the way a pin
// removal closes them.
func (tc *TestCluster) closeGatedConnections() {
	for _, node := range tc.nodes {
		for _, peer := range tc.nodes {
			if peer.nodeID == node.nodeID || node.gate.reachable(peer.nodeID) {
				continue
			}

			node.ln.CloseByPeerFingerprint(pki.Fingerprint(tc.pki.Nodes[peer.nodeID].Cert))
		}
	}
}

func (tc *TestCluster) dropPooledConnections() {
	for _, node := range tc.nodes {
		if !node.running {
			continue
		}

		if nt, ok := node.mgr.transport.(*hraft.NetworkTransport); ok {
			nt.CloseStreams()
		}

		if node.mgr.leaderClient != nil {
			node.mgr.leaderClient.close()
		}
	}
}

func (tc *TestCluster) Isolate(idx int) {
	tc.t.Helper()

	tc.Partition([]int{idx})
}

func (tc *TestCluster) Heal() {
	tc.t.Helper()

	for _, node := range tc.nodes {
		node.gate.reset()
	}

	tc.dropPooledConnections()
}

// WaitForConvergence polls until every running node's AppliedIndex reaches at
// least minIndex. Returns an error if the timeout expires first.
func (tc *TestCluster) WaitForConvergence(minIndex uint64, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		converged := true

		for _, node := range tc.nodes {
			if node.running && node.mgr.AppliedIndex() < minIndex {
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
		for i := range tc.nodes {
			tc.StopNode(i)
		}

		for _, node := range tc.nodes {
			if node.ln != nil {
				node.ln.Stop()
			}
		}

		tc.cancel()
	})
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

// reservePorts opens n loopback listeners simultaneously so the OS assigns n
// distinct ephemeral ports, and returns those ports with a release func that
// closes the probe sockets. Keeping every probe open until release prevents
// the OS from reusing a just-freed port for another node of the same cluster;
// callers must release() immediately before binding the real listeners.
func reservePorts(t testing.TB, n int) ([]int, func()) {
	t.Helper()

	lc := net.ListenConfig{}
	probes := make([]net.Listener, 0, n)
	ports := make([]int, 0, n)

	for range n {
		l, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
		if err != nil {
			for _, p := range probes {
				_ = p.Close()
			}

			t.Fatalf("reserve free port: %v", err)
		}

		probes = append(probes, l)
		ports = append(ports, l.Addr().(*net.TCPAddr).Port)
	}

	return ports, func() {
		for _, p := range probes {
			_ = p.Close()
		}
	}
}
