// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package raft

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/hashicorp/raft"
	"go.uber.org/zap"
)

const (
	databaseSnapshotID  = "database"
	snapshotInstallsDir = "installs"
	installDataSuffix   = ".db"
	installStateSuffix  = ".state"
	installTempSuffix   = ".tmp"
)

type databaseSnapshotStore struct {
	fsm *FSM
	dir string
}

var _ raft.SnapshotStore = (*databaseSnapshotStore)(nil)

func newDatabaseSnapshotStore(fsm *FSM, raftDir string) (*databaseSnapshotStore, error) {
	dir := filepath.Join(raftDir, snapshotInstallsDir)

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create snapshot install directory: %w", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read snapshot install directory: %w", err)
	}

	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())

		if err := os.RemoveAll(path); err != nil {
			return nil, fmt.Errorf("remove leftover snapshot install %s: %w", path, err)
		}

		logger.RaftLog.Info("Removed leftover snapshot install", zap.String("path", path))
	}

	return &databaseSnapshotStore{fsm: fsm, dir: dir}, nil
}

func (s *databaseSnapshotStore) Create(_ raft.SnapshotVersion, index, term uint64, configuration raft.Configuration, configurationIndex uint64, _ raft.Transport) (raft.SnapshotSink, error) {
	return &databaseSnapshotSink{
		store: s,
		state: AppliedState{
			Index:              index,
			Term:               term,
			Configuration:      raft.EncodeConfiguration(configuration),
			ConfigurationIndex: configurationIndex,
		},
		id: databaseSnapshotID,
	}, nil
}

func (s *databaseSnapshotStore) List() ([]*raft.SnapshotMeta, error) {
	s.fsm.mu.RLock()
	state, err := readAppliedState(context.Background(), s.fsm.applier.PlainDB())
	s.fsm.mu.RUnlock()

	if err != nil {
		return nil, err
	}

	if state.Index == 0 {
		return nil, nil
	}

	return []*raft.SnapshotMeta{state.snapshotMeta(databaseSnapshotID)}, nil
}

func (s *databaseSnapshotStore) Open(id string) (*raft.SnapshotMeta, io.ReadCloser, error) {
	if id == databaseSnapshotID {
		return s.openDatabase()
	}

	return s.openInstall(id)
}

func (s *databaseSnapshotStore) openDatabase() (*raft.SnapshotMeta, io.ReadCloser, error) {
	snap, err := s.fsm.pinDatabase()
	if err != nil {
		return nil, nil, err
	}

	state, err := readAppliedState(context.Background(), snap.readConn)
	if err != nil {
		snap.Release()
		return nil, nil, err
	}

	if state.Index == 0 {
		snap.Release()
		return nil, nil, errors.New("database has not applied any raft entries")
	}

	err = snap.copyPinned()

	snap.releasePinned()

	if err != nil {
		snap.Release()
		return nil, nil, err
	}

	f, err := os.Open(snap.path) // #nosec: G304 — path is under our snapshot staging dir
	if err != nil {
		snap.Release()
		return nil, nil, fmt.Errorf("open database snapshot copy: %w", err)
	}

	info, err := f.Stat()
	if err != nil {
		_ = f.Close()

		snap.Release()

		return nil, nil, fmt.Errorf("stat database snapshot copy: %w", err)
	}

	meta := state.snapshotMeta(databaseSnapshotID)
	meta.Size = int64(len(snap.header)) + info.Size()

	return meta, &snapshotSource{
		Reader: io.MultiReader(bytes.NewReader(snap.header[:]), f),
		state:  state,
		self:   true,
		release: func() {
			_ = f.Close()

			snap.Release()
		},
	}, nil
}

