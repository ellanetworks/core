// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package raft

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/hashicorp/raft"
)

type newestSnapshotStore struct {
	raft.SnapshotStore
}

func (s newestSnapshotStore) List() ([]*raft.SnapshotMeta, error) {
	metas, err := s.SnapshotStore.List()
	if err != nil || len(metas) <= 1 {
		return metas, err
	}

	return metas[:1], nil
}

func guardSnapshotRestoreOnStart(raftConfig *raft.Config, fsm *FSM, logs raft.LogStore, snaps raft.SnapshotStore) error {
	if raftConfig.NoSnapshotRestoreOnStart {
		return nil
	}

	metas, err := snaps.List()
	if err != nil {
		return fmt.Errorf("list raft snapshots: %w", err)
	}

	if len(metas) == 0 {
		return nil
	}

	newest := metas[0]

	_, rc, err := snaps.Open(newest.ID)
	if err != nil {
		return fmt.Errorf("newest raft snapshot %s (index %d) is unreadable; refusing to start rather than fall back to an older snapshot: %w",
			newest.ID, newest.Index, err)
	}

	defer func() { _ = rc.Close() }()

	var header [snapshotHeaderSize]byte

	if _, err := io.ReadFull(rc, header[:]); err != nil {
		return fmt.Errorf("read header of raft snapshot %s: %w", newest.ID, err)
	}

	if !bytes.Equal(header[:4], []byte(snapshotMagic)) {
		return nil
	}

	snapshotSchema := binary.BigEndian.Uint32(header[8:12])

	lastApplied, err := fsm.readLastApplied()
	if err != nil {
		return fmt.Errorf("read fsm_state.lastApplied: %w", err)
	}

	if lastApplied == 0 {
		return nil
	}

	liveSchema, err := fsm.readLiveSchemaVersion()
	if err != nil {
		return err
	}

	if liveSchema <= snapshotSchema {
		return nil
	}

	pending, err := firstCommandAfter(logs, newest.Index)
	if err != nil {
		return err
	}

	if pending == 0 {
		return nil
	}

	return fmt.Errorf(
		"refusing to restore raft snapshot %s (index %d, schema %d) over the live database at schema %d (fsm_state.lastApplied=%d): "+
			"the unapplied command at log index %d may have been captured at the newer schema; restore from a backup or write a peers.json recovery configuration",
		newest.ID, newest.Index, snapshotSchema, liveSchema, lastApplied, pending)
}

func firstCommandAfter(logs raft.LogStore, index uint64) (uint64, error) {
	first, err := logs.FirstIndex()
	if err != nil {
		return 0, fmt.Errorf("read first raft log index: %w", err)
	}

	last, err := logs.LastIndex()
	if err != nil {
		return 0, fmt.Errorf("read last raft log index: %w", err)
	}

	for i := max(first, index+1); i <= last; i++ {
		var entry raft.Log

		if err := logs.GetLog(i, &entry); err != nil {
			return 0, fmt.Errorf("read raft log index %d: %w", i, err)
		}

		if entry.Type == raft.LogCommand {
			return i, nil
		}
	}

	return 0, nil
}

func (f *FSM) readLiveSchemaVersion() (uint32, error) {
	var v uint32

	err := f.applier.PlainDB().QueryRowContext(context.Background(),
		"SELECT version FROM schema_version WHERE id = 1").Scan(&v)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || strings.Contains(err.Error(), "no such table") {
			return 0, nil
		}

		return 0, fmt.Errorf("read live schema_version: %w", err)
	}

	return v, nil
}
