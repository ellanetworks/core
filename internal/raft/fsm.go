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
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/version"
	"github.com/hashicorp/raft"
	sqlite3 "github.com/mattn/go-sqlite3"
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
	// flow_reports, etc.) from srcPath into destPath so they survive
	// a full database file swap during restore. fsm_state is NOT
	// included: its value must come from the snapshot so post-snapshot
	// log entries are replayed correctly.
	BackupLocalTables(ctx context.Context, srcPath, destPath string) error

	// RestoreLocalTables copies previously backed-up local-only tables
	// from backupPath back into destPath after a database file swap.
	// fsm_state is NOT restored: the snapshot already contains the
	// correct lastApplied for its point-in-time state.
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

func (f *FSM) Apply(l *raft.Log) interface{} {
	if l.Type != raft.LogCommand {
		return nil
	}

	return f.ApplyBatch([]*raft.Log{l})[0]
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
		logger.RaftLog.Error("FSM: failed to read lastApplied in batch — halting node",
			zap.Error(err))

		panic(fmt.Sprintf("FSM.ApplyBatch: failed to read lastApplied: %v", err))
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
			logger.RaftLog.Error("FSM: failed to unmarshal command in batch — halting node",
				zap.Uint64("index", l.Index),
				zap.Error(err))

			panic(fmt.Sprintf("FSM.ApplyBatch: failed to unmarshal command at index %d: %v", l.Index, err))
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
func readSchemaVersionConn(ctx context.Context, conn *sql.Conn) (uint32, error) {
	var v uint32

	err := conn.QueryRowContext(ctx, "SELECT version FROM schema_version WHERE id = 1").Scan(&v)
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

// snapshotReadDSN builds the DSN for the pinned snapshot reader. The
// file: prefix is required for SQLite to parse mode=ro at all —
// go-sqlite3 truncates a DSN at the first '?' when it does not start
// with file:, which would silently open the handle READWRITE|CREATE and
// fabricate an empty database if the path were ever missing.
func snapshotReadDSN(path string) string {
	return "file:" + path + "?mode=ro&_busy_timeout=5000"
}

// Snapshot implements raft.FSM. It pins a read snapshot of ella.db on a
// dedicated connection and returns immediately; the page copy happens in
// Persist, which Raft runs on its own goroutine.
//
// Raft calls Snapshot on the runFSM goroutine that also runs ApplyBatch, so
// no apply can interleave here: the pinned transaction observes exactly the
// state Raft records as the snapshot index. SQLite's WAL keeps that view
// stable while later applies continue to commit.
//
// The f.mu read lock taken here is handed to the returned snapshot and
// held until the page copy in Persist finishes. Raft runs Persist on the
// snapshot goroutine, concurrently with the runFSM goroutine that serves
// Restore, and Restore unlinks ella.db-wal/-shm and renames a new file
// over ella.db. Keeping the read lock across the copy is what stops a
// Restore from pulling those files out from under the pinned reader.
func (f *FSM) Snapshot() (raft.FSMSnapshot, error) {
	f.mu.RLock()

	ctx := context.Background()

	snap := &fsmSnapshot{dataDir: f.dataDir, unpin: f.mu.RUnlock}

	readDB, err := sql.Open("sqlite3", snapshotReadDSN(f.applier.Path()))
	if err != nil {
		snap.Release()
		return nil, fmt.Errorf("open snapshot read connection: %w", err)
	}

	snap.readDB = readDB

	conn, err := readDB.Conn(ctx)
	if err != nil {
		snap.Release()
		return nil, fmt.Errorf("acquire snapshot read connection: %w", err)
	}

	snap.readConn = conn

	if err := conn.Raw(func(dc any) error {
		raw, ok := dc.(*sqlite3.SQLiteConn)
		if !ok {
			return fmt.Errorf("unexpected sqlite driver conn type %T", dc)
		}

		snap.readRaw = raw

		return nil
	}); err != nil {
		snap.Release()
		return nil, fmt.Errorf("unwrap snapshot read connection: %w", err)
	}

	if _, err := conn.ExecContext(ctx, "BEGIN"); err != nil {
		snap.Release()
		return nil, fmt.Errorf("begin snapshot read transaction: %w", err)
	}

	snap.inTx = true

	var tables int
	if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_schema").Scan(&tables); err != nil {
		snap.Release()
		return nil, fmt.Errorf("materialize snapshot read transaction: %w", err)
	}

	schemaVer, err := readSchemaVersionConn(ctx, conn)
	if err != nil {
		snap.Release()
		return nil, fmt.Errorf("read schema_version for snapshot: %w", err)
	}

	writeSnapshotHeader(snap.header[:], schemaVer, version.ProtocolVersion())

	return snap, nil
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

	// Read lastApplied from the old database before it is replaced.
	// Used below for a one-time migration check.
	oldLastApplied, _ := f.readLastApplied()

	// Preserve local-only tables (radio_events, flow_reports, etc.)
	// across the file swap. These are per-node and not part of the
	// replicated snapshot. fsm_state is intentionally NOT preserved:
	// the snapshot contains the correct lastApplied for its data, and
	// overwriting it with a stale value would cause the FSM to skip
	// replaying post-snapshot log entries.
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

	// One-time migration: older code kept fsm_state in localOnlyTables,
	// which preserved lastApplied across snapshot restores. If a previous
	// restart cycle skipped entries (because the preserved lastApplied was
	// higher than the snapshot's), later entries in the Raft log were
	// captured against the divergent post-skip state. Replaying ALL entries
	// now would hit changeset conflicts (NOTFOUND) because entries from
	// before the skip and after the skip are incompatible.
	//
	// Detect the upgrade by checking for a marker file that the new code
	// creates after every successful startup. If absent, this is the first
	// restore with the new code; preserve the old lastApplied so entries
	// from the buggy cycle remain skipped (they were captured against a
	// divergent state anyway). Future restores (after a new snapshot is
	// taken by the fixed code) use the snapshot's lastApplied normally.
	migrationMarker := filepath.Join(f.dataDir, ".fsm_migrated")
	if _, statErr := os.Stat(migrationMarker); os.IsNotExist(statErr) {
		snapshotLastApplied, _ := f.readLastApplied()
		if oldLastApplied > snapshotLastApplied {
			if wErr := f.writeLastApplied(oldLastApplied); wErr != nil {
				return fmt.Errorf("migration: preserve old lastApplied: %w", wErr)
			}

			logger.RaftLog.Warn("FSM: preserved lastApplied from pre-upgrade database (one-time migration)",
				zap.Uint64("snapshot_lastApplied", snapshotLastApplied),
				zap.Uint64("preserved_lastApplied", oldLastApplied))
		}

		if wErr := os.WriteFile(migrationMarker, []byte("1"), 0o600); wErr != nil {
			logger.RaftLog.Warn("FSM: failed to write migration marker", zap.Error(wErr))
		}
	}

	logger.RaftLog.Info("FSM: restored database from Raft snapshot")

	return nil
}

// fsmSnapshot holds a pinned read transaction over ella.db, plus the
// FSM read lock that keeps Restore out while that transaction is live.
type fsmSnapshot struct {
	dataDir  string
	readDB   *sql.DB
	readConn *sql.Conn
	readRaw  *sqlite3.SQLiteConn
	inTx     bool
	path     string
	header   [snapshotHeaderSize]byte

	// unpin releases the FSM read lock acquired in Snapshot. unpinOnce
	// guarantees exactly one release across the Persist and Release
	// paths, either of which may run first.
	unpin     func()
	unpinOnce sync.Once
}

const snapshotChunkSize = 64 * 1024

// Persist copies the pinned snapshot into a temp file using SQLite's online
// backup API, then streams the header and that file to the Raft snapshot sink
// in 64 KiB chunks. Raft runs Persist on a dedicated goroutine, so the copy
// no longer blocks applies; the backup reads through the transaction pinned
// in Snapshot, so its contents match the recorded snapshot index.
//
// The pinned transaction and the FSM read lock are dropped as soon as the
// page copy completes, before the (potentially long) streaming phase: the
// temp file is self-contained by then, and holding the read transaction any
// longer would block WAL checkpointing while applies keep appending.
func (s *fsmSnapshot) Persist(sink raft.SnapshotSink) error {
	err := s.copyPinned()

	s.releasePinned()

	if err != nil {
		_ = sink.Cancel()
		return err
	}

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

func (s *fsmSnapshot) copyPinned() error {
	snapshotDir := filepath.Join(s.dataDir, "raft", "snapshots", "tmp")
	if err := os.MkdirAll(snapshotDir, 0o700); err != nil {
		return fmt.Errorf("create snapshot tmp dir: %w", err)
	}

	tmpFile, err := os.CreateTemp(snapshotDir, "snapshot-*.db")
	if err != nil {
		return fmt.Errorf("create snapshot temp file: %w", err)
	}

	s.path = tmpFile.Name()

	_ = tmpFile.Close()

	ctx := context.Background()

	destDB, err := sql.Open("sqlite3", s.path)
	if err != nil {
		return fmt.Errorf("open snapshot destination: %w", err)
	}

	defer func() { _ = destDB.Close() }()

	destConn, err := destDB.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquire snapshot destination connection: %w", err)
	}

	defer func() { _ = destConn.Close() }()

	return destConn.Raw(func(dc any) error {
		destRaw, ok := dc.(*sqlite3.SQLiteConn)
		if !ok {
			return fmt.Errorf("unexpected sqlite driver conn type %T", dc)
		}

		backup, err := destRaw.Backup("main", s.readRaw, "main")
		if err != nil {
			return fmt.Errorf("start snapshot backup: %w", err)
		}

		done, err := backup.Step(-1)
		if err != nil {
			_ = backup.Finish()
			return fmt.Errorf("copy snapshot pages: %w", err)
		}

		if !done {
			_ = backup.Finish()
			return errors.New("snapshot backup did not copy every page")
		}

		if err := backup.Finish(); err != nil {
			return fmt.Errorf("finish snapshot backup: %w", err)
		}

		return nil
	})
}

// releasePinned rolls back the pinned read transaction, closes its
// connection, and releases the FSM read lock so Restore can proceed.
// Idempotent; safe to call from Persist and again from Release.
func (s *fsmSnapshot) releasePinned() {
	if s.inTx {
		_, _ = s.readConn.ExecContext(context.Background(), "ROLLBACK")
		s.inTx = false
	}

	if s.readConn != nil {
		_ = s.readConn.Close()
		s.readConn = nil
	}

	if s.readDB != nil {
		_ = s.readDB.Close()
		s.readDB = nil
	}

	s.readRaw = nil

	if s.unpin != nil {
		s.unpinOnce.Do(s.unpin)
	}
}

// Release rolls back the pinned read transaction and removes the temp file.
func (s *fsmSnapshot) Release() {
	s.releasePinned()

	if s.path != "" {
		_ = os.Remove(s.path)
		s.path = ""
	}
}
