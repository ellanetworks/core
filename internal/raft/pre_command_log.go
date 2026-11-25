// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package raft

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/hashicorp/raft"
	"go.uber.org/zap"
)

func isPreCommandEntry(entry *raft.Log) bool {
	if entry.Type != raft.LogCommand || len(entry.Data) < 2 {
		return false
	}

	t := CommandType(binary.BigEndian.Uint16(entry.Data[:2]))

	return t != CmdBatch && t != CmdMigrateShared
}

func firstPreCommandEntry(logs raft.LogStore) (uint64, error) {
	first, err := logs.FirstIndex()
	if err != nil {
		return 0, fmt.Errorf("read first raft log index: %w", err)
	}

	last, err := logs.LastIndex()
	if err != nil {
		return 0, fmt.Errorf("read last raft log index: %w", err)
	}

	if first == 0 {
		return 0, nil
	}

	for i := first; i <= last; i++ {
		var entry raft.Log

		if err := logs.GetLog(i, &entry); err != nil {
			if errors.Is(err, raft.ErrLogNotFound) {
				continue
			}

			return 0, fmt.Errorf("read raft log index %d: %w", i, err)
		}

		if isPreCommandEntry(&entry) {
			return i, nil
		}
	}

	return 0, nil
}

func (m *Manager) compactPreCommandLog(ctx context.Context) error {
	index, err := firstPreCommandEntry(m.logStore)
	if err != nil || index == 0 {
		return err
	}

	retention := m.raft.ReloadableConfig()
	compacting := retention
	compacting.TrailingLogs = 0

	if err := m.raft.ReloadConfig(compacting); err != nil {
		return fmt.Errorf("apply log-compaction config: %w", err)
	}

	snapErr := awaitFuture(ctx, m.raft.Snapshot())

	if restoreErr := m.raft.ReloadConfig(retention); restoreErr != nil {
		return errors.Join(snapErr, fmt.Errorf("restore log retention: %w", restoreErr))
	}

	if snapErr != nil {
		return fmt.Errorf("snapshot: %w", snapErr)
	}

	logger.RaftLog.Info("Raft: compacted log entries written before replicated commands",
		zap.Uint64("first_index", index))

	return nil
}
