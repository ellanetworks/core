// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package raft

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/cluster/listener"
	hraft "github.com/hashicorp/raft"
)

func proposeAndIndex(t *testing.T, m *Manager, n int) uint64 {
	t.Helper()

	var last uint64

	for i := range n {
		res, err := m.Propose(&Command{Type: 1, Payload: []byte{byte(i)}}, 5*time.Second)
		if err != nil {
			t.Fatalf("propose %d: %v", i, err)
		}

		last = res.Index
	}

	return last
}

func waitForLeaderExcept(tc *TestCluster, excluded int, timeout time.Duration) int {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		if idx := tc.LeaderIndex(); idx >= 0 && idx != excluded {
			return idx
		}

		time.Sleep(5 * time.Millisecond)
	}

	return -1
}

func TestCluster_ConstructsHAWiring(t *testing.T) {
	tc := SetupTestCluster(t, 3, newTestApplier(t))

	for i, m := range tc.Nodes {
		if m.followerTracker == nil {
			t.Errorf("node %d: followerTracker not constructed", i)
		}

		if m.autopilot == nil {
			t.Errorf("node %d: autopilot not constructed", i)
		}
	}

	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if leader := tc.Leader(); leader != nil {
			if st := leader.autopilot.State(); st != nil && len(st.Servers) == 3 {
				return
			}
		}

		time.Sleep(20 * time.Millisecond)
	}

	t.Fatal("autopilot never reported a 3-server state")
}

func TestCluster_FollowerTrackerMarksStoppedPeerUnhealthy(t *testing.T) {
	tc := SetupTestClusterWithAppliers(t, 3, func() Applier { return newTestApplier(t) })

	leaderIdx := tc.LeaderIndex()
	if leaderIdx < 0 {
		t.Fatal("no leader")
	}

	leader := tc.Nodes[leaderIdx]
	followerIdx := (leaderIdx + 1) % 3
	peerID := hraft.ServerID(strconv.Itoa(tc.nodes[followerIdx].nodeID))

	proposeAndIndex(t, leader, 3)
	tc.StopNode(followerIdx)

	ft := leader.followerTracker

	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		ft.mu.RLock()
		state, known := ft.peers[peerID]

		unhealthy := known && !state.healthy

		ft.mu.RUnlock()

		if unhealthy {
			return
		}

		time.Sleep(20 * time.Millisecond)
	}

	ft.mu.RLock()
	tracked := len(ft.peers)
	ft.mu.RUnlock()

	t.Fatalf("leader never marked stopped peer %s unhealthy (tracked peers: %d)", peerID, tracked)
}

func TestCluster_RestartNodeRetainsState(t *testing.T) {
	tc := SetupTestClusterWithAppliers(t, 3, func() Applier { return newTestApplier(t) })

	leaderIdx := tc.LeaderIndex()
	if leaderIdx < 0 {
		t.Fatal("no leader")
	}

	proposeAndIndex(t, tc.Nodes[leaderIdx], 3)

	followerIdx := (leaderIdx + 1) % 3
	dataDir := tc.nodes[followerIdx].dataDir

	tc.StopNode(followerIdx)

	target := proposeAndIndex(t, tc.Nodes[leaderIdx], 5)

	tc.RestartNode(followerIdx)

	if tc.nodes[followerIdx].dataDir != dataDir {
		t.Fatalf("restart changed data directory: %q -> %q", dataDir, tc.nodes[followerIdx].dataDir)
	}

	if err := tc.WaitForConvergence(target, 15*time.Second); err != nil {
		t.Fatalf("restarted node did not catch up: %v", err)
	}
}

