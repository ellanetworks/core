// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package raft

import (
	"errors"
	"testing"
	"time"
)

func TestLeadershipTransferToWithoutCandidates(t *testing.T) {
	m := &Manager{}

	for _, tc := range []struct {
		name       string
		candidates []Server
	}{
		{"nil", nil},
		{"empty", []Server{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := m.LeadershipTransferTo(tc.candidates)
			if !errors.Is(err, ErrNoTransferTarget) {
				t.Fatalf("err = %v, want ErrNoTransferTarget", err)
			}
		})
	}
}

func TestLeadershipTransferToHonoursTheNamedTarget(t *testing.T) {
	applier := newTestApplier(t)
	tc := SetupTestCluster(t, 3, applier)

	leader := tc.Leader()
	if leader == nil {
		t.Fatal("no leader")
	}

	leaderIdx := tc.LeaderIndex()

	targetIdx := (leaderIdx + 1) % len(tc.Nodes)
	target := tc.Nodes[targetIdx]

	var candidate Server

	for _, srv := range leader.Servers() {
		if srv.NodeID == target.RaftID() {
			candidate = srv
			break
		}
	}

	if candidate.NodeID == "" {
		t.Fatalf("target %s is not in the leader's configuration", target.RaftID())
	}

	if err := leader.LeadershipTransferTo([]Server{candidate}); err != nil {
		t.Fatalf("transfer to %s: %v", candidate.NodeID, err)
	}

	deadline := time.After(5 * time.Second)

	for !target.LeaderObserver().IsLeader() {
		select {
		case <-deadline:
			t.Fatalf("leadership did not land on the named target %s", candidate.NodeID)
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}
