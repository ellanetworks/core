// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package raft

import (
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/hashicorp/raft"
	"go.uber.org/zap"
)

func readPersistedConfiguration(logs raft.LogStore, snaps raft.SnapshotStore) (raft.Configuration, error) {
	var (
		config    raft.Configuration
		baseIndex uint64
	)

	snapshots, err := snaps.List()
	if err != nil {
		return config, fmt.Errorf("list snapshots: %w", err)
	}

	for _, snapshot := range snapshots {
		if snapshot.Version == 0 {
			continue
		}

		config = snapshot.Configuration
		baseIndex = snapshot.Index

		break
	}

	firstIndex, err := logs.FirstIndex()
	if err != nil {
		return config, fmt.Errorf("read first log index: %w", err)
	}

	lastIndex, err := logs.LastIndex()
	if err != nil {
		return config, fmt.Errorf("read last log index: %w", err)
	}

	start := baseIndex + 1
	if firstIndex > start {
		start = firstIndex
	}

	for index := start; index <= lastIndex; index++ {
		var entry raft.Log

		if err := logs.GetLog(index, &entry); err != nil {
			return config, fmt.Errorf("read log at index %d: %w", index, err)
		}

		if entry.Type == raft.LogConfiguration {
			config = raft.DecodeConfiguration(entry.Data)
		}
	}

	return config, nil
}

func maybeRealignSelfAddress(
	raftDir string,
	cfg *raft.Config,
	fsm raft.FSM,
	logs raft.LogStore,
	stable raft.StableStore,
	snaps raft.SnapshotStore,
	transport raft.Transport,
) (bool, error) {
	persisted, err := readPersistedConfiguration(logs, snaps)
	if err != nil {
		return false, err
	}

	if len(persisted.Servers) != 1 {
		return false, nil
	}

	server := persisted.Servers[0]
	if server.ID != cfg.LocalID {
		return false, nil
	}

	advertised := transport.LocalAddr()
	if server.Address == advertised {
		return false, nil
	}

	logger.RaftLog.Warn("Raft: sole server recorded at a stale address, realigning",
		zap.String("node_id", string(cfg.LocalID)),
		zap.String("recorded_address", string(server.Address)),
		zap.String("advertise_address", string(advertised)),
		zap.String("raft_dir", raftDir),
	)

	realigned := raft.Configuration{
		Servers: []raft.Server{{
			ID:       server.ID,
			Suffrage: server.Suffrage,
			Address:  advertised,
		}},
	}

	if err := raft.RecoverCluster(cfg, fsm, logs, stable, snaps, transport, realigned); err != nil {
		return false, fmt.Errorf("realign sole server address: %w", err)
	}

	return true, nil
}
