// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package raft

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/hashicorp/raft"
	"go.uber.org/zap"
)

const legacySnapshotsDir = "snapshots"

func migrateLegacySnapshots(raftDir string, raftConfig *raft.Config, fsm *FSM, logs raft.LogStore) (bool, error) {
	ctx := context.Background()
	legacyDir := filepath.Join(raftDir, legacySnapshotsDir)

	state, err := readAppliedState(ctx, fsm.applier.PlainDB())
	if err != nil {
		return false, err
	}

	_, statErr := os.Stat(legacyDir)

	hasLegacyDir := statErr == nil
	if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		return false, fmt.Errorf("inspect legacy snapshot directory: %w", statErr)
	}

	if !hasLegacyDir && (state.Configuration != nil || state.Index == 0) {
		return false, nil
	}

	var (
		legacy raft.SnapshotStore
		newest *raft.SnapshotMeta
	)

	if hasLegacyDir {
		store, err := raft.NewFileSnapshotStoreWithLogger(raftDir, 1, newZapRaftSubLogger("snapshot"))
		if err != nil {
			return false, fmt.Errorf("open legacy snapshot store: %w", err)
		}

		legacy = newestSnapshotStore{SnapshotStore: store}

		metas, err := legacy.List()
		if err != nil {
			return false, fmt.Errorf("list legacy snapshots: %w", err)
		}

		if len(metas) > 0 {
			newest = metas[0]
		}
	}

	position := state.Index

	if newest != nil && position < newest.Index {
		covered, err := onlyNonCommandsBetween(logs, position, newest.Index)
		if err != nil {
			return false, err
		}

		if !covered {
			if err := restoreLegacySnapshot(raftConfig, fsm, logs, legacy, newest); err != nil {
				return false, err
			}
		}

		position = newest.Index
	}

	if position == 0 {
		return false, removeLegacySnapshots(legacyDir, hasLegacyDir)
	}

	migrated, err := legacyAppliedState(logs, newest, position)
	if err != nil {
		return false, err
	}

	if err := durably(ctx, fsm.applier.PlainDB(), func(db sqlExecer) error {
		return overwriteAppliedState(ctx, db, migrated)
	}); err != nil {
		return false, fmt.Errorf("record migrated raft position: %w", err)
	}

	if err := fsm.syncAppliedIndex(); err != nil {
		return false, err
	}

	logger.RaftLog.Info("Raft: moved snapshot position into the database",
		zap.Uint64("index", migrated.Index),
		zap.Uint64("term", migrated.Term),
		zap.Uint64("configuration_index", migrated.ConfigurationIndex))

	return true, removeLegacySnapshots(legacyDir, hasLegacyDir)
}

func restoreLegacySnapshot(raftConfig *raft.Config, fsm *FSM, logs raft.LogStore, legacy raft.SnapshotStore, newest *raft.SnapshotMeta) error {
	restoring := *raftConfig
	restoring.NoSnapshotRestoreOnStart = false

	if err := guardSnapshotRestoreOnStart(&restoring, fsm, logs, legacy); err != nil {
		return err
	}

	_, source, err := legacy.Open(newest.ID)
	if err != nil {
		return fmt.Errorf("open legacy snapshot %s: %w", newest.ID, err)
	}

	if err := fsm.Restore(source); err != nil {
		return fmt.Errorf("restore legacy snapshot %s: %w", newest.ID, err)
	}

	logger.RaftLog.Info("Raft: restored the database from a legacy snapshot before migrating",
		zap.String("id", newest.ID),
		zap.Uint64("index", newest.Index))

	return nil
}

func legacyAppliedState(logs raft.LogStore, newest *raft.SnapshotMeta, position uint64) (AppliedState, error) {
	var (
		term               uint64
		configuration      raft.Configuration
		configurationIndex uint64
		scanFrom           uint64 = 1
	)

	if newest != nil {
		configuration = newest.Configuration
		configurationIndex = newest.ConfigurationIndex
		scanFrom = newest.Index + 1

		if position == newest.Index {
			term = newest.Term
		}
	}

	first, err := logs.FirstIndex()
	if err != nil {
		return AppliedState{}, fmt.Errorf("read first raft log index: %w", err)
	}

	scanFrom = max(scanFrom, first)

	for index := scanFrom; index <= position; index++ {
		var entry raft.Log

		if err := logs.GetLog(index, &entry); err != nil {
			return AppliedState{}, fmt.Errorf(
				"database is ahead of the raft store: fsm_state.lastApplied=%d but raft log index %d is unreadable: %w",
				position, index, err)
		}

		if entry.Type == raft.LogConfiguration {
			configuration = raft.DecodeConfiguration(entry.Data)
			configurationIndex = entry.Index
		}

		if index == position {
			term = entry.Term
		}
	}

	if term == 0 {
		return AppliedState{}, fmt.Errorf("database is ahead of the raft store: no raft term recorded for index %d", position)
	}

	if len(configuration.Servers) == 0 {
		return AppliedState{}, fmt.Errorf("no raft configuration found at or before index %d", position)
	}

	return AppliedState{
		Index:              position,
		Term:               term,
		Configuration:      raft.EncodeConfiguration(configuration),
		ConfigurationIndex: configurationIndex,
	}, nil
}

func onlyNonCommandsBetween(logs raft.LogStore, after, through uint64) (bool, error) {
	first, err := logs.FirstIndex()
	if err != nil {
		return false, fmt.Errorf("read first raft log index: %w", err)
	}

	last, err := logs.LastIndex()
	if err != nil {
		return false, fmt.Errorf("read last raft log index: %w", err)
	}

	if first == 0 || first > after+1 || last < through {
		return false, nil
	}

	for index := after + 1; index <= through; index++ {
		var entry raft.Log

		if err := logs.GetLog(index, &entry); err != nil {
			return false, fmt.Errorf("read raft log index %d: %w", index, err)
		}

		if entry.Type == raft.LogCommand {
			return false, nil
		}
	}

	return true, nil
}

func removeLegacySnapshots(legacyDir string, exists bool) error {
	if !exists {
		return nil
	}

	if err := os.RemoveAll(legacyDir); err != nil {
		return fmt.Errorf("remove legacy snapshot directory: %w", err)
	}

	logger.RaftLog.Info("Raft: removed legacy snapshot directory", zap.String("path", legacyDir))

	return nil
}
