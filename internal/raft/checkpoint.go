// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package raft

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/hashicorp/raft"
	"go.uber.org/zap"
)

var ErrAdmissionPending = errors.New("raft: this node is still checkpointing its sole-server state; retry shortly")

const (
	admissionWaitTimeout = 30 * time.Second

	checkpointRetryInitialBackoff = time.Second
	checkpointRetryMaxBackoff     = 30 * time.Second
)

func (m *Manager) awaitAdmitted() error {
	if !m.admissionPending.Load() {
		return nil
	}

	deadline := time.Now().Add(admissionWaitTimeout)

	for m.admissionPending.Load() {
		if m.raft.State() != raft.Leader {
			return raft.ErrNotLeader
		}

		if time.Now().After(deadline) {
			return ErrAdmissionPending
		}

		time.Sleep(25 * time.Millisecond)
	}

	return nil
}

func (m *Manager) retrySoleNodeCheckpoint(ctx context.Context) error {
	backoff := checkpointRetryInitialBackoff

	for {
		err := m.soleNodeCheckpoint(ctx)
		if err == nil {
			return nil
		}

		if ctx.Err() != nil || m.raft.State() != raft.Leader {
			return err
		}

		logger.RaftLog.Error("Raft: sole-server checkpoint failed; retrying before this node admits peers",
			zap.String("node_id", m.raftID),
			zap.Error(err),
			zap.Duration("next_backoff", backoff),
		)

		select {
		case <-ctx.Done():
			return err
		case <-time.After(backoff):
		}

		backoff *= 2
		if backoff > checkpointRetryMaxBackoff {
			backoff = checkpointRetryMaxBackoff
		}
	}
}

func (m *Manager) soleNodeCheckpoint(ctx context.Context) error {
	future := m.raft.GetConfiguration()
	if err := awaitFuture(ctx, future); err != nil {
		return fmt.Errorf("read raft configuration: %w", err)
	}

	servers := future.Configuration().Servers

	if len(servers) != 1 || servers[0].ID != raft.ServerID(m.raftID) {
		m.checkpointDone.Store(true)
		m.admissionPending.Store(false)

		return nil
	}

	if servers[0].Suffrage != raft.Voter {
		return fmt.Errorf(
			"refusing to checkpoint: the sole recorded server %s is a %s; correct the Raft configuration explicitly (peers.json) before starting this node",
			m.raftID, servers[0].Suffrage)
	}

	advertised := m.transport.LocalAddr()

	if m.config.Enabled && servers[0].Address != advertised {
		logger.RaftLog.Warn("Raft: sole server recorded at a stale address, realigning",
			zap.String("node_id", m.raftID),
			zap.String("recorded_address", string(servers[0].Address)),
			zap.String("advertise_address", string(advertised)),
			zap.String("raft_dir", filepath.Join(m.dataDir, "raft")),
		)

		if err := awaitFuture(ctx, m.raft.AddVoter(servers[0].ID, advertised, 0, 0)); err != nil {
			return fmt.Errorf("realign sole server address: %w", err)
		}
	}

	if err := awaitFuture(ctx, m.raft.Barrier(0)); err != nil {
		return fmt.Errorf("checkpoint barrier: %w", err)
	}

	retention := m.raft.ReloadableConfig()
	compacting := retention
	compacting.TrailingLogs = 0

	if err := m.raft.ReloadConfig(compacting); err != nil {
		return fmt.Errorf("apply checkpoint log-compaction config: %w", err)
	}

	snapErr := awaitFuture(ctx, m.raft.Snapshot())

	if restoreErr := m.raft.ReloadConfig(retention); restoreErr != nil {
		if snapErr != nil {
			return fmt.Errorf("checkpoint snapshot: %w (restoring log retention also failed: %v)", snapErr, restoreErr)
		}

		return fmt.Errorf("restore log retention after checkpoint: %w", restoreErr)
	}

	if snapErr != nil {
		if errors.Is(snapErr, raft.ErrNothingNewToSnapshot) {
			return fmt.Errorf("checkpoint snapshot: %w (the barrier fence did not advance the FSM boundary)", snapErr)
		}

		return fmt.Errorf("checkpoint snapshot: %w", snapErr)
	}

	m.checkpointDone.Store(true)
	m.admissionPending.Store(false)

	logger.RaftLog.Info("Raft: sole-server checkpoint complete",
		zap.String("node_id", m.raftID),
		zap.String("advertise_address", string(advertised)),
	)

	return nil
}