func TestCluster_IsolatedLeaderStepsDown(t *testing.T) {
	tc := SetupTestClusterWithAppliers(t, 3, func() Applier { return newTestApplier(t) })

	oldIdx := tc.LeaderIndex()
	if oldIdx < 0 {
		t.Fatal("no leader")
	}

	proposeAndIndex(t, tc.Nodes[oldIdx], 3)

	tc.Isolate(oldIdx)

	newIdx := waitForLeaderExcept(tc, oldIdx, 15*time.Second)
	if newIdx < 0 {
		t.Fatal("majority side never elected a new leader")
	}

	if _, err := tc.Nodes[oldIdx].Propose(&Command{Type: 1, Payload: []byte("isolated")}, 2*time.Second); err == nil {
		t.Fatal("isolated former leader accepted a write")
	}

	target := proposeAndIndex(t, tc.Nodes[newIdx], 3)

	tc.Heal()

	if err := tc.WaitForConvergence(target, 15*time.Second); err != nil {
		t.Fatalf("cluster did not converge after heal: %v", err)
	}
}

// dialPeer opens an mTLS cluster connection from one test node to another
// and closes it again, reporting whether the connection was established.
func dialPeer(t *testing.T, tc *TestCluster, from, to int) error {
	t.Helper()

	target := tc.nodes[to]

	conn, err := tc.nodes[from].ln.Dial(t.Context(), target.addr, target.nodeID, listener.ALPNHTTP, 2*time.Second)
	if err != nil {
		return err
	}

	return conn.Close()
}

func TestCluster_PartitionBlocksBothDirections(t *testing.T) {
	tc := SetupTestClusterWithAppliers(t, 3, func() Applier { return newTestApplier(t) })

	tc.Isolate(0)

	if err := dialPeer(t, tc, 0, 1); err == nil {
		t.Error("isolated node still reached a peer")
	}

	if err := dialPeer(t, tc, 1, 0); err == nil {
		t.Error("peer still reached the isolated node")
	}

	if err := dialPeer(t, tc, 1, 2); err != nil {
		t.Errorf("partition blocked two nodes on the same side: %v", err)
	}

	tc.Heal()

	if err := dialPeer(t, tc, 0, 1); err != nil {
		t.Errorf("heal did not clear the partition: %v", err)
	}
}

func waitForMembers(m *Manager, want int, timeout time.Duration) []int {
	deadline := time.Now().Add(timeout)

	var ids []int

	for time.Now().Before(deadline) {
		ids = m.MemberIDs()
		if len(ids) == want {
			return ids
		}

		time.Sleep(20 * time.Millisecond)
	}

	return ids
}

func waitForUnhealthyPeer(t *testing.T, leader *Manager, peerID hraft.ServerID, timeout time.Duration) {
	t.Helper()

	ft := leader.followerTracker
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		ft.mu.RLock()
		state, known := ft.peers[peerID]

		unhealthy := known && !state.healthy

		ft.mu.RUnlock()

		if unhealthy {
			return
		}

		time.Sleep(20 * time.Millisecond)
	}

	t.Fatalf("leader never marked peer %s unhealthy", peerID)
}

func TestCluster_RemoveStoppedNode(t *testing.T) {
	tc := SetupTestClusterWithAppliers(t, 3, func() Applier { return newTestApplier(t) })

	leaderIdx := tc.LeaderIndex()
	if leaderIdx < 0 {
		t.Fatal("no leader")
	}

	leader := tc.Nodes[leaderIdx]
	followerIdx := (leaderIdx + 1) % 3
	stoppedID := tc.nodes[followerIdx].nodeID

	proposeAndIndex(t, leader, 3)
	tc.StopNode(followerIdx)

	if err := leader.RemoveServer(stoppedID); err != nil {
		t.Fatalf("remove stopped node %d: %v", stoppedID, err)
	}

	ids := waitForMembers(leader, 2, 10*time.Second)
	for _, id := range ids {
		if id == stoppedID {
			t.Fatalf("stopped node %d still in configuration %v", stoppedID, ids)
		}
	}

	if _, err := leader.Propose(&Command{Type: 1, Payload: []byte("after-removal")}, 5*time.Second); err != nil {
		t.Fatalf("write after removing stopped node: %v", err)
	}
}

