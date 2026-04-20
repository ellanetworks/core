// Copyright 2026 Ella Networks

package raft

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
	autopilot "github.com/hashicorp/raft-autopilot"
	"go.uber.org/zap"
)

const (
	defaultCleanupDeadServers      = true
	defaultLastContactThreshold    = 10 * time.Second
	defaultMaxTrailingLogs         = uint64(500)
	defaultServerStabilizationTime = 10 * time.Second
)

// autopilotDelegate implements autopilot.ApplicationIntegration using the
// cluster_members table as the source of known servers.
type autopilotDelegate struct {
	manager          *Manager
	mu               sync.Mutex
	inflightRemovals map[raft.ServerID]bool
}

func (d *autopilotDelegate) AutopilotConfig() *autopilot.Config {
	minQuorum := uint((d.manager.config.BootstrapExpect + 1) / 2)
	if minQuorum < 1 {
		minQuorum = 1
	}

	return &autopilot.Config{
		CleanupDeadServers:      defaultCleanupDeadServers,
		LastContactThreshold:    defaultLastContactThreshold,
		MaxTrailingLogs:         defaultMaxTrailingLogs,
		MinQuorum:               minQuorum,
		ServerStabilizationTime: defaultServerStabilizationTime,
	}
}

func (d *autopilotDelegate) NotifyState(state *autopilot.State) {
	healthy := 0

	for _, s := range state.Servers {
		if s.Health.Healthy {
			healthy++
		}
	}

	logger.RaftLog.Debug("Autopilot state updated",
		zap.Bool("healthy", state.Healthy),
		zap.Int("failure_tolerance", state.FailureTolerance),
		zap.Int("servers", len(state.Servers)),
		zap.Int("healthy_servers", healthy),
	)
}

func (d *autopilotDelegate) FetchServerStats(_ context.Context, servers map[raft.ServerID]*autopilot.Server) map[raft.ServerID]*autopilot.ServerStats {
	result := make(map[raft.ServerID]*autopilot.ServerStats, len(servers))

	leaderStats := d.manager.raft.Stats()
	parsed := parseRaftStats(leaderStats)
	ft := d.manager.followerTracker

	for id, srv := range servers {
		if srv.IsLeader {
			result[id] = parsed
			continue
		}

		if ft == nil {
			result[id] = &autopilot.ServerStats{
				LastTerm:  parsed.LastTerm,
				LastIndex: parsed.LastIndex,
			}

			continue
		}

		lastContact, healthy := ft.peerStats(id)
		if healthy {
			result[id] = &autopilot.ServerStats{
				LastContact: 0,
				LastTerm:    parsed.LastTerm,
				LastIndex:   parsed.LastIndex,
			}
		} else {
			result[id] = &autopilot.ServerStats{
				LastContact: lastContact,
				LastTerm:    parsed.LastTerm,
				LastIndex:   0,
			}
		}
	}

	return result
}

func (d *autopilotDelegate) KnownServers() map[raft.ServerID]*autopilot.Server {
	future := d.manager.raft.GetConfiguration()
	if err := future.Error(); err != nil {
		logger.RaftLog.Warn("Autopilot: failed to get raft configuration", zap.Error(err))
		return nil
	}

	ft := d.manager.followerTracker
	localID := raft.ServerID(strconv.Itoa(d.manager.nodeID))

	servers := make(map[raft.ServerID]*autopilot.Server, len(future.Configuration().Servers))
	for _, srv := range future.Configuration().Servers {
		status := autopilot.NodeAlive

		if ft != nil && srv.ID != localID && !ft.isHealthy(srv.ID) {
			status = autopilot.NodeLeft
		}

		servers[srv.ID] = &autopilot.Server{
			ID:         srv.ID,
			Name:       string(srv.ID),
			Address:    srv.Address,
			NodeStatus: status,
		}
	}

	return servers
}

func (d *autopilotDelegate) RemoveFailedServer(srv *autopilot.Server) {
	d.mu.Lock()
	if d.inflightRemovals[srv.ID] {
		d.mu.Unlock()
		return
	}

	d.inflightRemovals[srv.ID] = true
	d.mu.Unlock()

	go func() {
		logger.RaftLog.Info("Autopilot: removing failed server",
			zap.String("id", string(srv.ID)),
			zap.String("address", string(srv.Address)),
		)

		future := d.manager.raft.RemoveServer(srv.ID, 0, 0)
		if err := future.Error(); err != nil {
			logger.RaftLog.Error("Autopilot: failed to remove server",
				zap.String("id", string(srv.ID)),
				zap.Error(err),
			)
		}

		d.mu.Lock()
		delete(d.inflightRemovals, srv.ID)
		d.mu.Unlock()
	}()
}

func parseRaftStats(stats map[string]string) *autopilot.ServerStats {
	s := &autopilot.ServerStats{}

	if v, ok := stats["last_contact"]; ok {
		if d, err := time.ParseDuration(v); err == nil {
			s.LastContact = d
		}
	}

	if v, ok := stats["last_log_term"]; ok {
		_, _ = fmt.Sscanf(v, "%d", &s.LastTerm)
	}

	if v, ok := stats["last_log_index"]; ok {
		_, _ = fmt.Sscanf(v, "%d", &s.LastIndex)
	}

	return s
}

// autopilotRunner wraps the autopilot.Autopilot lifecycle, starting it when
// this node becomes leader and stopping it when leadership is lost.
// Implements LeaderCallback.
type autopilotRunner struct {
	ap *autopilot.Autopilot
}

func newAutopilotRunner(r *raft.Raft, m *Manager) *autopilotRunner {
	delegate := &autopilotDelegate{
		manager:          m,
		inflightRemovals: make(map[raft.ServerID]bool),
	}

	ap := autopilot.New(r, delegate,
		autopilot.WithLogger(hclog.NewNullLogger()),
		autopilot.WithPromoter(autopilot.DefaultPromoter()),
	)

	return &autopilotRunner{ap: ap}
}

func (a *autopilotRunner) OnBecameLeader() {
	a.ap.Start(context.Background())
}

func (a *autopilotRunner) OnLostLeadership() {
	a.ap.Stop()
}

// State returns the current autopilot state snapshot. The state is only
// continuously updated while this node is leader; callers should check
// Manager.IsLeader() to decide whether to trust it. Returns nil when
// autopilot has not yet produced a first state (cold start window
// immediately after becoming leader).
func (a *autopilotRunner) State() *autopilot.State {
	if a == nil {
		return nil
	}

	return a.ap.GetState()
}
