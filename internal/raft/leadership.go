// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package raft

import (
	"context"
	"sync"
	"time"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/hashicorp/raft"
	"go.uber.org/zap"
)

type LeaderHook func(ctx context.Context)

type leaderTerm struct {
	raftTerm     uint64
	ctx          context.Context
	cancel       context.CancelFunc
	hooks        sync.WaitGroup
	hooksStarted bool
	done         chan struct{}
}

func (t *leaderTerm) startHook(hook LeaderHook) {
	t.hooks.Go(func() { hook(t.ctx) })
}

func (m *Manager) OnLeadership(hook LeaderHook) {
	m.leaderMu.Lock()
	defer m.leaderMu.Unlock()

	m.hooks = append(m.hooks, hook)

	if t := m.term; t != nil && t.hooksStarted {
		t.startHook(hook)
	}
}

func (m *Manager) monitorLeadership() {
	defer close(m.leaderLoopDone)

	if m.raft.State() == raft.Leader {
		m.beginTerm()
	}

	for {
		select {
		case <-m.shutdownCh:
			m.endTerm()
			return
		case isLeader, ok := <-m.raft.LeaderCh():
			if !ok {
				m.endTerm()
				return
			}

			if isLeader {
				m.beginTerm()
			} else {
				m.endTerm()
			}
		}
	}
}

func (m *Manager) beginTerm() {
	raftTerm := m.raft.CurrentTerm()

	m.leaderMu.Lock()
	current := m.term
	m.leaderMu.Unlock()

	if current != nil {
		select {
		case <-current.done:
		default:
			if current.raftTerm == raftTerm {
				return
			}
		}

		m.endTerm()
	}

	ctx, cancel := context.WithCancel(context.Background()) // #nosec G118 -- cancelled by endTerm
	t := &leaderTerm{raftTerm: raftTerm, ctx: ctx, cancel: cancel, done: make(chan struct{})}

	m.leaderMu.Lock()
	m.term = t
	m.leaderMu.Unlock()

	logger.RaftLog.Info("Leadership acquired", zap.Uint64("term", raftTerm))

	go m.runTerm(t)
}

func (m *Manager) endTerm() {
	m.leaderMu.Lock()
	t := m.term
	m.term = nil
	m.leaderMu.Unlock()

	if t == nil {
		return
	}

	logger.RaftLog.Info("Leadership lost", zap.Uint64("term", t.raftTerm))

	t.cancel()
	m.notifyLeaderChange()
	<-t.done
}

func (m *Manager) runTerm(t *leaderTerm) {
	defer close(t.done)

	if !m.barrierForTerm(t.ctx, t.raftTerm) {
		return
	}

	if m.followerTracker != nil {
		stopTracker := m.followerTracker.start(raft.ServerID(m.raftID))
		defer stopTracker()
	}

	if m.autopilot != nil {
		m.autopilot.Start(t.ctx)

		defer func() { <-m.autopilot.Stop() }()
	}

	m.leaderMu.Lock()
	if m.term == t {
		t.hooksStarted = true

		for _, hook := range m.hooks {
			t.startHook(hook)
		}
	}
	m.leaderMu.Unlock()

	<-t.ctx.Done()
	t.hooks.Wait()
}

func (m *Manager) barrierForTerm(ctx context.Context, raftTerm uint64) bool {
	for {
		err := m.raft.Barrier(0).Error()
		if err == nil {
			m.barrierMu.Lock()
			m.barrieredTerm = raftTerm
			m.barrierMu.Unlock()

			m.notifyLeaderChange()

			return true
		}

		if ctx.Err() != nil || m.raft.State() != raft.Leader {
			logger.RaftLog.Warn("Post-leadership barrier abandoned", zap.Error(err))
			return false
		}

		logger.RaftLog.Error("Post-leadership barrier failed; retrying before leader hooks run", zap.Error(err))

		select {
		case <-ctx.Done():
			return false
		case <-time.After(leaderBarrierRetryInterval):
		}
	}
}

func (m *Manager) notifyLeaderChange() {
	m.barrierMu.Lock()
	defer m.barrierMu.Unlock()

	close(m.leaderChanged)
	m.leaderChanged = make(chan struct{})
}

func (m *Manager) barrierState() (uint64, <-chan struct{}) {
	m.barrierMu.Lock()
	defer m.barrierMu.Unlock()

	return m.barrieredTerm, m.leaderChanged
}

// WriteBarrier blocks until the FSM has applied every entry committed before
// this call. raft.State() reports Leader while entries from the previous term
// are still queued for the FSM, so a changeset captured in that window carries
// pre-images those entries invalidate. Once per term is enough: everything
// this node appends afterwards is ordered behind the barrier.
func (m *Manager) WriteBarrier(timeout time.Duration) error {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		barriered, changed := m.barrierState()

		term := m.raft.CurrentTerm()
		if term != 0 && barriered == term {
			return nil
		}

		if m.raft.State() != raft.Leader {
			return raft.ErrNotLeader
		}

		select {
		case <-changed:
		case <-timer.C:
			return ErrBarrierTimeout
		}
	}
}

func (m *Manager) WaitForLeaderBarrier(ctx context.Context) error {
	for {
		barriered, changed := m.barrierState()
		if barriered != 0 {
			return nil
		}

		select {
		case <-changed:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
