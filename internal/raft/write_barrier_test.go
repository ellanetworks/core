// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package raft

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	hraft "github.com/hashicorp/raft"
)

type gatedApplier struct {
	*testApplier

	gate <-chan struct{}
}

func (a *gatedApplier) ApplyCommand(ctx context.Context, cmd *Command, idx uint64) (any, error) {
	if a.gate != nil {
		<-a.gate
	}

	return a.testApplier.ApplyCommand(ctx, cmd, idx)
}

func newGatedCluster(t *testing.T) (*TestCluster, []*gatedApplier, func()) {
	t.Helper()

	var (
		next    atomic.Int32
		once    sync.Once
		release = make(chan struct{})
	)

	next.Store(-1)

	appliers := make([]*gatedApplier, 3)

	tc := SetupTestClusterWithAppliers(t, 3, func() Applier {
		i := int(next.Add(1))
		a := &gatedApplier{testApplier: newTestApplier(t)}

		if i > 0 {
			a.gate = release
		}

		appliers[i] = a

		return a
	})

	unblock := func() { once.Do(func() { close(release) }) }

	t.Cleanup(unblock)

	return tc, appliers, unblock
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

func isElectionChurn(err error) bool {
	return errors.Is(err, hraft.ErrLeadershipLost) || errors.Is(err, hraft.ErrNotLeader)
}

func awaitStableLeader(t *testing.T, tc *TestCluster, survivors []int) (*Manager, int) {
	t.Helper()

	const (
		tick       = 5 * time.Millisecond
		stableHold = 40
	)

	deadline := time.After(20 * time.Second)

	var (
		current *Manager
		idx     int
		held    int
	)

	for {
		select {
		case <-deadline:
			t.Fatal("no stable leader elected")
		case <-time.After(tick):
		}

		var (
			leader   *Manager
			leaderAt int
		)

		for _, i := range survivors {
			if tc.Nodes[i].IsLeader() {
				leader, leaderAt = tc.Nodes[i], i
			}
		}

		if leader == nil || leader != current {
			current, idx, held = leader, leaderAt, 0

			continue
		}

		held++
		if held == stableHold {
			return current, idx
		}
	}
}

func awaitStableLastIndex(t *testing.T, m *Manager) uint64 {
	t.Helper()

	const (
		tick        = 10 * time.Millisecond
		stableTicks = 50
	)

	deadline := time.After(15 * time.Second)
	last := m.raft.LastIndex()
	stable := 0

	for {
		select {
		case <-deadline:
			t.Fatalf("last index never settled (still %d)", last)
		case <-time.After(tick):
		}

		cur := m.raft.LastIndex()
		if cur != last {
			last = cur
			stable = 0

			continue
		}

		stable++
		if stable == stableTicks {
			return last
		}
	}
}

func TestWriteBarrier_WaitsForPriorTermEntries(t *testing.T) {
	const (
		proposals = 30
		attempts  = 5
	)

	tc, appliers, unblock := newGatedCluster(t)

	leader := tc.Nodes[0]
	if !leader.IsLeader() {
		t.Fatal("node 0 is not the leader")
	}

	proposeN(t, leader, proposals)

	if err := leader.Shutdown(); err != nil {
		t.Fatalf("shutdown leader: %v", err)
	}

	tc.Listeners[0].Stop()

	for range attempts {
		newLeader, idx := awaitStableLeader(t, tc, []int{1, 2})
		applier := appliers[idx]

		if got := len(applier.seen()); got != 0 {
			t.Fatalf("commands applied before the barrier: want 0 (the FSM backlog is held), got %d", got)
		}

		done := make(chan error, 1)

		go func() { done <- newLeader.WriteBarrier(30 * time.Second) }()

		select {
		case err := <-done:
			if err == nil {
				t.Fatal("write barrier returned while the prior-term backlog was still unapplied")
			}

			if isElectionChurn(err) {
				continue
			}

			t.Fatalf("write barrier: %v", err)
		case <-time.After(250 * time.Millisecond):
		}

		unblock()

		if err := <-done; err != nil {
			t.Fatalf("write barrier: %v", err)
		}

		if got := len(applier.seen()); got < proposals {
			t.Fatalf("commands applied after barrier: want at least %d, got %d", proposals, got)
		}

		return
	}

	t.Fatalf("no barrier attempt held leadership across %d elections", attempts)
}

func backlogTimeoutAttempt(t *testing.T, tc *TestCluster, appliers []*gatedApplier) (*Manager, bool) {
	t.Helper()

	newLeader, idx := awaitStableLeader(t, tc, []int{1, 2})

	if got := len(appliers[idx].seen()); got != 0 {
		t.Fatalf("commands applied before the barrier: want 0 (the FSM backlog is held), got %d", got)
	}

	switch err := newLeader.WriteBarrier(10 * time.Millisecond); {
	case errors.Is(err, ErrBarrierTimeout):
	case isElectionChurn(err):
		return nil, false
	default:
		t.Fatalf("write barrier against a backlog: want ErrBarrierTimeout, got %v", err)
	}

	beforeRetries := awaitStableLastIndex(t, newLeader)

	for range 5 {
		switch err := newLeader.WriteBarrier(10 * time.Millisecond); {
		case errors.Is(err, ErrBarrierTimeout):
		case isElectionChurn(err):
			return nil, false
		default:
			t.Fatalf("write barrier retry: want ErrBarrierTimeout, got %v", err)
		}
	}

	if got := awaitStableLastIndex(t, newLeader); got != beforeRetries {
		t.Fatalf("last index after 5 timed-out retries: want %d (one barrier in flight), got %d", beforeRetries, got)
	}

	return newLeader, true
}

func TestWriteBarrier_TimesOutOnBacklog(t *testing.T) {
	const (
		proposals = 30
		attempts  = 5
	)

	tc, appliers, unblock := newGatedCluster(t)

	proposeN(t, tc.Nodes[0], proposals)

	if err := tc.Nodes[0].Shutdown(); err != nil {
		t.Fatalf("shutdown leader: %v", err)
	}

	tc.Listeners[0].Stop()

	for range attempts {
		newLeader, ok := backlogTimeoutAttempt(t, tc, appliers)
		if !ok {
			continue
		}

		unblock()

		if err := newLeader.WriteBarrier(30 * time.Second); err != nil {
			t.Fatalf("write barrier after backlog drains: %v", err)
		}

		return
	}

	t.Fatalf("no barrier attempt held leadership across %d elections", attempts)
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
