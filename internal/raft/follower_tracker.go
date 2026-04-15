// Copyright 2026 Ella Networks

package raft

import (
	"sync"
	"time"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/hashicorp/raft"
	"go.uber.org/zap"
)

type followerState struct {
	lastContact time.Time
	healthy     bool
}

// followerTracker uses Raft heartbeat observations to maintain per-follower
// liveness state. The leader's autopilot delegate reads this to return
// accurate stats via FetchServerStats and liveness via KnownServers.
type followerTracker struct {
	mu       sync.RWMutex
	peers    map[raft.ServerID]*followerState
	observer *raft.Observer
	r        *raft.Raft
	stopCh   chan struct{}
	stopped  chan struct{}
}

func newFollowerTracker(r *raft.Raft) *followerTracker {
	return &followerTracker{
		peers:   make(map[raft.ServerID]*followerState),
		r:       r,
		stopCh:  make(chan struct{}),
		stopped: make(chan struct{}),
	}
}

// start registers a Raft observer and begins processing heartbeat
// observations. Call when this node becomes leader.
func (ft *followerTracker) start(localID raft.ServerID) {
	ft.mu.Lock()

	ft.peers = make(map[raft.ServerID]*followerState)

	future := ft.r.GetConfiguration()
	if err := future.Error(); err == nil {
		now := time.Now()

		for _, srv := range future.Configuration().Servers {
			if srv.ID == localID {
				continue
			}

			ft.peers[srv.ID] = &followerState{
				lastContact: now,
				healthy:     true,
			}
		}
	}

	ft.mu.Unlock()

	ch := make(chan raft.Observation, 64)
	ft.observer = raft.NewObserver(ch, false, func(o *raft.Observation) bool {
		switch o.Data.(type) {
		case raft.FailedHeartbeatObservation, raft.ResumedHeartbeatObservation, raft.PeerObservation:
			return true
		default:
			return false
		}
	})

	ft.r.RegisterObserver(ft.observer)

	go ft.run(ch, localID)
}

func (ft *followerTracker) run(ch <-chan raft.Observation, localID raft.ServerID) {
	defer close(ft.stopped)

	for {
		select {
		case <-ft.stopCh:
			return
		case obs := <-ch:
			switch v := obs.Data.(type) {
			case raft.FailedHeartbeatObservation:
				ft.mu.Lock()
				if s, ok := ft.peers[v.PeerID]; ok {
					if s.healthy {
						logger.RaftLog.Warn("Follower heartbeat failed",
							zap.String("peer", string(v.PeerID)),
							zap.Time("last_contact", v.LastContact))
					}

					s.healthy = false
					s.lastContact = v.LastContact
				}
				ft.mu.Unlock()

			case raft.ResumedHeartbeatObservation:
				ft.mu.Lock()
				if s, ok := ft.peers[v.PeerID]; ok {
					logger.RaftLog.Info("Follower heartbeat resumed",
						zap.String("peer", string(v.PeerID)))

					s.healthy = true
					s.lastContact = time.Now()
				}
				ft.mu.Unlock()

			case raft.PeerObservation:
				ft.mu.Lock()
				if v.Removed {
					delete(ft.peers, v.Peer.ID)
				} else if v.Peer.ID != localID {
					if _, exists := ft.peers[v.Peer.ID]; !exists {
						ft.peers[v.Peer.ID] = &followerState{
							lastContact: time.Now(),
							healthy:     true,
						}
					}
				}
				ft.mu.Unlock()
			}
		}
	}
}

// stop deregisters the Raft observer and shuts down the run loop.
// Call when leadership is lost.
func (ft *followerTracker) stop() {
	if ft.observer != nil {
		ft.r.DeregisterObserver(ft.observer)
		ft.observer = nil
	}

	select {
	case <-ft.stopCh:
	default:
		close(ft.stopCh)
	}

	<-ft.stopped

	ft.mu.Lock()
	ft.peers = make(map[raft.ServerID]*followerState)
	ft.stopCh = make(chan struct{})
	ft.stopped = make(chan struct{})
	ft.mu.Unlock()
}

// peerStats returns the last contact duration and health for the given
// follower. Returns (0, false) if the peer is unknown.
func (ft *followerTracker) peerStats(id raft.ServerID) (lastContact time.Duration, healthy bool) {
	ft.mu.RLock()
	defer ft.mu.RUnlock()

	s, ok := ft.peers[id]
	if !ok {
		return 0, false
	}

	if !s.healthy {
		return time.Since(s.lastContact), false
	}

	return 0, true
}

// isHealthy reports whether the given peer is considered reachable.
func (ft *followerTracker) isHealthy(id raft.ServerID) bool {
	ft.mu.RLock()
	defer ft.mu.RUnlock()

	s, ok := ft.peers[id]
	if !ok {
		return false
	}

	return s.healthy
}

// followerTrackerCallback adapts followerTracker to LeaderCallback so the
// LeaderObserver can start/stop tracking automatically.
type followerTrackerCallback struct {
	ft      *followerTracker
	localID raft.ServerID
}

func (c *followerTrackerCallback) OnBecameLeader() {
	c.ft.start(c.localID)
}

func (c *followerTrackerCallback) OnLostLeadership() {
	c.ft.stop()
}

func (ft *followerTracker) asLeaderCallback(localID raft.ServerID) LeaderCallback {
	return &followerTrackerCallback{ft: ft, localID: localID}
}
