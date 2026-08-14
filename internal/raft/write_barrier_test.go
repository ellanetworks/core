// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package raft

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	hraft "github.com/hashicorp/raft"
)

type slowApplier struct {
	*testApplier

	delayNanos atomic.Int64
}

func (a *slowApplier) ApplyCommand(ctx context.Context, cmd *Command, idx uint64) (any, error) {
	if d := a.delayNanos.Load(); d > 0 {
		time.Sleep(time.Duration(d))
	}

	return a.testApplier.ApplyCommand(ctx, cmd, idx)
}

// newLaggingCluster slows the followers only, so whichever one is promoted
// carries a backlog of committed-but-unapplied entries.
func newLaggingCluster(t *testing.T, delay time.Duration) (*TestCluster, []*slowApplier) {
	t.Helper()

	var next atomic.Int32

	next.Store(-1)

	appliers := make([]*slowApplier, 3)

	tc := SetupTestClusterWithAppliers(t, 3, func() Applier {
		i := int(next.Add(1))
		a := &slowApplier{testApplier: newTestApplier(t)}

		if i > 0 {
			a.delayNanos.Store(int64(delay))
		}

		appliers[i] = a

		return a
	})

	return tc, appliers
}

func proposeN(t *testing.T, leader *Manager, n int) {
	t.Helper()

	for i := range n {
		cmd, err := NewCommand(CmdChangeset, map[string]int{"n": i})
		if err != nil {
			t.Fatalf("new command: %v", err)
		}

		if _, err := leader.Propose(cmd, 5*time.Second); err != nil {
			t.Fatalf("propose %d: %v", i, err)
		}
	}
}

func awaitLeader(t *testing.T, tc *TestCluster, survivors []int, appliers []*slowApplier) (*Manager, int) {
	t.Helper()

	deadline := time.After(10 * time.Second)

	for {
		select {
		case <-deadline:
			t.Fatal("no new leader elected")
		case <-time.After(2 * time.Millisecond):
		}

		for _, i := range survivors {
			if tc.Nodes[i].IsLeader() {
				return tc.Nodes[i], len(appliers[i].seen())
			}
		}
	}
}

func TestWriteBarrier_WaitsForPriorTermEntries(t *testing.T) {
	const proposals = 30

	tc, appliers := newLaggingCluster(t, 150*time.Millisecond)

	leader := tc.Nodes[0]
	if !leader.IsLeader() {
		t.Fatal("node 0 is not the leader")
	}

	proposeN(t, leader, proposals)

	if err := leader.Shutdown(); err != nil {
		t.Fatalf("shutdown leader: %v", err)
	}

	tc.Listeners[0].Stop()

	newLeader, appliedAtElection := awaitLeader(t, tc, []int{1, 2}, appliers)

	if appliedAtElection >= proposals {
		t.Fatalf("FSM already caught up at election (%d of %d commands applied): the lag this test needs did not occur",
			appliedAtElection, proposals)
	}

	if err := newLeader.WriteBarrier(10 * time.Second); err != nil {
		t.Fatalf("write barrier: %v", err)
	}

	var applier *slowApplier

	for i, n := range tc.Nodes {
		if n == newLeader {
			applier = appliers[i]
		}
	}

	if got := len(applier.seen()); got < proposals {
		t.Fatalf("commands applied after barrier: want at least %d, got %d", proposals, got)
	}
}

func TestWriteBarrier_TimesOutOnBacklog(t *testing.T) {
	const proposals = 30

	tc, appliers := newLaggingCluster(t, 150*time.Millisecond)

	proposeN(t, tc.Nodes[0], proposals)

	if err := tc.Nodes[0].Shutdown(); err != nil {
		t.Fatalf("shutdown leader: %v", err)
	}

	tc.Listeners[0].Stop()

	newLeader, appliedAtElection := awaitLeader(t, tc, []int{1, 2}, appliers)

	if appliedAtElection >= proposals {
		t.Fatalf("FSM already caught up at election (%d of %d commands applied): the lag this test needs did not occur",
			appliedAtElection, proposals)
	}

	if err := newLeader.WriteBarrier(10 * time.Millisecond); !errors.Is(err, ErrBarrierTimeout) {
		t.Fatalf("write barrier against a backlog: want ErrBarrierTimeout, got %v", err)
	}

	beforeRetries := newLeader.raft.LastIndex()

	for range 5 {
		if err := newLeader.WriteBarrier(10 * time.Millisecond); !errors.Is(err, ErrBarrierTimeout) {
			t.Fatalf("write barrier retry: want ErrBarrierTimeout, got %v", err)
		}
	}

	if got := newLeader.raft.LastIndex(); got != beforeRetries {
		t.Fatalf("last index after 5 timed-out retries: want %d (one barrier in flight), got %d", beforeRetries, got)
	}

	if err := newLeader.WriteBarrier(30 * time.Second); err != nil {
		t.Fatalf("write barrier after backlog drains: %v", err)
	}
}

func TestWriteBarrier_OncePerTerm(t *testing.T) {
	applier := newTestApplier(t)
	tc := SetupTestCluster(t, 3, applier)

	leader := tc.Leader()
	if leader == nil {
		t.Fatal("no leader")
	}

	if err := leader.WriteBarrier(5 * time.Second); err != nil {
		t.Fatalf("first write barrier: %v", err)
	}

	afterFirst := leader.raft.LastIndex()

	if err := leader.WriteBarrier(5 * time.Second); err != nil {
		t.Fatalf("second write barrier: %v", err)
	}

	if got := leader.raft.LastIndex(); got != afterFirst {
		t.Fatalf("last index after second barrier: want %d (no new entry), got %d", afterFirst, got)
	}
}

func TestWriteBarrier_NotLeader(t *testing.T) {
	applier := newTestApplier(t)
	tc := SetupTestCluster(t, 3, applier)

	var follower *Manager

	for _, n := range tc.Nodes {
		if !n.IsLeader() {
			follower = n
			break
		}
	}

	if follower == nil {
		t.Fatal("no follower")
	}

	start := time.Now()

	err := follower.WriteBarrier(30 * time.Second)
	if !errors.Is(err, hraft.ErrNotLeader) {
		t.Fatalf("write barrier on follower: want ErrNotLeader, got %v", err)
	}

	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("write barrier on follower took %s: want an immediate return", elapsed)
	}
}
