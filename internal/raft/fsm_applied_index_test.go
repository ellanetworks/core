// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package raft

import (
	"testing"
	"time"
)

func awaitAppliedIndex(t *testing.T, m *Manager, want uint64, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)

	for m.AppliedIndex() != want {
		if time.Now().After(deadline) {
			t.Fatalf("AppliedIndex() = %d, want %d", m.AppliedIndex(), want)
		}

		time.Sleep(20 * time.Millisecond)
	}
}

func TestAppliedIndexSurvivesRestartWithNothingToReplay(t *testing.T) {
	tc := SetupTestClusterWithAppliers(t, 3, func() Applier { return newTestApplier(t) })

	leaderIdx := tc.LeaderIndex()
	if leaderIdx < 0 {
		t.Fatal("no leader")
	}

	target := proposeAndIndex(t, tc.Nodes[leaderIdx], 5)

	if err := tc.WaitForConvergence(target, 15*time.Second); err != nil {
		t.Fatalf("cluster did not converge: %v", err)
	}

	followerIdx := (leaderIdx + 1) % 3

	if err := tc.Nodes[followerIdx].Snapshot(); err != nil {
		t.Fatalf("follower snapshot: %v", err)
	}

	tc.StopNode(followerIdx)
	tc.RestartNode(followerIdx)

	awaitAppliedIndex(t, tc.Nodes[followerIdx], target, 5*time.Second)
}

func TestAppliedIndexFollowsSnapshotInstall(t *testing.T) {
	tc := SetupTestClusterWithAppliers(t, 3, func() Applier { return newTestApplier(t) })

	leaderIdx := tc.LeaderIndex()
	if leaderIdx < 0 {
		t.Fatal("no leader")
	}

	followerIdx := (leaderIdx + 1) % 3

	proposeAndIndex(t, tc.Nodes[leaderIdx], 3)
	tc.StopNode(followerIdx)

	target := proposeAndIndex(t, tc.Nodes[leaderIdx], 250)

	if err := tc.Nodes[leaderIdx].Snapshot(); err != nil {
		t.Fatalf("leader snapshot: %v", err)
	}

	tc.RestartNode(followerIdx)

	awaitAppliedIndex(t, tc.Nodes[followerIdx], target, 15*time.Second)
}