func (s *databaseSnapshotStore) openInstall(id string) (*raft.SnapshotMeta, io.ReadCloser, error) {
	if id == "" || strings.ContainsAny(id, `/\`) {
		return nil, nil, fmt.Errorf("invalid snapshot id %q", id)
	}

	dataPath := filepath.Join(s.dir, id+installDataSuffix)
	statePath := filepath.Join(s.dir, id+installStateSuffix)

	raw, err := os.ReadFile(statePath) // #nosec: G304 — path is under the snapshot install dir
	if err != nil {
		return nil, nil, fmt.Errorf("read snapshot %s state: %w", id, err)
	}

	var state AppliedState
	if err := json.Unmarshal(raw, &state); err != nil {
		return nil, nil, fmt.Errorf("decode snapshot %s state: %w", id, err)
	}

	f, err := os.Open(dataPath) // #nosec: G304 — path is under the snapshot install dir
	if err != nil {
		return nil, nil, fmt.Errorf("open snapshot %s: %w", id, err)
	}

	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, nil, fmt.Errorf("stat snapshot %s: %w", id, err)
	}

	meta := state.snapshotMeta(id)
	meta.Size = info.Size()

	return meta, &snapshotSource{
		Reader: f,
		state:  state,
		release: func() {
			_ = f.Close()
			_ = os.Remove(dataPath)
			_ = os.Remove(statePath)
		},
	}, nil
}

type snapshotSource struct {
	io.Reader
	state     AppliedState
	self      bool
	release   func()
	closeOnce sync.Once
}

func (s *snapshotSource) Close() error {
	s.closeOnce.Do(s.release)
	return nil
}

func snapshotSourceOf(rc io.ReadCloser) (*snapshotSource, bool) {
	for {
		switch r := rc.(type) {
		case *snapshotSource:
			return r, true
		case raft.ReadCloserWrapper:
			rc = r.WrappedReadCloser()
		default:
			return nil, false
		}
	}
}

type databaseSnapshotSink struct {
	store *databaseSnapshotStore
	state AppliedState

	mu     sync.Mutex
	file   *os.File
	id     string
	closed bool
}

func (k *databaseSnapshotSink) Write(p []byte) (int, error) {
	k.mu.Lock()
	defer k.mu.Unlock()

	if k.file == nil {
		f, err := os.CreateTemp(k.store.dir, "install-*"+installTempSuffix)
		if err != nil {
			return 0, fmt.Errorf("create snapshot install file: %w", err)
		}

		k.file = f
	}

	return k.file.Write(p)
}

func (k *databaseSnapshotSink) ID() string {
	k.mu.Lock()
	defer k.mu.Unlock()

	return k.id
}

func (k *databaseSnapshotSink) Cancel() error {
	k.mu.Lock()
	defer k.mu.Unlock()

	if k.closed {
		return nil
	}

	k.closed = true

	if k.file != nil {
		_ = k.file.Close()

		return os.Remove(k.file.Name())
	}

	return nil
}

func (k *databaseSnapshotSink) Close() error {
	k.mu.Lock()
	defer k.mu.Unlock()

	if k.closed {
		return nil
	}

	k.closed = true

	if k.file == nil {
		return k.store.recordPosition(k.state)
	}

	id, err := k.store.commitInstall(k.file, k.state)
	if err != nil {
		return err
	}

	k.id = id

	return nil
}

func (s *databaseSnapshotStore) recordPosition(state AppliedState) error {
	ctx := context.Background()

	s.fsm.mu.RLock()
	err := durably(ctx, s.fsm.applier.PlainDB(), func(db sqlExecer) error {
		return advanceAppliedState(ctx, db, state)
	})
	s.fsm.mu.RUnlock()

	if err != nil {
		return fmt.Errorf("record snapshot position %d: %w", state.Index, err)
	}

	return s.fsm.syncAppliedIndex()
}

func (s *databaseSnapshotStore) commitInstall(f *os.File, state AppliedState) (string, error) {
	tmpPath := f.Name()

	fail := func(err error) (string, error) {
		_ = f.Close()
		_ = os.Remove(tmpPath)

		return "", err
	}

	if err := f.Close(); err != nil {
		return fail(fmt.Errorf("close snapshot install: %w", err))
	}

	id := fmt.Sprintf("%d-%d-%d", state.Term, state.Index, time.Now().UnixMilli())

	raw, err := json.Marshal(state)
	if err != nil {
		return fail(fmt.Errorf("encode snapshot state: %w", err))
	}

	if err := os.WriteFile(filepath.Join(s.dir, id+installStateSuffix), raw, 0o600); err != nil {
		return fail(fmt.Errorf("write snapshot state: %w", err))
	}

	if err := os.Rename(tmpPath, filepath.Join(s.dir, id+installDataSuffix)); err != nil {
		_ = os.Remove(filepath.Join(s.dir, id+installStateSuffix))
		return fail(fmt.Errorf("commit snapshot install: %w", err))
	}

	return id, nil
}
