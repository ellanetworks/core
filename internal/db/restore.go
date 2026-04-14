// Copyright 2024 Ella Networks

package db

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/canonical/sqlair"
	"github.com/ellanetworks/core/internal/logger"
	ellaraft "github.com/ellanetworks/core/internal/raft"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

const (
	safetyCopyLocalFilename = "restore_safety_local.db"
	manifestArchiveName     = "manifest.json"
	// maxBackupMemberSize caps a single tar member at 2 GiB; combined with
	// maxBackupTotalSize this defends against decompression bombs.
	//
	// These in-package limits are intentionally larger than the API
	// upload cap (see internal/api/server/api_restore.go's maxRestoreSize,
	// currently 256 MiB): the API gate is the operational ceiling for
	// HTTP-uploaded backups, while these constants are defence-in-depth
	// for in-process callers (tests, future tooling) that may legitimately
	// extract larger archives. Lowering these to the API cap would couple
	// the parser to a transport-layer policy.
	maxBackupMemberSize = 2 << 30
	// maxBackupTotalSize caps the cumulative bytes extracted from a single
	// backup, regardless of how many members it contains.
	maxBackupTotalSize = 4 << 30
)

// validateSQLiteFile runs PRAGMA integrity_check against the file.
func validateSQLiteFile(ctx context.Context, path string) error {
	conn, err := sql.Open("sqlite3", path)
	if err != nil {
		return fmt.Errorf("failed to open file as SQLite database: %w", err)
	}

	defer func() { _ = conn.Close() }()

	var result string
	if err := conn.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&result); err != nil {
		return fmt.Errorf("integrity check query failed: %w", err)
	}

	if result != "ok" {
		return fmt.Errorf("integrity check returned: %s", result)
	}

	return nil
}

// extractBackupArchive reads a backup tar.gz from r and writes shared.db and
// local.db into destDir. The manifest is parsed and validated but not
// returned. Unknown members, missing required members, oversize files,
// duplicate entries, and path traversal attempts are rejected.
func extractBackupArchive(r io.Reader, destDir string) error {
	gzReader, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("failed to open gzip stream: %w", err)
	}

	defer func() { _ = gzReader.Close() }()

	tarReader := tar.NewReader(gzReader)

	var (
		sawManifest         bool
		sawShared, sawLocal bool
		totalExtracted      int64
	)

	for {
		hdr, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			return fmt.Errorf("failed to read tar entry: %w", err)
		}

		if hdr.Typeflag != tar.TypeReg {
			return fmt.Errorf("unexpected tar entry type %d for %q", hdr.Typeflag, hdr.Name)
		}

		// Reject path traversal: only bare filenames are allowed.
		if filepath.Base(hdr.Name) != hdr.Name || hdr.Name == "" || hdr.Name == "." || hdr.Name == ".." {
			return fmt.Errorf("invalid tar entry name %q", hdr.Name)
		}

		if hdr.Size < 0 || hdr.Size > maxBackupMemberSize {
			return fmt.Errorf("tar entry %q has invalid size %d", hdr.Name, hdr.Size)
		}

		if totalExtracted+hdr.Size > maxBackupTotalSize {
			return fmt.Errorf("tar entry %q would exceed total extracted budget of %d bytes", hdr.Name, maxBackupTotalSize)
		}

		totalExtracted += hdr.Size

		switch hdr.Name {
		case manifestArchiveName:
			if sawManifest {
				return fmt.Errorf("duplicate tar entry %q", hdr.Name)
			}

			data, err := io.ReadAll(io.LimitReader(tarReader, maxBackupMemberSize))
			if err != nil {
				return fmt.Errorf("failed to read manifest: %w", err)
			}

			var m BackupManifest
			if err := json.Unmarshal(data, &m); err != nil {
				return fmt.Errorf("failed to decode manifest: %w", err)
			}

			if m.Version != BackupManifestVersion {
				return fmt.Errorf("unsupported backup manifest version %d (expected %d)", m.Version, BackupManifestVersion)
			}

			sawManifest = true

		case SharedDBFilename:
			if sawShared {
				return fmt.Errorf("duplicate tar entry %q", hdr.Name)
			}

			if err := writeArchiveMember(filepath.Join(destDir, SharedDBFilename), tarReader, hdr.Size); err != nil {
				return fmt.Errorf("failed to write shared.db: %w", err)
			}

			sawShared = true

		case LocalDBFilename:
			if sawLocal {
				return fmt.Errorf("duplicate tar entry %q", hdr.Name)
			}

			if err := writeArchiveMember(filepath.Join(destDir, LocalDBFilename), tarReader, hdr.Size); err != nil {
				return fmt.Errorf("failed to write local.db: %w", err)
			}

			sawLocal = true

		default:
			return fmt.Errorf("unexpected backup member %q", hdr.Name)
		}
	}

	if !sawManifest {
		return errors.New("backup is missing manifest.json")
	}

	if !sawShared {
		return fmt.Errorf("backup is missing %s", SharedDBFilename)
	}

	if !sawLocal {
		return fmt.Errorf("backup is missing %s", LocalDBFilename)
	}

	return nil
}

