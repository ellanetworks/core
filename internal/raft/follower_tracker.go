// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

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
	mu    sync.RWMutex
	peers map[raft.ServerID]*followerState
	r     *raft.Raft
}

func newFollowerTracker(r *raft.Raft) *followerTracker {
	return &followerTracker{
		peers: make(map[raft.ServerID]*followerState),
		r:     r,
	}
}

func (ft *followerTracker) start(localID raft.ServerID) (stop func()) {
	peers := make(map[raft.ServerID]*followerState)

	future := ft.r.GetConfiguration()
	if err := future.Error(); err == nil {
		now := time.Now()

		for _, srv := range future.Configuration().Servers {
			if srv.ID == localID {
				continue
			}

			peers[srv.ID] = &followerState{
				lastContact: now,
				healthy:     true,
			}
		}
	}

	ft.mu.Lock()
	ft.peers = peers
	ft.mu.Unlock()

	ch := make(chan raft.Observation, 64)
	observer := raft.NewObserver(ch, false, func(o *raft.Observation) bool {
		switch o.Data.(type) {
		case raft.FailedHeartbeatObservation, raft.ResumedHeartbeatObservation, raft.PeerObservation:
			return true
		default:
			return false
		}
	})

	ft.r.RegisterObserver(observer)

	stopCh := make(chan struct{})
	stopped := make(chan struct{})

	go ft.run(ch, localID, stopCh, stopped)

	return func() {
		ft.r.DeregisterObserver(observer)
		close(stopCh)
		<-stopped

		ft.mu.Lock()
		ft.peers = make(map[raft.ServerID]*followerState)
		ft.mu.Unlock()
	}
}

func (ft *followerTracker) run(ch <-chan raft.Observation, localID raft.ServerID, stopCh chan struct{}, stopped chan struct{}) {
	defer close(stopped)

	for {
		select {
		case <-stopCh:
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

// peerStats returns how long the peer has been out of contact and whether
// it is answering heartbeats. An untracked peer reports (0, false).
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
