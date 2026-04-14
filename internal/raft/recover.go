// Copyright 2026 Ella Networks

package raft

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/hashicorp/raft"
	"go.uber.org/zap"
)

// raftLogCacheSize wraps the BoltDB log store in an in-memory LRU.
// Hot reads (recent log entries during replication or snapshotting)
// avoid the BoltDB round-trip.
const raftLogCacheSize = 512

// peersFileName is the operator-authored recovery configuration. When
// present at <raftDir>/peers.json on startup, maybeRecoverCluster forces
// the cluster to adopt it via raft.RecoverCluster and then removes the file.
const peersFileName = "peers.json"

// maybeRecoverCluster applies <raftDir>/peers.json if present, then removes
// the file. This is the canonical operator escape hatch when Raft state
// becomes inconsistent (lost quorum, corrupt configuration, wrong server
// set baked into the log).
//
// Must be called before raft.NewRaft. Returns true if recovery ran, so the
// caller can skip a redundant BootstrapCluster on a fresh single-node data
// directory that carries a peers.json.
func maybeRecoverCluster(
	raftDir string,
	cfg *raft.Config,
	fsm raft.FSM,
	logs raft.LogStore,
	stable raft.StableStore,
	snaps raft.SnapshotStore,
	transport raft.Transport,
) (bool, error) {
	peersPath := filepath.Join(raftDir, peersFileName)

	info, err := os.Stat(peersPath)
	if os.IsNotExist(err) {
		return false, nil
	}

	if err != nil {
		return false, fmt.Errorf("stat %s: %w", peersPath, err)
	}

	if info.IsDir() {
		return false, fmt.Errorf("%s is a directory, expected a file", peersPath)
	}

	logger.RaftLog.Warn("Raft: peers.json detected, forcing cluster recovery",
		zap.String("path", peersPath))

	recoveryConfig, err := raft.ReadConfigJSON(peersPath)
	if err != nil {
		return false, fmt.Errorf("read peers.json: %w", err)
	}

	if err := raft.RecoverCluster(cfg, fsm, logs, stable, snaps, transport, recoveryConfig); err != nil {
		return false, fmt.Errorf("recover cluster from peers.json: %w", err)
	}

	if err := os.Remove(peersPath); err != nil {
		return false, fmt.Errorf("remove peers.json after recovery: %w", err)
	}

	logger.RaftLog.Info("Raft: cluster recovered from peers.json",
		zap.Int("servers", len(recoveryConfig.Servers)))

	return true, nil
}
