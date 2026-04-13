// Copyright 2026 Ella Networks

package raft

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/hashicorp/raft"
	"go.uber.org/zap"
)

// Applier is implemented by the database layer to execute FSM commands.
// This interface breaks the import cycle: internal/raft depends on this
// interface, and internal/db implements it.
type Applier interface {
	// ApplyCommand executes a Raft command against the shared database.
	// Each call corresponds to a single committed log entry. The
	// implementation dispatches on cmd.Type to the appropriate applyX
	// method, which uses sqlair to execute the SQL. SQLite's
	// MaxOpenConns(1) serialises access, so no explicit transaction
	// wrapping is needed here — sqlair methods manage their own
	// transactions as they do in standalone mode.
	ApplyCommand(ctx context.Context, cmd *Command) (any, error)

	// SharedPlainDB returns the raw *sql.DB for the shared database,
	// needed for snapshot operations (VACUUM INTO) and ID counter seeding.
	SharedPlainDB() *sql.DB

	// SharedPath returns the filesystem path to shared.db.
	SharedPath() string

	// ReopenShared closes and reopens the shared database connection,
	// re-prepares all sqlair statements. Called after FSM.Restore
	// replaces the shared.db file on disk.
	ReopenShared(ctx context.Context) error
}

// FSM implements raft.FSM for the shared database.
//
// Each Apply call deserializes a Command and executes it via the Applier
// interface. Snapshots use SQLite's VACUUM INTO for a consistent, WAL-free
// copy. Restore replaces shared.db atomically and reopens connections.
type FSM struct {
	applier Applier

	// appliedIndex is the Raft index of the last successfully applied log.
	// Updated atomically at the end of every Apply; used by the RYW barrier.
	appliedIndex atomic.Uint64

	// dataDir is the directory containing shared.db and the raft/ subdirectory.
	dataDir string

	// mu serializes snapshot and restore operations against applies.
	mu sync.RWMutex
}

// NewFSM creates a new FSM backed by the given Applier.
func NewFSM(applier Applier, dataDir string) *FSM {
	return &FSM{
		applier: applier,
		dataDir: dataDir,
	}
}

// AppliedIndex returns the Raft index of the last applied log entry.
func (f *FSM) AppliedIndex() uint64 {
	return f.appliedIndex.Load()
}

// Apply implements raft.FSM. It is called by the Raft library on every node
// (leader and followers) for each committed log entry.
func (f *FSM) Apply(l *raft.Log) interface{} {
	f.mu.RLock()
	defer f.mu.RUnlock()

	cmd, err := UnmarshalCommand(l.Data)
	if err != nil {
		logger.DBLog.Error("FSM: failed to unmarshal command",
			zap.Uint64("index", l.Index),
			zap.Error(err))

		return fmt.Errorf("unmarshal command: %w", err)
	}

	ctx := context.Background()

	result, err := f.applier.ApplyCommand(ctx, cmd)
	if err != nil {
		logger.DBLog.Error("FSM: command failed",
			zap.Uint64("index", l.Index),
			zap.String("command", cmd.Type.String()),
			zap.Error(err))

		return err
	}

	f.appliedIndex.Store(l.Index)

	return result
}

// Snapshot implements raft.FSM. It uses SQLite's VACUUM INTO to produce a
// consistent, WAL-free copy of shared.db in a temp file, then returns an
// FSMSnapshot that streams it to the Raft snapshot sink.
func (f *FSM) Snapshot() (raft.FSMSnapshot, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	snapshotDir := filepath.Join(f.dataDir, "raft", "snapshots", "tmp")
	if err := os.MkdirAll(snapshotDir, 0o700); err != nil {
		return nil, fmt.Errorf("create snapshot tmp dir: %w", err)
	}

	tmpFile, err := os.CreateTemp(snapshotDir, "snapshot-*.db")
	if err != nil {
		return nil, fmt.Errorf("create snapshot temp file: %w", err)
	}

	tmpPath := tmpFile.Name()

	// Close immediately — VACUUM INTO creates the file itself.
	_ = tmpFile.Close()

	ctx := context.Background()

	_, err = f.applier.SharedPlainDB().ExecContext(ctx, "VACUUM INTO ?", tmpPath)
	if err != nil {
		_ = os.Remove(tmpPath)
		return nil, fmt.Errorf("VACUUM INTO snapshot: %w", err)
	}

	return &fsmSnapshot{path: tmpPath}, nil
}

// Restore implements raft.FSM. It replaces shared.db with the snapshot
// contents, then reopens the database connection and re-prepares statements.
func (f *FSM) Restore(rc io.ReadCloser) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	defer func() { _ = rc.Close() }()

	// Write snapshot to a temp file in the data directory.
	tmpFile, err := os.CreateTemp(f.dataDir, "restore-*.db")
	if err != nil {
		return fmt.Errorf("create restore temp file: %w", err)
	}

	tmpPath := tmpFile.Name()

	if _, err := io.Copy(tmpFile, rc); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)

		return fmt.Errorf("write snapshot to temp file: %w", err)
	}

	if err := tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)

		return fmt.Errorf("fsync temp file: %w", err)
	}

	_ = tmpFile.Close()

	// Remove WAL/SHM sidecars before the rename.
	sharedPath := f.applier.SharedPath()
	for _, suffix := range []string{"-wal", "-shm"} {
		_ = os.Remove(sharedPath + suffix)
	}

	// Atomically replace shared.db.
	if err := os.Rename(tmpPath, sharedPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rename snapshot over shared.db: %w", err)
	}

	// Fsync the parent directory.
	if dir, err := os.Open(f.dataDir); err == nil {
		_ = dir.Sync()
		_ = dir.Close()
	}

	ctx := context.Background()

	if err := f.applier.ReopenShared(ctx); err != nil {
		return fmt.Errorf("reopen shared.db after restore: %w", err)
	}

	logger.DBLog.Info("FSM: restored shared.db from Raft snapshot")

	return nil
}

// fsmSnapshot holds a temp-file-backed snapshot of shared.db.
type fsmSnapshot struct {
	path string
}

const snapshotChunkSize = 64 * 1024

// Persist streams the snapshot file to the Raft snapshot sink in 64 KiB chunks.
func (s *fsmSnapshot) Persist(sink raft.SnapshotSink) error {
	f, err := os.Open(s.path) // #nosec: G304 — path is under our snapshot tmp dir
	if err != nil {
		_ = sink.Cancel()
		return fmt.Errorf("open snapshot file: %w", err)
	}

	defer func() { _ = f.Close() }()

	buf := make([]byte, snapshotChunkSize)

	for {
		n, readErr := f.Read(buf)
		if n > 0 {
			if _, writeErr := sink.Write(buf[:n]); writeErr != nil {
				_ = sink.Cancel()
				return fmt.Errorf("write to snapshot sink: %w", writeErr)
			}
		}

		if readErr == io.EOF {
			break
		}

		if readErr != nil {
			_ = sink.Cancel()
			return fmt.Errorf("read snapshot file: %w", readErr)
		}
	}

	if err := sink.Close(); err != nil {
		return fmt.Errorf("close snapshot sink: %w", err)
	}

	return nil
}

// Release cleans up the temp snapshot file.
func (s *fsmSnapshot) Release() {
	_ = os.Remove(s.path)
}
