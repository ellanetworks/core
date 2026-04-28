// Copyright 2026 Ella Networks

package raft

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/version"
	"github.com/hashicorp/raft"
	_ "github.com/mattn/go-sqlite3"
	"go.uber.org/zap"
)

// Applier is implemented by the database layer to execute FSM commands.
// This interface breaks the import cycle: internal/raft depends on this
// interface, and internal/db implements it.
type Applier interface {
	// ApplyCommand executes a Raft command against the shared database.
	// Each call corresponds to a single committed log entry. logIndex
	// is the Raft log index of the entry being applied; the
	// implementation uses it to publish post-apply change events. The
	// implementation dispatches on cmd.Type to the appropriate applyX
	// method, which uses sqlair to execute the SQL. SQLite's
	// MaxOpenConns(1) serialises access, so no explicit transaction
	// wrapping is needed here — sqlair methods manage their own
	// transactions as they do in standalone mode.
	ApplyCommand(ctx context.Context, cmd *Command, logIndex uint64) (any, error)

	// PlainDB returns the raw *sql.DB for the application database,
	// needed for snapshot operations (VACUUM INTO) and ID counter seeding.
	PlainDB() *sql.DB

	// Path returns the filesystem path to the database file.
	Path() string

	// Reopen closes and reopens the database connection, re-prepares all
	// sqlair statements. Called after FSM.Restore replaces the database
	// file on disk.
	Reopen(ctx context.Context) error

	// BackupLocalTables copies local-only tables (radio_events,
	// flow_reports, fsm_state) from srcPath into destPath so they survive
	// a full database file swap during restore.
	BackupLocalTables(ctx context.Context, srcPath, destPath string) error

	// RestoreLocalTables copies previously backed-up local-only tables
	// from backupPath back into destPath after a database file swap.
	RestoreLocalTables(ctx context.Context, backupPath, destPath string) error
}

// FSM implements raft.FSM for the application database.
//
// Each Apply call deserializes a Command and executes it via the Applier
// interface. Snapshots use SQLite's VACUUM INTO for a consistent, WAL-free
// copy. Restore replaces the database file atomically and reopens connections.
type FSM struct {
	applier Applier

	// appliedIndex is the Raft index of the last successfully applied log.
	// Updated atomically at the end of every Apply; used by the RYW barrier.
	appliedIndex atomic.Uint64

	// dataDir is the directory containing the database file and the raft/ subdirectory.
	dataDir string

	// mu excludes Restore (write lock) from concurrent Apply and Snapshot
	// calls (read lock). hashicorp/raft calls Apply serially, so the RLock
	// is not for Apply-vs-Apply serialization.
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
	if l.Type != raft.LogCommand {
		return nil
	}

	f.mu.RLock()
	defer f.mu.RUnlock()

	// Skip already-applied entries. After a crash without a recent
	// snapshot, hashicorp/raft replays committed entries from index 0.
	// Changesets are not idempotent, so re-applying them would hit the
	// conflict callback and crash-loop. The durable lastApplied value
	// (fsm_state table) lets us skip entries that were already applied
	// before the crash.
	lastApplied, err := f.readLastApplied()
	if err != nil {
		logger.RaftLog.Error("FSM: failed to read lastApplied",
			zap.Uint64("index", l.Index),
			zap.Error(err))

		return fmt.Errorf("read lastApplied: %w", err)
	}

	if l.Index <= lastApplied {
		f.appliedIndex.Store(l.Index)
		return nil
	}

	cmd, err := UnmarshalCommand(l.Data)
	if err != nil {
		logger.RaftLog.Error("FSM: failed to unmarshal command",
			zap.Uint64("index", l.Index),
			zap.Error(err))

		return fmt.Errorf("unmarshal command: %w", err)
	}

	ctx := context.Background()

	result, err := f.applier.ApplyCommand(ctx, cmd, l.Index)
	if cmd.Type == CmdChangeset {
		ObserveChangesetBytes(len(cmd.Payload))
	}

	if err != nil {
		logger.RaftLog.Error("FSM: command failed — halting node",
			zap.Uint64("index", l.Index),
			zap.String("command", cmd.Label()),
			zap.Error(err))

		// A committed log entry that fails to apply means this node has
		// diverged from the cluster. Continuing would silently desync
		// state. The SAVEPOINT inside sqlite3changeset_apply guarantees
		// the DB is clean (fully applied or fully rolled back), so the
		// safest response is to stop the node. Recovery: restart and
		// replay from the latest snapshot, or rejoin the cluster.
		panic(fmt.Sprintf("FSM.Apply: fatal apply error at index %d (cmd=%s): %v", l.Index, cmd.Label(), err))
	}

	// Persist the applied index so crash-recovery replay can skip it.
	if err := f.writeLastApplied(l.Index); err != nil {
		logger.RaftLog.Error("FSM: failed to persist lastApplied — halting node",
			zap.Uint64("index", l.Index),
			zap.Error(err))

		panic(fmt.Sprintf("FSM.Apply: failed to write lastApplied at index %d: %v", l.Index, err))
	}

	f.appliedIndex.Store(l.Index)

	return result
}

