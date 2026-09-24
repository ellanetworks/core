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
	removeNodeID := removeNode.RaftID()

	// Remove the node from the Raft configuration.
	if err := leader.RemoveServer(removeNodeID); err != nil {
		t.Fatalf("RemoveServer(%s): %v", removeNodeID, err)
	}

	ids := leader.MemberIDs()
	if len(ids) != 2 {
		t.Fatalf("expected 2 members after removal, got %d: %v", len(ids), ids)
	}

	for _, id := range ids {
		if id == removeNodeID {
			t.Fatalf("removed node %s still in MemberIDs", removeNodeID)
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
		t.Fatalf("AddVoter(%s): %v", removeNodeID, err)
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

	var lastIndex uint64

	for i := range numCommands {
		cmd, err := NewCommand(CmdChangeset, map[string]string{
			"key": fmt.Sprintf("value-%d", i),
		})
		if err != nil {
			t.Fatalf("new command %d: %v", i, err)
		}

		result, err := leader.Propose(cmd, 5*time.Second)
		if err != nil {
			t.Fatalf("propose %d: %v", i, err)
		}

		if result.Index <= lastIndex {
			t.Fatalf("propose %d returned index %d: want an index above %d", i, result.Index, lastIndex)
		}

		lastIndex = result.Index
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