func writeArchiveMember(destPath string, src io.Reader, size int64) error {
	out, err := os.OpenFile(destPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600) // #nosec: G304 — destination is under db.Dir()
	if err != nil {
		return err
	}

	if _, err := io.CopyN(out, src, size); err != nil {
		_ = out.Close()
		return err
	}

	return out.Close()
}

// rollbackLocalFromSafetyCopy restores local.db from its safety copy and
// reopens the local connection. Called when a restore fails after local.db
// has been swapped.
func (db *Database) rollbackLocalFromSafetyCopy(ctx context.Context) error {
	if db.local != nil {
		_ = db.local.PlainDB().Close()
	}

	if err := copyWithinDir(db.Dir(), safetyCopyLocalFilename, LocalDBFilename); err != nil {
		return fmt.Errorf("failed to restore local.db from safety copy: %w", err)
	}

	for _, suffix := range []string{"-wal", "-shm"} {
		_ = os.Remove(db.LocalPath() + suffix)
	}

	return db.reopenLocal(ctx)
}

// reopenLocal opens a fresh connection to local.db, runs migrations, and
// re-prepares all sqlair statements.
func (db *Database) reopenLocal(ctx context.Context) error {
	localConn, err := openSQLiteConnection(ctx, db.LocalPath(), SyncNormal)
	if err != nil {
		return fmt.Errorf("failed to reopen local database: %w", err)
	}

	if err := runLocalMigrations(ctx, localConn); err != nil {
		_ = localConn.Close()
		return fmt.Errorf("local schema migration after restore failed: %w", err)
	}

	db.local = sqlair.NewDB(localConn)

	if err := db.PrepareStatements(); err != nil {
		return fmt.Errorf("failed to re-prepare statements after local reopen: %w", err)
	}

	return nil
}

// copyWithinDir copies srcName to dstName as bare filenames inside dir,
// using os.Root to enforce that path traversal cannot escape the directory.
func copyWithinDir(dir, srcName, dstName string) error {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}

	defer func() { _ = root.Close() }()

	src, err := root.Open(srcName)
	if err != nil {
		return err
	}

	defer func() { _ = src.Close() }()

	dst, err := root.OpenFile(dstName, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}

	if _, err := io.Copy(dst, src); err != nil {
		_ = dst.Close()
		return err
	}

	return dst.Close()
}