// ApplyBatch implements raft.BatchingFSM. The Raft library calls this instead
// of Apply when multiple committed log entries are available, passing up to
// MaxAppendEntries logs at once. Reading lastApplied once and writing it once
// at the end eliminates 2*(N-1) SQLite round-trips compared to per-entry Apply.
func (f *FSM) ApplyBatch(logs []*raft.Log) []interface{} {
	f.mu.RLock()
	defer f.mu.RUnlock()

	lastApplied, err := f.readLastApplied()
	if err != nil {
		logger.RaftLog.Error("FSM: failed to read lastApplied in batch",
			zap.Error(err))

		ret := make([]interface{}, len(logs))
		for i := range ret {
			ret[i] = fmt.Errorf("read lastApplied: %w", err)
		}

		return ret
	}

	results := make([]interface{}, len(logs))
	ctx := context.Background()

	var highestApplied uint64

	for i, l := range logs {
		if l.Type != raft.LogCommand {
			continue
		}

		if l.Index <= lastApplied {
			f.appliedIndex.Store(l.Index)
			highestApplied = l.Index

			continue
		}

		cmd, err := UnmarshalCommand(l.Data)
		if err != nil {
			logger.RaftLog.Error("FSM: failed to unmarshal command in batch",
				zap.Uint64("index", l.Index),
				zap.Error(err))

			results[i] = fmt.Errorf("unmarshal command: %w", err)

			continue
		}

		result, applyErr := f.applier.ApplyCommand(ctx, cmd, l.Index)
		if cmd.Type == CmdChangeset {
			ObserveChangesetBytes(len(cmd.Payload))
		}

		if applyErr != nil {
			logger.RaftLog.Error("FSM: command failed in batch — halting node",
				zap.Uint64("index", l.Index),
				zap.String("command", cmd.Label()),
				zap.Error(applyErr))

			panic(fmt.Sprintf("FSM.ApplyBatch: fatal apply error at index %d (cmd=%s): %v", l.Index, cmd.Label(), applyErr))
		}

		results[i] = result
		highestApplied = l.Index
		f.appliedIndex.Store(l.Index)
	}

	if highestApplied > lastApplied {
		if err := f.writeLastApplied(highestApplied); err != nil {
			logger.RaftLog.Error("FSM: failed to persist lastApplied after batch — halting node",
				zap.Uint64("highestApplied", highestApplied),
				zap.Error(err))

			panic(fmt.Sprintf("FSM.ApplyBatch: failed to write lastApplied at index %d: %v", highestApplied, err))
		}
	}

	return results
}

// Compile-time check: FSM must satisfy raft.BatchingFSM.
var _ raft.BatchingFSM = (*FSM)(nil)

