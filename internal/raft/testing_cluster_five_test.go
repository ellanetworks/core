// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package raft

import (
	"errors"
	"testing"
	"time"

	hraft "github.com/hashicorp/raft"
)

// TestFiveNodeClusterToleratesTwoFailures is the P1 promise at N=5: five nodes
// tolerate two failures. Quorum is 3, so writes must survive losing two voters
// and must stop on the third — the boundary is what makes the promise
// meaningful, since a cluster that kept accepting writes at 2-of-5 would be
// acknowledging data it can lose.
//
// Deliberately a unit test. The properties here are decided by the Raft
// configuration alone, so a fifth container would add CI minutes and a flake
// surface without testing anything this does not.
func TestFiveNodeClusterToleratesTwoFailures(t *testing.T) {
	tc := SetupTestClusterWithAppliers(t, 5, func() Applier {
		a := newTestApplier(t)
		a.writeRows = true

		return a
	})

	if got := len(tc.Nodes); got != 5 {
		t.Fatalf("cluster size = %d, want 5", got)
	}

	leader := tc.Leader()
	if leader == nil {
		t.Fatal("no leader in a healthy 5-node cluster")
	}

	mustPropose(t, leader, "all-five-up")

	// Stop two non-leader voters: 3 of 5 remain, exactly quorum.
	stopped := stopFollowers(t, tc, 2)

	leader = tc.WaitForLeader(10 * time.Second)
	if leader == nil {
		t.Fatal("no leader with 3 of 5 voters up; quorum of 3 should hold")
	}

	mustPropose(t, leader, "three-of-five")

	// Stop a third: 2 of 5 remain, below quorum. Writes must stop.
	stopFollowersExcluding(t, tc, stopped, 1)

	cmd, err := NewCommand(CmdChangeset, map[string]string{"phase": "two-of-five"})
	if err != nil {
		t.Fatalf("new command: %v", err)
	}

	_, err = leader.Propose(cmd, 2*time.Second)
	if err == nil {
		t.Fatal("Propose succeeded with 2 of 5 voters; a write acknowledged below quorum can be lost")
	}

	switch {
	case errors.Is(err, hraft.ErrLeadershipLost),
		errors.Is(err, hraft.ErrNotLeader),
		errors.Is(err, hraft.ErrEnqueueTimeout),
		errors.Is(err, hraft.ErrRaftShutdown):
	default:
		t.Fatalf("Propose below quorum returned %v (%T); want a raft commit failure", err, err)
	}
}

// TestFiveNodeClusterRecoversWhenVotersReturn pins the other half of P1: the
// stall is temporary. Once a stopped voter comes back, quorum is restored and
// writes resume without operator intervention.
func TestFiveNodeClusterRecoversWhenVotersReturn(t *testing.T) {
	tc := SetupTestClusterWithAppliers(t, 5, func() Applier {
		a := newTestApplier(t)
		a.writeRows = true

		return a
	})

	if tc.Leader() == nil {
		t.Fatal("no leader in a healthy 5-node cluster")
	}

	// Drop to 2 of 5 by stopping three followers.
	stopped := stopFollowers(t, tc, 3)

	// Bring one back: 3 of 5, quorum restored.
	tc.RestartNode(stopped[0])

	leader := tc.WaitForLeader(20 * time.Second)
	if leader == nil {
		t.Fatal("no leader after a voter returned; quorum of 3 should be restored")
	}

	mustPropose(t, leader, "after-recovery")
}

// stopFollowers stops n non-leader nodes and returns their indices.
func stopFollowers(t *testing.T, tc *TestCluster, n int) []int {
	t.Helper()

	return stopFollowersExcluding(t, tc, nil, n)
}

// stopFollowersExcluding stops n non-leader nodes, skipping any already in
// skip, and returns the full set of stopped indices.
func stopFollowersExcluding(t *testing.T, tc *TestCluster, skip []int, n int) []int {
	t.Helper()

	skipped := make(map[int]bool, len(skip))
	for _, i := range skip {
		skipped[i] = true
	}

	leaderIdx := tc.LeaderIndex()

	stopped := append([]int(nil), skip...)

	for i := range tc.Nodes {
		if n == 0 {
			break
		}

		if i == leaderIdx || skipped[i] {
			continue
		}

		tc.StopNode(i)

		stopped = append(stopped, i)
		n--
	}

	if n != 0 {
		t.Fatalf("could not stop enough followers; %d still needed", n)
	}

	return stopped
}

func mustPropose(t *testing.T, m *Manager, phase string) {
	t.Helper()

	cmd, err := NewCommand(CmdChangeset, map[string]string{"phase": phase})
	if err != nil {
		t.Fatalf("new command (%s): %v", phase, err)
	}

	result, err := m.Propose(cmd, 10*time.Second)
	if err != nil {
		t.Fatalf("propose (%s): %v", phase, err)
	}

	if result.Index == 0 {
		t.Fatalf("propose (%s) returned index 0", phase)
	}
}
