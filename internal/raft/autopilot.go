// Copyright 2026 Ella Networks

package raft

import (
	"context"
	"fmt"
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
	defaultMinQuorum               = uint(2)
	defaultServerStabilizationTime = 10 * time.Second
)

// autopilotDelegate implements autopilot.ApplicationIntegration using the
// cluster_members table as the source of known servers.
type autopilotDelegate struct {
	manager *Manager
}

func (d *autopilotDelegate) AutopilotConfig() *autopilot.Config {
	return &autopilot.Config{
		CleanupDeadServers:      defaultCleanupDeadServers,
		LastContactThreshold:    defaultLastContactThreshold,
		MaxTrailingLogs:         defaultMaxTrailingLogs,
		MinQuorum:               defaultMinQuorum,
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

	for id, srv := range servers {
		if srv.IsLeader {
			result[id] = parseRaftStats(leaderStats)
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

	servers := make(map[raft.ServerID]*autopilot.Server, len(future.Configuration().Servers))
	for _, srv := range future.Configuration().Servers {
		servers[srv.ID] = &autopilot.Server{
			ID:         srv.ID,
			Name:       string(srv.ID),
			Address:    srv.Address,
			NodeStatus: autopilot.NodeAlive,
		}
	}

	return servers
}

func (d *autopilotDelegate) RemoveFailedServer(srv *autopilot.Server) {
	logger.RaftLog.Info("Autopilot: removing failed server",
		zap.String("id", string(srv.ID)),
		zap.String("address", string(srv.Address)),
	)
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
	delegate := &autopilotDelegate{manager: m}
	ap := autopilot.New(r, delegate,
		autopilot.WithLogger(hclog.NewNullLogger()),
	)

	return &autopilotRunner{ap: ap}
}

func (a *autopilotRunner) OnBecameLeader() {
	a.ap.Start(context.Background())
}

func (a *autopilotRunner) OnLostLeadership() {
	a.ap.Stop()
}