// goroutineDump returns a full stack dump, growing the buffer until it
// fits. A truncated dump silently loses frames, which would turn a leaked
// goroutine into a passing count.
func goroutineDump(t *testing.T) string {
	t.Helper()

	for size := 1 << 20; ; size *= 2 {
		buf := make([]byte, size)
		if n := runtime.Stack(buf, true); n < size {
			return string(buf[:n])
		}

		if size >= 1<<27 {
			t.Fatalf("goroutine dump exceeds %d bytes", size)
		}
	}
}

// TestCluster_StoppedPeerStaysInConfiguration guards the property that
// membership changes are operator-driven: a node that stops answering is
// reported unhealthy but keeps its place in the Raft configuration, so a
// reboot or an upgrade never costs the cluster a member.
func TestCluster_StoppedPeerStaysInConfiguration(t *testing.T) {
	tc := SetupTestClusterWithAppliers(t, 3, func() Applier { return newTestApplier(t) })

	leaderIdx := tc.LeaderIndex()
	if leaderIdx < 0 {
		t.Fatal("no leader")
	}

	leader := tc.Nodes[leaderIdx]
	followerIdx := (leaderIdx + 1) % 3
	stoppedID := tc.nodes[followerIdx].nodeID
	peerID := hraft.ServerID(strconv.Itoa(stoppedID))

	proposeAndIndex(t, leader, 3)
	tc.StopNode(followerIdx)
	waitForUnhealthyPeer(t, leader, peerID, 15*time.Second)

	// Autopilot reconciles every 100 ms in this harness, so a few seconds
	// covers many passes.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if ids := leader.MemberIDs(); len(ids) != 3 {
			t.Fatalf("node %d left the configuration after a brief outage; configuration is %v", stoppedID, ids)
		}

		time.Sleep(50 * time.Millisecond)
	}
}

func countGoroutines(t *testing.T, needles ...string) int {
	t.Helper()

	count := 0

	for _, frame := range strings.Split(goroutineDump(t), "\n\n") {
		for _, needle := range needles {
			if strings.Contains(frame, needle) {
				count++
				break
			}
		}
	}

	return count
}

func TestCluster_ShutdownStopsLeaderRoutines(t *testing.T) {
	// The count is process-global, so the assertion is against the count
	// taken before this cluster starts rather than against zero.
	const autopilotRoutine, trackerRoutine = "raft-autopilot", "followerTracker).run"

	baseline := countGoroutines(t, autopilotRoutine, trackerRoutine)

	tc := SetupTestClusterWithAppliers(t, 3, func() Applier { return newTestApplier(t) })

	if tc.LeaderIndex() < 0 {
		t.Fatal("no leader")
	}

	proposeAndIndex(t, tc.Nodes[tc.LeaderIndex()], 3)

	if running := countGoroutines(t, autopilotRoutine, trackerRoutine); running <= baseline {
		t.Fatalf("leader routines = %d, want more than the %d running before the cluster started", running, baseline)
	}

	for i := range tc.nodes {
		tc.StopNode(i)
	}

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if countGoroutines(t, autopilotRoutine, trackerRoutine) <= baseline {
			return
		}

		time.Sleep(50 * time.Millisecond)
	}

	t.Fatalf("leader routines after shutdown = %d, want at most the %d running before the cluster started",
		countGoroutines(t, autopilotRoutine, trackerRoutine), baseline)
}

