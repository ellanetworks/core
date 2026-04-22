// Copyright 2026 Ella Networks

package raft

import (
	"sync"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/hashicorp/raft"
	"go.uber.org/zap"
)

// LeaderCallback is invoked by the LeaderObserver on leadership transitions.
// Implementations must not block; long-running reactions should be dispatched
// to a goroutine internally.
type LeaderCallback interface {
	OnBecameLeader()
	OnLostLeadership()
}

// LeaderObserver watches raft.LeaderCh() and fans out leadership transitions
// to registered callbacks. In single-server mode the local node is always
// leader, so OnBecameLeader fires once at startup.
type LeaderObserver struct {
	mu        sync.Mutex
	callbacks []LeaderCallback
	isLeader  bool
	stopCh    chan struct{}
	stopped   chan struct{}
}

// NewLeaderObserver creates a LeaderObserver. Call Run to start watching.
func NewLeaderObserver() *LeaderObserver {
	return &LeaderObserver{
		stopCh:  make(chan struct{}),
		stopped: make(chan struct{}),
	}
}

// Register adds a callback. If the observer has already determined that this
// node is the leader, the callback's OnBecameLeader is fired immediately so
// late subscribers don't miss the initial transition.
func (o *LeaderObserver) Register(cb LeaderCallback) {
	o.mu.Lock()
	alreadyLeader := o.isLeader
	o.callbacks = append(o.callbacks, cb)
	o.mu.Unlock()

	if alreadyLeader {
		cb.OnBecameLeader()
	}
}

// IsLeader returns the last observed leadership state.
func (o *LeaderObserver) IsLeader() bool {
	o.mu.Lock()
	defer o.mu.Unlock()

	return o.isLeader
}

// Run watches leaderCh for transitions and notifies all registered callbacks.
// It blocks until Stop is called or the channel is closed.
func (o *LeaderObserver) Run(r *raft.Raft) {
	defer close(o.stopped)

	// Fire initial state based on current Raft state.
	if r.State() == raft.Leader {
		o.setLeader(true)
	}

	for {
		select {
		case <-o.stopCh:
			return
		case isLeader, ok := <-r.LeaderCh():
			if !ok {
				return
			}

			o.setLeader(isLeader)
		}
	}
}

func (o *LeaderObserver) setLeader(isLeader bool) {
	o.mu.Lock()
	prev := o.isLeader
	o.isLeader = isLeader
	cbs := make([]LeaderCallback, len(o.callbacks))
	copy(cbs, o.callbacks)
	o.mu.Unlock()

	if isLeader == prev {
		return
	}

	if isLeader {
		logger.RaftLog.Info("Leadership acquired, notifying subscribers",
			zap.Int("subscribers", len(cbs)))

		for _, cb := range cbs {
			cb.OnBecameLeader()
		}
	} else {
		logger.RaftLog.Info("Leadership lost, notifying subscribers",
			zap.Int("subscribers", len(cbs)))

		for _, cb := range cbs {
			cb.OnLostLeadership()
		}
	}
}

// Stop signals the Run loop to exit and waits for it to finish.
func (o *LeaderObserver) Stop() {
	select {
	case <-o.stopCh:
	default:
		close(o.stopCh)
	}

	<-o.stopped
}