// Restore replaces both shared.db and local.db with the contents of the
// backup tar.gz in backupFile. shared.db is replicated via Raft through a
// CmdRestore log entry so followers stay in sync; local.db is per-node and
// swapped in place. A safety copy of local.db is rolled back on failure.
func (db *Database) Restore(ctx context.Context, backupFile *os.File) error {
	// Concurrency guard: only one restore at a time.
	if !db.restoreMu.TryLock() {
		return ErrRestoreInProgress
	}
	defer db.restoreMu.Unlock()

	_, span := tracer.Start(ctx, "db/restore", trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	if db.shared == nil || db.local == nil {
		return fmt.Errorf("database connections are not initialized")
	}

	if backupFile == nil {
		return fmt.Errorf("backup file is nil")
	}

	if _, err := backupFile.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("failed to rewind backup file: %w", err)
	}

	// Stage the archive and validate every embedded SQLite file.
	stageDir, err := os.MkdirTemp(db.Dir(), "restore-stage-*")
	if err != nil {
		return fmt.Errorf("failed to create restore stage directory: %w", err)
	}

	defer func() { _ = os.RemoveAll(stageDir) }()

	if err := extractBackupArchive(backupFile, stageDir); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidBackupFile, err)
	}

	stagedShared := filepath.Join(stageDir, SharedDBFilename)
	stagedLocal := filepath.Join(stageDir, LocalDBFilename)

	if err := validateSQLiteFile(ctx, stagedShared); err != nil {
		return fmt.Errorf("%w: shared: %v", ErrInvalidBackupFile, err)
	}

	if err := validateSQLiteFile(ctx, stagedLocal); err != nil {
		return fmt.Errorf("%w: local: %v", ErrInvalidBackupFile, err)
	}

	// The shared.db bytes are carried through the Raft log so that followers
	// and future replays of the log reconstruct the same state.
	sharedBytes, err := os.ReadFile(stagedShared) // #nosec: G304 — path is under stageDir
	if err != nil {
		return fmt.Errorf("failed to read staged shared.db: %w", err)
	}

	if err := db.takeLocalSafetyCopy(ctx); err != nil {
		return fmt.Errorf("failed to create local safety copy: %w", err)
	}

	// The local safety copy is kept until we either fully succeed or fully
	// roll back. If the restore fails and the rollback also fails, the
	// safety copy is the only remaining good image of local.db.
	var safeToDeleteLocalSafetyCopy bool

	defer func() {
		if safeToDeleteLocalSafetyCopy {
			if err := os.Remove(filepath.Join(db.Dir(), safetyCopyLocalFilename)); err != nil && !os.IsNotExist(err) {
				logger.DBLog.Warn("Failed to remove local safety copy",
					zap.String("file", safetyCopyLocalFilename), zap.Error(err))
			}

			return
		}

		logger.WithTrace(ctx, logger.DBLog).Warn(
			"Leaving local restore safety copy in place; manual recovery may be required",
			zap.String("dir", db.Dir()),
			zap.String("local", safetyCopyLocalFilename),
		)
	}()

	if err := db.swapLocalFromStage(ctx, stagedLocal); err != nil {
		if rbErr := db.rollbackLocalFromSafetyCopy(ctx); rbErr != nil {
			logger.WithTrace(ctx, logger.DBLog).Error("Local rollback after failed swap also failed", zap.Error(rbErr))
			return fmt.Errorf("swap local.db: %w (rollback failed: %v)", err, rbErr)
		}

		safeToDeleteLocalSafetyCopy = true

		return fmt.Errorf("swap local.db: %w", err)
	}

	// Route shared.db through Raft: applyRestore swaps the file on every node
	// and re-opens the shared connection.
	if _, err := db.propose(ellaraft.CmdRestore, &bytesPayload{Value: sharedBytes}); err != nil {
		if rbErr := db.rollbackLocalFromSafetyCopy(ctx); rbErr != nil {
			logger.WithTrace(ctx, logger.DBLog).Error("Local rollback after failed propose also failed", zap.Error(rbErr))
			return fmt.Errorf("propose restore: %w (local rollback failed: %v)", err, rbErr)
		}

		safeToDeleteLocalSafetyCopy = true

		return fmt.Errorf("propose restore: %w", err)
	}

	safeToDeleteLocalSafetyCopy = true

	// CmdRestore carries the full shared.db as a log entry. Force a
	// snapshot so the blob doesn't linger in the Raft log and get replicated
	// to followers that fall behind.
	if err := db.raftManager.Snapshot(); err != nil {
		logger.WithTrace(ctx, logger.DBLog).Warn(
			"Failed to snapshot after restore; log retains shared.db blob until next scheduled snapshot",
			zap.Error(err))
	}

	return nil
}