func TestCluster_PartitionBlocksClusterHTTP(t *testing.T) {
	tc := SetupTestClusterWithAppliers(t, 3, func() Applier { return newTestApplier(t) })
	wireClusterProposeHandlers(t, tc)

	leaderIdx := tc.LeaderIndex()
	if leaderIdx < 0 {
		t.Fatal("no leader")
	}

	followerIdx := (leaderIdx + 1) % 3
	follower := tc.Nodes[followerIdx]

	payload, err := json.Marshal(map[string]string{"via": "follower"})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	if _, err := follower.ForwardOperation(t.Context(), "TestOp", payload, 5*time.Second); err != nil {
		t.Fatalf("forward before partition: %v", err)
	}

	tc.Isolate(followerIdx)

	if _, err := follower.ForwardOperation(t.Context(), "TestOp", payload, 2*time.Second); err == nil {
		t.Fatal("partitioned follower forwarded a write to the leader over cluster HTTP")
	}
}

// postProposeTo sends a propose envelope over the cluster port to one
// specific node, so the assertion is about that node's ALPN handler rather
// than about wherever the cluster happens to have put leadership.
func postProposeTo(t *testing.T, tc *TestCluster, from, to int, payload []byte) (int, error) {
	t.Helper()

	envelope, err := json.Marshal(ProposeForwardRequest{Operation: "TestOp", Payload: payload})
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}

	target := tc.nodes[to]

	resp, err := tc.nodes[from].mgr.clusterHTTPDo(
		t.Context(), http.MethodPost, target.addr, target.nodeID, ProposeForwardPath, bytes.NewReader(envelope),
	)
	if err != nil {
		return 0, err
	}

	defer func() { _ = resp.Body.Close() }()

	_, _ = io.Copy(io.Discard, resp.Body)

	return resp.StatusCode, nil
}

func TestCluster_RestartKeepsNonRaftALPNHandlers(t *testing.T) {
	tc := SetupTestClusterWithAppliers(t, 3, func() Applier { return newTestApplier(t) })
	wireClusterProposeHandlers(t, tc)

	leaderIdx := tc.LeaderIndex()
	if leaderIdx < 0 {
		t.Fatal("no leader")
	}

	restartedIdx := (leaderIdx + 1) % 3
	peerIdx := (leaderIdx + 2) % 3

	payload, err := json.Marshal(map[string]string{"via": "peer"})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	tc.RestartNode(restartedIdx)

	if tc.WaitForLeader(15*time.Second) == nil {
		t.Fatal("no leader after restart")
	}

	// A served status — 200 from a restarted node that took leadership, 421
	// from one that did not — proves the propose handler is registered and
	// bound to the Manager the restart created. A transport error means the
	// listener dropped the protocol with the old Manager.
	var status int

	deadline := time.Now().Add(15 * time.Second)

	for {
		status, err = postProposeTo(t, tc, peerIdx, restartedIdx, payload)
		if err == nil || time.Now().After(deadline) {
			break
		}

		time.Sleep(50 * time.Millisecond)
	}

	if err != nil {
		t.Fatalf("restarted node did not serve cluster HTTP: %v", err)
	}

	if status != http.StatusOK && status != http.StatusMisdirectedRequest {
		t.Fatalf("restarted node answered propose with %d, want %d or %d",
			status, http.StatusOK, http.StatusMisdirectedRequest)
	}
}

func TestCluster_StopNodeDropsNonRaftALPNHandlers(t *testing.T) {
	tc := SetupTestClusterWithAppliers(t, 3, func() Applier { return newTestApplier(t) })
	wireClusterProposeHandlers(t, tc)

	leaderIdx := tc.LeaderIndex()
	if leaderIdx < 0 {
		t.Fatal("no leader")
	}

	stoppedIdx := (leaderIdx + 1) % 3
	peerIdx := (leaderIdx + 2) % 3

	payload, err := json.Marshal(map[string]string{"via": "peer"})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	if _, err := postProposeTo(t, tc, peerIdx, stoppedIdx, payload); err != nil {
		t.Fatalf("propose to running node: %v", err)
	}

	tc.StopNode(stoppedIdx)

	if status, err := postProposeTo(t, tc, peerIdx, stoppedIdx, payload); err == nil {
		t.Fatalf("stopped node answered propose with %d, want a transport error", status)
	}
}
