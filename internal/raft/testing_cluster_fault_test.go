// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package raft

import (
	"encoding/json"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

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

func TestCluster_PartitionBlocksBothDirections(t *testing.T) {
	tc := SetupTestClusterWithAppliers(t, 3, func() Applier { return newTestApplier(t) })

	tc.Isolate(0)

	if tc.nodes[0].gate.reachable(0, tc.nodes[1].addr) {
		t.Error("isolated node can still dial a peer")
	}

	if tc.nodes[1].gate.reachable(tc.nodes[0].nodeID, "") {
		t.Error("peer still accepts connections from the isolated node")
	}

	if !tc.nodes[1].gate.reachable(0, tc.nodes[2].addr) {
		t.Error("partition blocked two nodes on the same side")
	}

	tc.Heal()

	if !tc.nodes[0].gate.reachable(0, tc.nodes[1].addr) {
		t.Error("heal did not clear the partition")
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

func TestCluster_AutopilotRemovesFailedServer(t *testing.T) {
	tc := SetupTestClusterWithAppliers(t, 3, func() Applier { return newTestApplier(t) })

	leaderIdx := tc.LeaderIndex()
	if leaderIdx < 0 {
		t.Fatal("no leader")
	}

	leader := tc.Nodes[leaderIdx]
	followerIdx := (leaderIdx + 1) % 3
	failedID := tc.nodes[followerIdx].nodeID
	peerID := hraft.ServerID(strconv.Itoa(failedID))

	proposeAndIndex(t, leader, 3)
	tc.StopNode(followerIdx)
	waitForUnhealthyPeer(t, leader, peerID, 15*time.Second)

	ids := waitForMembers(leader, 2, 30*time.Second)
	for _, id := range ids {
		if id == failedID {
			t.Fatalf("autopilot did not evict failed node %d; configuration is %v", failedID, ids)
		}
	}
}

func countGoroutines(t *testing.T, needles ...string) int {
	t.Helper()

	buf := make([]byte, 1<<20)
	buf = buf[:runtime.Stack(buf, true)]

	count := 0

	for _, frame := range strings.Split(string(buf), "\n\n") {
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
	tc := SetupTestClusterWithAppliers(t, 3, func() Applier { return newTestApplier(t) })

	if tc.LeaderIndex() < 0 {
		t.Fatal("no leader")
	}

	proposeAndIndex(t, tc.Nodes[tc.LeaderIndex()], 3)

	if countGoroutines(t, "raft-autopilot", "followerTracker).run") == 0 {
		t.Fatal("expected autopilot and follower-tracker goroutines while leader is running")
	}

	for i := range tc.nodes {
		tc.StopNode(i)
	}

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if countGoroutines(t, "raft-autopilot", "followerTracker).run") == 0 {
			return
		}

		time.Sleep(50 * time.Millisecond)
	}

	t.Fatalf("leader routines still running after shutdown: %d", countGoroutines(t, "raft-autopilot", "followerTracker).run"))
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

func TestCluster_RestartKeepsNonRaftALPNHandlers(t *testing.T) {
	tc := SetupTestClusterWithAppliers(t, 3, func() Applier { return newTestApplier(t) })
	wireClusterProposeHandlers(t, tc)

	leaderIdx := tc.LeaderIndex()
	if leaderIdx < 0 {
		t.Fatal("no leader")
	}

	followerIdx := (leaderIdx + 1) % 3

	payload, err := json.Marshal(map[string]string{"via": "follower"})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	tc.RestartNode(followerIdx)

	if tc.WaitForLeader(15*time.Second) == nil {
		t.Fatal("no leader after restart")
	}

	deadline := time.Now().Add(15 * time.Second)
	for {
		_, err = tc.Nodes[followerIdx].ForwardOperation(t.Context(), "TestOp", payload, 5*time.Second)
		if err == nil || time.Now().After(deadline) {
			break
		}

		time.Sleep(50 * time.Millisecond)
	}

	if err != nil {
		t.Fatalf("cluster HTTP forwarding broken after restart: %v", err)
	}
}