// takeLocalSafetyCopy VACUUMs local.db into a safety copy in db.Dir().
func (db *Database) takeLocalSafetyCopy(ctx context.Context) error {
	path := filepath.Join(db.Dir(), safetyCopyLocalFilename)
	if _, err := db.local.PlainDB().ExecContext(ctx, "VACUUM INTO ?", path); err != nil {
		return fmt.Errorf("failed to VACUUM local safety copy: %w", err)
	}

	return nil
}

// swapLocalFromStage closes the local connection, removes WAL/SHM sidecars,
// atomically renames stagedLocal over local.db, and reopens the connection.
func (db *Database) swapLocalFromStage(ctx context.Context, stagedLocal string) error {
	if db.local != nil {
		if err := db.local.PlainDB().Close(); err != nil {
			return fmt.Errorf("close local database: %w", err)
		}
	}

	for _, suffix := range []string{"-wal", "-shm"} {
		p := db.LocalPath() + suffix
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			logger.WithTrace(ctx, logger.DBLog).Warn("Failed to remove stale local sidecar",
				zap.String("file", p), zap.Error(err))
		}
	}

	if err := os.Rename(stagedLocal, db.LocalPath()); err != nil {
		return fmt.Errorf("rename staged local.db: %w", err)
	}

	if err := fsyncDir(db.Dir()); err != nil {
		logger.WithTrace(ctx, logger.DBLog).Warn("Failed to fsync data directory after local swap",
			zap.String("dir", db.Dir()), zap.Error(err))
	}

	return db.reopenLocal(ctx)
}

// applyRestore is invoked by the FSM for each CmdRestore log entry (on the
// leader after propose, and on followers/replay). It writes the carried
// shared.db bytes to a staged file, validates the SQLite image, atomically
// swaps it into place, reopens the shared connection, and re-seeds the ID
// counters so deterministic IDs pick up from MAX(id) in the restored state.
func (db *Database) applyRestore(ctx context.Context, p *bytesPayload) (any, error) {
	stagedPath := filepath.Join(db.Dir(), "restore-apply-staged.db")
	if err := os.WriteFile(stagedPath, p.Value, 0o600); err != nil {
		return nil, fmt.Errorf("write staged shared.db: %w", err)
	}

	defer func() { _ = os.Remove(stagedPath) }()

	if err := validateSQLiteFile(ctx, stagedPath); err != nil {
		return nil, fmt.Errorf("validate staged shared.db: %w", err)
	}

	if db.shared != nil {
		_ = db.shared.PlainDB().Close()
	}

	for _, suffix := range []string{"-wal", "-shm"} {
		_ = os.Remove(db.SharedPath() + suffix)
	}

	if err := os.Rename(stagedPath, db.SharedPath()); err != nil {
		return nil, fmt.Errorf("install new shared.db: %w", err)
	}

	if err := fsyncDir(db.Dir()); err != nil {
		logger.WithTrace(ctx, logger.DBLog).Warn(
			"Failed to fsync data directory after restore apply",
			zap.String("dir", db.Dir()), zap.Error(err))
	}

	if err := db.ReopenShared(ctx); err != nil {
		return nil, fmt.Errorf("reopen shared.db after restore apply: %w", err)
	}

	return nil, nil
}
