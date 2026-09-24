// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package raft

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

type termProbe struct {
	starts     atomic.Int32
	inFlight   atomic.Int32
	overlapped atomic.Bool
}

func (p *termProbe) hook(ctx context.Context) {
	if p.inFlight.Add(1) > 1 {
		p.overlapped.Store(true)
	}

	p.starts.Add(1)

	<-ctx.Done()
	time.Sleep(20 * time.Millisecond)
	p.inFlight.Add(-1)
}

func (p *termProbe) waitStarts(t *testing.T, want int32) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for p.starts.Load() < want {
		if time.Now().After(deadline) {
			t.Fatalf("leader hook started %d times, want %d", p.starts.Load(), want)
		}

		time.Sleep(5 * time.Millisecond)
	}
}

func newStoppedLeaderLoop(t *testing.T) (*Manager, *termProbe) {
	t.Helper()

	m, _ := NewTestManager(t, newTestApplier(t))

	m.shutdownOnce.Do(func() { close(m.shutdownCh) })
	<-m.leaderLoopDone

	probe := &termProbe{}
	m.OnLeadership(probe.hook)

	m.beginTerm()
	t.Cleanup(m.endTerm)

	probe.waitStarts(t, 1)

	return m, probe
}

func TestLeaderTermsDoNotOverlap(t *testing.T) {
	m, probe := newStoppedLeaderLoop(t)

	for i := range 2 {
		m.leaderMu.Lock()
		m.term.raftTerm++
		m.leaderMu.Unlock()

		m.beginTerm()

		probe.waitStarts(t, int32(i+2))
	}

	if probe.overlapped.Load() {
		t.Fatal("a leader hook started before the previous term's hooks returned")
	}
}

func TestDuplicateLeaderNotificationKeepsTheTerm(t *testing.T) {
	m, _ := newStoppedLeaderLoop(t)

	m.leaderMu.Lock()
	before := m.term
	m.leaderMu.Unlock()

	m.beginTerm()

	m.leaderMu.Lock()
	after := m.term
	m.leaderMu.Unlock()

	if after != before {
		t.Fatal("a repeated leadership notification for the same term restarted the term")
	}
}

func TestLeadershipRegainedAfterAbandonedTermRestartsIt(t *testing.T) {
	m, probe := newStoppedLeaderLoop(t)

	m.endTerm()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	dead := &leaderTerm{raftTerm: m.raft.CurrentTerm(), ctx: ctx, cancel: cancel, done: make(chan struct{})}
	close(dead.done)

	m.leaderMu.Lock()
	m.term = dead
	m.leaderMu.Unlock()

	m.beginTerm()

	probe.waitStarts(t, 2)

	if err := m.WriteBarrier(time.Second); err != nil {
		t.Fatalf("WriteBarrier after the restarted term: %v", err)
	}
}
