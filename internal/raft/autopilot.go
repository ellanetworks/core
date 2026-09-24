// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

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
	defaultCleanupDeadServers      = false
	defaultLastContactThreshold    = 10 * time.Second
	defaultMaxTrailingLogs         = uint64(500)
	defaultServerStabilizationTime = 10 * time.Second
)

// autopilotDelegate implements autopilot.ApplicationIntegration over the
// Raft configuration, with per-peer liveness supplied by the follower
// tracker.
type autopilotDelegate struct {
	manager *Manager
}

func (d *autopilotDelegate) AutopilotConfig() *autopilot.Config {
	lastContact := defaultLastContactThreshold
	if d.manager.config.Autopilot.LastContactThreshold > 0 {
		lastContact = d.manager.config.Autopilot.LastContactThreshold
	}

	stabilization := defaultServerStabilizationTime
	if d.manager.config.Autopilot.ServerStabilizationTime > 0 {
		stabilization = d.manager.config.Autopilot.ServerStabilizationTime
	}

	return &autopilot.Config{
		CleanupDeadServers:      defaultCleanupDeadServers,
		LastContactThreshold:    lastContact,
		MaxTrailingLogs:         defaultMaxTrailingLogs,
		ServerStabilizationTime: stabilization,
	}
}

func (d *autopilotDelegate) NotifyState(_ *autopilot.State) {}

func (d *autopilotDelegate) FetchServerStats(_ context.Context, servers map[raft.ServerID]*autopilot.Server) map[raft.ServerID]*autopilot.ServerStats {
	result := make(map[raft.ServerID]*autopilot.ServerStats, len(servers))

	parsed := parseRaftStats(d.manager.raft.Stats())
	ft := d.manager.followerTracker
	localID := raft.ServerID(d.manager.raftID)
	isLocalLeader := d.manager.raft.State() == raft.Leader

	for id := range servers {
		if isLocalLeader && id == localID {
			result[id] = &autopilot.ServerStats{
				LastContact: parsed.LastContact,
				LastTerm:    parsed.LastTerm,
				LastIndex:   parsed.LastIndex,
			}

			continue
		}

		// Follower rows mirror the leader's LastTerm/LastIndex because we
		// don't track per-peer log progression here, which makes autopilot's
		// internal log-lag check a no-op. LastContact carries the follower
		// tracker's view and is what drives the health readout.
		stats := &autopilot.ServerStats{
			LastTerm:  parsed.LastTerm,
			LastIndex: parsed.LastIndex,
		}

		if ft != nil {
			if lastContact, healthy := ft.peerStats(id); !healthy {
				stats.LastContact = lastContact
			}
		}

		result[id] = stats
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
	localID := raft.ServerID(d.manager.raftID)

	servers := make(map[raft.ServerID]*autopilot.Server, len(future.Configuration().Servers))
	for _, srv := range future.Configuration().Servers {
		status := autopilot.NodeAlive

		if ft != nil && srv.ID != localID {
			if _, healthy := ft.peerStats(srv.ID); !healthy {
				status = autopilot.NodeLeft
			}
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
	logger.RaftLog.Warn("Autopilot asked to remove a failed server; membership changes are operator-driven",
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

func newAutopilot(r *raft.Raft, m *Manager) *autopilot.Autopilot {
	delegate := &autopilotDelegate{manager: m}

	opts := []autopilot.Option{
		autopilot.WithLogger(hclog.NewNullLogger()),
		autopilot.WithPromoter(autopilot.DefaultPromoter()),
	}

	if m.config.Autopilot.ReconcileInterval > 0 {
		opts = append(opts, autopilot.WithReconcileInterval(m.config.Autopilot.ReconcileInterval))
	}

	if m.config.Autopilot.UpdateInterval > 0 {
		opts = append(opts, autopilot.WithUpdateInterval(m.config.Autopilot.UpdateInterval))
	}

	return autopilot.New(r, delegate, opts...)
}