// readLastApplied returns the durable lastApplied Raft index from the
// fsm_state table. Returns 0 if the table is empty or missing.
func (f *FSM) readLastApplied() (uint64, error) {
	var idx uint64

	err := f.applier.PlainDB().QueryRowContext(
		context.Background(),
		"SELECT lastApplied FROM fsm_state WHERE id = 1",
	).Scan(&idx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || strings.Contains(err.Error(), "no such table") {
			return 0, nil
		}

		return 0, err
	}

	return idx, nil
}

// writeLastApplied persists the Raft index of the last applied log entry
// into the fsm_state table.
func (f *FSM) writeLastApplied(index uint64) error {
	_, err := f.applier.PlainDB().ExecContext(
		context.Background(),
		"UPDATE fsm_state SET lastApplied = ? WHERE id = 1",
		index,
	)

	return err
}

// Snapshot header format (16 bytes):
//
//	[0:4]   magic "ELSN"
//	[4:8]   snapshot format version (uint32, big-endian) — starts at 1
//	[8:12]  shared_schema_version (uint32, big-endian)
//	[12:16] protocol_version (uint32, big-endian)
const (
	snapshotMagic         = "ELSN"
	snapshotHeaderSize    = 16
	snapshotFormatVersion = 1
)

// sqliteMagic is the first 16 bytes of any SQLite database file.
var sqliteMagic = []byte("SQLite format 3\x00")

// readSchemaVersion opens a SQLite file read-only and returns the
// schema_version value. Returns 0 if the table doesn't exist.
func readSchemaVersion(ctx context.Context, path string) (uint32, error) {
	db, err := sql.Open("sqlite3", path+"?mode=ro")
	if err != nil {
		return 0, err
	}

	defer func() { _ = db.Close() }()

	var v uint32

	err = db.QueryRowContext(ctx, "SELECT version FROM schema_version WHERE id = 1").Scan(&v)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || strings.Contains(err.Error(), "no such table: schema_version") {
			return 0, nil
		}

		return 0, fmt.Errorf("read schema_version row: %w", err)
	}

	return v, nil
}

// writeSnapshotHeader writes the 16-byte ELSN header into buf.
func writeSnapshotHeader(buf []byte, schemaVersion, protocolVersion uint32) {
	copy(buf[0:4], snapshotMagic)
	binary.BigEndian.PutUint32(buf[4:8], snapshotFormatVersion)
	binary.BigEndian.PutUint32(buf[8:12], schemaVersion)
	binary.BigEndian.PutUint32(buf[12:16], protocolVersion)
}

// Snapshot implements raft.FSM. It uses SQLite's VACUUM INTO to produce a
// consistent, WAL-free copy of ella.db in a temp file, then returns an
// FSMSnapshot that streams it to the Raft snapshot sink.
//
// The RLock participates in Apply/Restore exclusion. MaxOpenConns(1) already
// serialises SQLite writers, but the lock guards against future changes to
// the connection cap.
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

	_, err = f.applier.PlainDB().ExecContext(ctx, "VACUUM INTO ?", tmpPath)
	if err != nil {
		_ = os.Remove(tmpPath)
		return nil, fmt.Errorf("VACUUM INTO snapshot: %w", err)
	}

	schemaVer, err := readSchemaVersion(ctx, tmpPath)
	if err != nil {
		_ = os.Remove(tmpPath)
		return nil, fmt.Errorf("read schema_version from snapshot: %w", err)
	}

	var header [snapshotHeaderSize]byte
	writeSnapshotHeader(header[:], schemaVer, version.ProtocolVersion())

	return &fsmSnapshot{path: tmpPath, header: header}, nil
}

