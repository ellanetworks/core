// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package raft

import (
	"errors"
	"testing"
	"time"

	"github.com/hashicorp/raft"
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

	for !target.leadershipEstablished() {
		select {
		case <-deadline:
			t.Fatalf("leadership did not land on the named target %s", candidate.NodeID)
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}

type errFuture struct{ err error }

func (f errFuture) Error() error { return f.err }

var errTransferTimeout = errors.New("leadership transfer timeout")

func transferSetup(t *testing.T) (*Manager, Server, Server) {
	t.Helper()

	tc := SetupTestCluster(t, 3, newTestApplier(t))

	leader := tc.Leader()
	if leader == nil {
		t.Fatal("no leader")
	}

	var peers []Server

	for _, srv := range leader.Servers() {
		if srv.NodeID != leader.RaftID() {
			peers = append(peers, srv)
		}
	}

	if len(peers) != 2 {
		t.Fatalf("peers %+v, want 2", peers)
	}

	return leader, peers[0], peers[1]
}

func timedOutAfterMovingTo(t *testing.T, m *Manager, moveTo Server) func(*Server) raft.Future {
	calls := 0

	return func(target *Server) raft.Future {
		calls++
		if calls > 1 {
			t.Errorf("transfer attempt %d issued after leadership already moved", calls)
			return errFuture{raft.ErrNotLeader}
		}

		if err := m.raft.LeadershipTransferToServer(raft.ServerID(moveTo.NodeID), raft.ServerAddress(moveTo.Address)).Error(); err != nil {
			t.Fatalf("move leadership to %s: %v", moveTo.NodeID, err)
		}

		return errFuture{errTransferTimeout}
	}
}

func TestLeadershipTransferThatTimesOutButLandsOnTheTargetSucceeds(t *testing.T) {
	leader, target, _ := transferSetup(t)
	leader.transferStarter = timedOutAfterMovingTo(t, leader, target)

	if err := leader.LeadershipTransferTo([]Server{target}); err != nil {
		t.Fatalf("transfer to %s: %v", target.NodeID, err)
	}
}

func TestLeadershipTransferThatLandsOutsideTheTargetsFails(t *testing.T) {
	leader, target, other := transferSetup(t)
	leader.transferStarter = timedOutAfterMovingTo(t, leader, other)

	err := leader.LeadershipTransferTo([]Server{target})
	if !errors.Is(err, ErrTransferLandedElsewhere) {
		t.Fatalf("err = %v, want ErrTransferLandedElsewhere", err)
	}
}

func TestLeadershipTransferThatLeavesLeadershipHereFails(t *testing.T) {
	leader, target, other := transferSetup(t)

	calls := 0
	leader.transferStarter = func(*Server) raft.Future {
		calls++
		return errFuture{errTransferTimeout}
	}

	err := leader.LeadershipTransferTo([]Server{target, other})
	if !errors.Is(err, errTransferTimeout) {
		t.Fatalf("err = %v, want the transfer timeout", err)
	}

	if calls != 1 || !leader.IsLeader() {
		t.Fatalf("%d attempts, leader %v; want one attempt and leadership kept", calls, leader.IsLeader())
	}
}
