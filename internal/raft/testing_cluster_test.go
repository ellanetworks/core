// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package raft

import (
	"context"
	"database/sql"
	"errors"
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
	appliers := make([]*testApplier, 0, 3)

	tc := SetupTestClusterWithAppliers(t, 3, func() Applier {
		a := newTestApplier(t)
		a.writeRows = true
		appliers = append(appliers, a)

		return a
	})

	leader := tc.Leader()
	if leader == nil {
		t.Fatal("no leader")
	}

	first, err := NewCommand(CmdChangeset, map[string]string{"seq": "first"})
	if err != nil {
		t.Fatalf("new command: %v", err)
	}

	firstResult, err := leader.Propose(first, 5*time.Second)
	if err != nil {
		t.Fatalf("propose first: %v", err)
	}

	if firstResult.Index == 0 {
		t.Fatal("propose returned index 0; a committed entry always has a non-zero log index")
	}

	second, err := NewCommand(CmdChangeset, map[string]string{"seq": "second"})
	if err != nil {
		t.Fatalf("new command: %v", err)
	}

	secondResult, err := leader.Propose(second, 5*time.Second)
	if err != nil {
		t.Fatalf("propose second: %v", err)
	}

	if secondResult.Index <= firstResult.Index {
		t.Fatalf("log index did not advance: first=%d second=%d",
			firstResult.Index, secondResult.Index)
	}

	// Every node applies every committed entry, so the commands must reach
	// all three FSMs, not just the leader's.
	waitForAppliedIndex(t, tc, secondResult.Index, 10*time.Second)

	for i, a := range appliers {
		a.mu.Lock()
		got := len(a.commands)
		a.mu.Unlock()

		if got < 2 {
			t.Errorf("node %d applier saw %d commands, want at least 2", i+1, got)
		}
	}
}

// waitForAppliedIndex blocks until every node in tc reports an applied index
// of at least want.
func waitForAppliedIndex(t *testing.T, tc *TestCluster, want uint64, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)

	for _, n := range tc.Nodes {
		for n.AppliedIndex() < want {
			if time.Now().After(deadline) {
				t.Fatalf("node did not reach applied index %d within %s (stuck at %d)",
					want, timeout, n.AppliedIndex())
			}

			time.Sleep(10 * time.Millisecond)
		}
	}
}

// TestSetupTestCluster_LeaderFailover shuts down the leader, verifies that a
// new leader is elected among the remaining nodes, and that the new leader can
// accept writes.
func TestSetupTestCluster_LeaderFailover(t *testing.T) {
	applier := newTestApplier(t)
	tc := SetupTestCluster(t, 3, applier)

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

	// Shut down the leader to simulate node failure. This closes the
	// transport, breaking all Raft connections to followers.
	if err := leader.Shutdown(); err != nil {
		t.Fatalf("shutdown leader: %v", err)
	}

	tc.Listeners[leaderIdx].Stop()

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

			// Gate on the LeaderObserver rather than Manager.IsLeader() because
			// the observer is the signal downstream subscribers (and the
			// assertion below) actually see. Manager.IsLeader() reads
			// raft.State() directly and flips before the observer goroutine
			// has drained raft.LeaderCh(), which races this test.
			if n.LeaderObserver().IsLeader() {
				newLeader = n
				break
			}
		}
	}

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

	ids := leader.MemberIDs()
	if len(ids) != 2 {
		t.Fatalf("expected 2 members after removal, got %d: %v", len(ids), ids)
	}

	for _, id := range ids {
		if id == removeNodeID {
			t.Fatalf("removed node %d still in MemberIDs", removeNodeID)
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

	ids = leader.MemberIDs()
	if len(ids) != 3 {
		t.Fatalf("expected 3 members after re-add, got %d: %v", len(ids), ids)
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

// TestSetupTestCluster_TwoVoterQuorumLoss confirms that a 2-voter
// cluster halts writes when one voter is killed. Quorum is 2, so
// the survivor cannot commit any new entry on its own; Propose
// must return an error within the configured timeout rather than
// silently accepting writes.
func TestSetupTestCluster_TwoVoterQuorumLoss(t *testing.T) {
	applier := newTestApplier(t)
	tc := SetupTestCluster(t, 2, applier)

	leader := tc.Leader()
	if leader == nil {
		t.Fatal("no leader")
	}

	// Sanity: the healthy 2-voter cluster commits.
	cmd, err := NewCommand(CmdChangeset, map[string]string{"phase": "before-loss"})
	if err != nil {
		t.Fatalf("new command: %v", err)
	}

	if _, err := leader.Propose(cmd, 5*time.Second); err != nil {
		t.Fatalf("propose before partition: %v", err)
	}

	// Identify and kill the follower.
	var followerIdx int

	for i, n := range tc.Nodes {
		if n != leader {
			followerIdx = i
			break
		}
	}

	if err := tc.Nodes[followerIdx].Shutdown(); err != nil {
		t.Fatalf("shutdown follower: %v", err)
	}

	tc.Listeners[followerIdx].Stop()

	// Propose with a tight timeout. The leader cannot replicate to
	// the dead follower; Propose must fail (timeout, leadership
	// lost, or enqueue timeout — all acceptable).
	cmd, err = NewCommand(CmdChangeset, map[string]string{"phase": "after-loss"})
	if err != nil {
		t.Fatalf("new command: %v", err)
	}

	_, err = leader.Propose(cmd, 1*time.Second)
	if err == nil {
		t.Fatal("Propose should fail without a quorum; got nil error")
	}

	// The error must be one hraft raises when an entry cannot be committed,
	// not an incidental failure such as a marshalling error. internal/db
	// routes exactly these into ErrOutcomeUnknown / ErrProposeTimeout, which
	// is what turns a quorum-loss write into a retryable API response
	// instead of a generic 500.
	switch {
	case errors.Is(err, hraft.ErrLeadershipLost),
		errors.Is(err, hraft.ErrNotLeader),
		errors.Is(err, hraft.ErrEnqueueTimeout),
		errors.Is(err, hraft.ErrLeadershipTransferInProgress),
		errors.Is(err, hraft.ErrRaftShutdown):
	default:
		t.Fatalf("Propose without quorum returned %v (%T); want a raft commit failure "+
			"(leadership lost / not leader / enqueue timeout / shutdown)", err, err)
	}
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