// Restore implements raft.FSM. It replaces the database file with the
// payload bytes, then reopens the database connection and re-prepares
// statements. Two input shapes are accepted:
//
//  1. ELSN-prefixed snapshots produced by FSM.Snapshot and delivered
//     via the standard raft snapshot pipeline.
//  2. Raw SQLite files delivered via raft.UserRestore — the db.Restore
//     path hands the ella.db bytes extracted from a backup archive
//     directly to raft, which invokes FSM.Restore without the ELSN
//     wrapper.
func (f *FSM) Restore(rc io.ReadCloser) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	defer func() { _ = rc.Close() }()

	var peek [snapshotHeaderSize]byte

	if _, err := io.ReadFull(rc, peek[:]); err != nil {
		return fmt.Errorf("read snapshot header: %w", err)
	}

	var sqliteReader io.Reader

	switch {
	case bytes.Equal(peek[:4], []byte(snapshotMagic)):
		fmtVer := binary.BigEndian.Uint32(peek[4:8])
		if fmtVer > snapshotFormatVersion {
			return fmt.Errorf("snapshot format version %d exceeds supported version %d", fmtVer, snapshotFormatVersion)
		}

		protoVer := binary.BigEndian.Uint32(peek[12:16])
		if protoVer > version.ProtocolVersion() {
			return fmt.Errorf("snapshot protocol version %d exceeds this binary's protocol version %d", protoVer, version.ProtocolVersion())
		}

		sqliteReader = rc
	case bytes.Equal(peek[:], sqliteMagic):
		sqliteReader = io.MultiReader(bytes.NewReader(peek[:]), rc)
	default:
		return fmt.Errorf("corrupt snapshot: unrecognized header magic %q", peek[:4])
	}

	// Write snapshot to a temp file in the data directory.
	tmpFile, err := os.CreateTemp(f.dataDir, "restore-*.db")
	if err != nil {
		return fmt.Errorf("create restore temp file: %w", err)
	}

	tmpPath := tmpFile.Name()

	if _, err := io.Copy(tmpFile, sqliteReader); err != nil {
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

	// Preserve local-only tables (radio_events, flow_reports, fsm_state)
	// across the file swap. These are per-node and not part of the
	// replicated snapshot.
	dbPath := f.applier.Path()
	localOnlyPath := filepath.Join(f.dataDir, "restore_snapshot_local.db")

	ctx := context.Background()

	if err := f.applier.BackupLocalTables(ctx, dbPath, localOnlyPath); err != nil {
		_ = os.Remove(tmpPath)
		_ = os.Remove(localOnlyPath)

		return fmt.Errorf("backup local-only tables before restore: %w", err)
	}

	defer func() { _ = os.Remove(localOnlyPath) }()

	// Remove WAL/SHM sidecars before the rename.
	for _, suffix := range []string{"-wal", "-shm"} {
		_ = os.Remove(dbPath + suffix)
	}

	// Atomically replace the database file.
	if err := os.Rename(tmpPath, dbPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rename snapshot over database: %w", err)
	}

	if err := f.applier.RestoreLocalTables(ctx, localOnlyPath, dbPath); err != nil {
		return fmt.Errorf("restore local-only tables after snapshot restore: %w", err)
	}

	// Fsync the parent directory.
	if dir, err := os.Open(f.dataDir); err == nil {
		_ = dir.Sync()
		_ = dir.Close()
	}

	if err := f.applier.Reopen(ctx); err != nil {
		return fmt.Errorf("reopen database after restore: %w", err)
	}

	logger.RaftLog.Info("FSM: restored database from Raft snapshot")

	return nil
}

// fsmSnapshot holds a temp-file-backed snapshot of ella.db.
type fsmSnapshot struct {
	path   string
	header [snapshotHeaderSize]byte
}

const snapshotChunkSize = 64 * 1024

// Persist streams the snapshot header followed by the SQLite file to the Raft
// snapshot sink in 64 KiB chunks.
func (s *fsmSnapshot) Persist(sink raft.SnapshotSink) error {
	// Write the ELSN header first.
	if _, err := sink.Write(s.header[:]); err != nil {
		_ = sink.Cancel()
		return fmt.Errorf("write snapshot header: %w", err)
	}

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
