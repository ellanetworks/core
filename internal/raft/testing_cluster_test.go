// Copyright 2026 Ella Networks

package raft

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	hraft "github.com/hashicorp/raft"
)

func TestSetupTestCluster_ThreeNodes(t *testing.T) {
	applier := newTestApplier(t)

	tc := SetupTestCluster(t, 3, applier)

	if len(tc.Nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(tc.Nodes))
	}

	leader := tc.Leader()
	if leader == nil {
		t.Fatal("expected a leader")
	}

	followers := 0

	for _, n := range tc.Nodes {
		if !n.IsLeader() {
			followers++
		}
	}

	if followers != 2 {
		t.Fatalf("expected 2 followers, got %d", followers)
	}
}

func TestSetupTestCluster_LeaderPropose(t *testing.T) {
	applier := newTestApplier(t)

	tc := SetupTestCluster(t, 3, applier)

	leader := tc.Leader()
	if leader == nil {
		t.Fatal("no leader")
	}

	cmd := &Command{Type: 1, Payload: []byte("test")}
	if _, err := leader.Propose(cmd, 5*time.Second); err != nil {
		t.Fatalf("propose failed: %v", err)
	}
}

// TestSetupTestCluster_LeaderFailover disconnects the leader from the cluster,
// verifies that a new leader is elected among the remaining nodes, that the
// observer callbacks fire correctly, and that the new leader can accept writes.
func TestSetupTestCluster_LeaderFailover(t *testing.T) {
	applier := newTestApplier(t)
	tc := SetupTestCluster(t, 3, applier)

	// Register observer callbacks on all nodes.
	callbacks := make([]*testCallback, len(tc.Nodes))
	for i, n := range tc.Nodes {
		cb := &testCallback{}
		n.LeaderObserver().Register(cb)
		callbacks[i] = cb
	}

	// Wait for initial leader callbacks to settle.
	leader := tc.Leader()
	if leader == nil {
		t.Fatal("no leader")
	}

	var leaderIdx int

	for i, n := range tc.Nodes {
		if n == leader {
			leaderIdx = i
			break
		}
	}

	// Disconnect the leader from every other node.
	leaderTransport := leader.transport.(*hraft.InmemTransport)

	for i, n := range tc.Nodes {
		if i == leaderIdx {
			continue
		}

		peerTransport := n.transport.(*hraft.InmemTransport)
		leaderTransport.Disconnect(hraft.ServerAddress(n.RaftAddress()))
		peerTransport.Disconnect(hraft.ServerAddress(leader.RaftAddress()))
	}

	// Wait for a new leader among the survivors.
	deadline := time.After(5 * time.Second)

	var newLeader *Manager

	for newLeader == nil {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for new leader after partition")
		default:
			time.Sleep(10 * time.Millisecond)
		}

		for i, n := range tc.Nodes {
			if i == leaderIdx {
				continue
			}

			if n.IsLeader() {
				newLeader = n
				break
			}
		}
	}

	// The old leader should eventually step down (lose lease).
	deadline = time.After(5 * time.Second)

	for leader.IsLeader() {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for old leader to step down")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	// Verify observer callbacks: old leader lost leadership.
	deadline = time.After(2 * time.Second)

	for callbacks[leaderIdx].lostLeader.Load() < 1 {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for OnLostLeadership on old leader")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	// New leader's observer should report leadership.
	if !newLeader.LeaderObserver().IsLeader() {
		t.Fatal("new leader's observer does not report IsLeader()")
	}

	// Propose on the new leader.
	cmd, err := NewCommand(CmdChangeset, map[string]string{"after": "failover"})
	if err != nil {
		t.Fatalf("new command: %v", err)
	}

	if _, err := newLeader.Propose(cmd, 5*time.Second); err != nil {
		t.Fatalf("propose on new leader failed: %v", err)
	}
}

// TestSetupTestCluster_RemoveAndReaddServer removes a node from a 3-node
// cluster, verifies the 2-node cluster is functional, then re-adds the node
// and verifies it rejoins.
func TestSetupTestCluster_RemoveAndReaddServer(t *testing.T) {
	applier := newTestApplier(t)
	tc := SetupTestCluster(t, 3, applier)

	leader := tc.Leader()
	if leader == nil {
		t.Fatal("no leader")
	}

	// Identify a follower to remove (pick the last non-leader).
	var removeIdx int

	for i := len(tc.Nodes) - 1; i >= 0; i-- {
		if tc.Nodes[i] != leader {
			removeIdx = i
			break
		}
	}

	removeNode := tc.Nodes[removeIdx]
	removeNodeID := removeNode.NodeID()

	// Remove the node from the Raft configuration.
	if err := leader.RemoveServer(removeNodeID); err != nil {
		t.Fatalf("RemoveServer(%d): %v", removeNodeID, err)
	}

	// VoterIDs should now contain only 2 nodes.
	ids := leader.VoterIDs()
	if len(ids) != 2 {
		t.Fatalf("expected 2 voters after removal, got %d: %v", len(ids), ids)
	}

	for _, id := range ids {
		if id == removeNodeID {
			t.Fatalf("removed node %d still in VoterIDs", removeNodeID)
		}
	}

	// Propose on the 2-node cluster.
	cmd, err := NewCommand(CmdChangeset, map[string]string{"phase": "2-node"})
	if err != nil {
		t.Fatalf("new command: %v", err)
	}

	if _, err := leader.Propose(cmd, 5*time.Second); err != nil {
		t.Fatalf("propose on 2-node cluster: %v", err)
	}

	// Re-add the removed node.
	if err := leader.AddVoter(removeNodeID, removeNode.RaftAddress()); err != nil {
		t.Fatalf("AddVoter(%d): %v", removeNodeID, err)
	}

	ids = leader.VoterIDs()
	if len(ids) != 3 {
		t.Fatalf("expected 3 voters after re-add, got %d: %v", len(ids), ids)
	}

	// Propose on the restored 3-node cluster.
	cmd, err = NewCommand(CmdChangeset, map[string]string{"phase": "3-node-restored"})
	if err != nil {
		t.Fatalf("new command: %v", err)
	}

	if _, err := leader.Propose(cmd, 5*time.Second); err != nil {
		t.Fatalf("propose on restored 3-node cluster: %v", err)
	}
}

// TestSetupTestCluster_FSMConvergence verifies that after proposing commands
// through the leader, every node's SQLite database contains identical rows.
// Each node has its own testApplier (and SQLite file); Raft replication is
// the only way data reaches followers.
func TestSetupTestCluster_FSMConvergence(t *testing.T) {
	tc := SetupTestClusterWithAppliers(t, 3, func() Applier {
		a := newTestApplier(t)
		a.writeRows = true

		return a
	})

	leader := tc.Leader()
	if leader == nil {
		t.Fatal("no leader")
	}

	// Propose several commands through the leader.
	const numCommands = 10

	for i := range numCommands {
		cmd, err := NewCommand(CmdChangeset, map[string]string{
			"key": fmt.Sprintf("value-%d", i),
		})
		if err != nil {
			t.Fatalf("new command %d: %v", i, err)
		}

		if _, err := leader.Propose(cmd, 5*time.Second); err != nil {
			t.Fatalf("propose %d: %v", i, err)
		}
	}

	// Wait for all nodes to apply up to the leader's index.
	leaderIdx := leader.AppliedIndex()

	err := tc.WaitForConvergence(leaderIdx, 5*time.Second)
	if err != nil {
		t.Fatalf("convergence: %v", err)
	}

	// Read rows from each node's SQLite and compare.
	ctx := context.Background()

	var reference []string

	for i, a := range tc.Appliers {
		ta := a.(*testApplier)
		rows := queryAllRows(t, ctx, ta.db)

		if i == 0 {
			reference = rows
			continue
		}

		if len(rows) != len(reference) {
			t.Fatalf("node %d has %d rows, node 1 has %d", i+1, len(rows), len(reference))
		}

		for j := range rows {
			if rows[j] != reference[j] {
				t.Fatalf("node %d row %d differs: got %q, want %q", i+1, j, rows[j], reference[j])
			}
		}
	}

	if len(reference) != numCommands {
		t.Fatalf("expected %d rows, got %d", numCommands, len(reference))
	}

	t.Logf("all %d nodes have identical %d rows", len(tc.Nodes), len(reference))
}

// queryAllRows returns all rows from table t ordered by id.
func queryAllRows(t testing.TB, ctx context.Context, db *sql.DB) []string {
	t.Helper()

	rows, err := db.QueryContext(ctx, "SELECT v FROM t ORDER BY id")
	if err != nil {
		t.Fatalf("query rows: %v", err)
	}

	defer func() { _ = rows.Close() }()

	var result []string

	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			t.Fatalf("scan row: %v", err)
		}

		result = append(result, v)
	}

	if err := rows.Err(); err != nil {
		t.Fatalf("rows iteration: %v", err)
	}

	return result
}
