// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package raft

import (
	"errors"
	"testing"
	"time"

	hraft "github.com/hashicorp/raft"
)

const mustProposeTimeout = 30 * time.Second

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

	mustPropose(t, tc, "all-five-up")

	stopped := stopFollowers(t, tc, 2)

	leader = tc.WaitForLeader(10 * time.Second)
	if leader == nil {
		t.Fatal("no leader with 3 of 5 voters up; quorum of 3 should hold")
	}

	mustPropose(t, tc, "three-of-five")

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

func TestFiveNodeClusterRecoversWhenVotersReturn(t *testing.T) {
	tc := SetupTestClusterWithAppliers(t, 5, func() Applier {
		a := newTestApplier(t)
		a.writeRows = true

		return a
	})

	if tc.Leader() == nil {
		t.Fatal("no leader in a healthy 5-node cluster")
	}

	stopped := stopFollowers(t, tc, 3)

	tc.RestartNode(stopped[0])

	leader := tc.WaitForLeader(20 * time.Second)
	if leader == nil {
		t.Fatal("no leader after a voter returned; quorum of 3 should be restored")
	}

	mustPropose(t, tc, "after-recovery")
}

func stopFollowers(t *testing.T, tc *TestCluster, n int) []int {
	t.Helper()

	return stopFollowersExcluding(t, tc, nil, n)
}

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

func mustPropose(t *testing.T, tc *TestCluster, phase string) {
	t.Helper()

	cmd, err := NewCommand(CmdChangeset, map[string]string{"phase": phase})
	if err != nil {
		t.Fatalf("new command (%s): %v", phase, err)
	}

	deadline := time.Now().Add(mustProposeTimeout)
	lastErr := errors.New("no leader")

	for time.Now().Before(deadline) {
		leader := tc.WaitForLeader(time.Until(deadline))
		if leader == nil {
			break
		}

		result, proposeErr := leader.Propose(cmd, 10*time.Second)
		if proposeErr == nil {
			if result.Index == 0 {
				t.Fatalf("propose (%s) returned index 0", phase)
			}

			return
		}

		lastErr = proposeErr

		if !transientProposeError(proposeErr) {
			t.Fatalf("propose (%s): %v", phase, proposeErr)
		}

		time.Sleep(20 * time.Millisecond)
	}

	t.Fatalf("propose (%s) did not commit within %s; last error: %v", phase, mustProposeTimeout, lastErr)
}

func transientProposeError(err error) bool {
	return errors.Is(err, hraft.ErrLeadershipLost) ||
		errors.Is(err, hraft.ErrNotLeader) ||
		errors.Is(err, hraft.ErrLeadershipTransferInProgress) ||
		errors.Is(err, hraft.ErrEnqueueTimeout)
}
