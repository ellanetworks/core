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
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

const (
	safetyCopySharedFilename = "restore_safety_shared.db"
	safetyCopyLocalFilename  = "restore_safety_local.db"
	manifestArchiveName      = "manifest.json"
	// maxBackupMemberSize caps tar member sizes to guard against
	// decompression bombs.
	maxBackupMemberSize = 8 << 30
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

// extractBackupArchive reads a backup tar.gz from r, writes shared.db,
// local.db, and manifest.json into destDir, and returns the parsed manifest.
// Unknown members, missing required members, oversize files, and path
// traversal attempts are rejected.
func extractBackupArchive(r io.Reader, destDir string) (*BackupManifest, error) {
	gzReader, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("failed to open gzip stream: %w", err)
	}

	defer func() { _ = gzReader.Close() }()

	tarReader := tar.NewReader(gzReader)

	var (
		manifest            *BackupManifest
		sawShared, sawLocal bool
	)

	for {
		hdr, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			return nil, fmt.Errorf("failed to read tar entry: %w", err)
		}

		if hdr.Typeflag != tar.TypeReg {
			return nil, fmt.Errorf("unexpected tar entry type %d for %q", hdr.Typeflag, hdr.Name)
		}

		// Reject path traversal: only bare filenames are allowed.
		if filepath.Base(hdr.Name) != hdr.Name || hdr.Name == "" || hdr.Name == "." || hdr.Name == ".." {
			return nil, fmt.Errorf("invalid tar entry name %q", hdr.Name)
		}

		if hdr.Size < 0 || hdr.Size > maxBackupMemberSize {
			return nil, fmt.Errorf("tar entry %q has invalid size %d", hdr.Name, hdr.Size)
		}

		switch hdr.Name {
		case manifestArchiveName:
			data, err := io.ReadAll(io.LimitReader(tarReader, maxBackupMemberSize))
			if err != nil {
				return nil, fmt.Errorf("failed to read manifest: %w", err)
			}

			var m BackupManifest
			if err := json.Unmarshal(data, &m); err != nil {
				return nil, fmt.Errorf("failed to decode manifest: %w", err)
			}

			if m.Version != BackupManifestVersion {
				return nil, fmt.Errorf("unsupported backup manifest version %d (expected %d)", m.Version, BackupManifestVersion)
			}

			manifest = &m

		case SharedDBFilename:
			if err := writeArchiveMember(filepath.Join(destDir, SharedDBFilename), tarReader, hdr.Size); err != nil {
				return nil, fmt.Errorf("failed to write shared.db: %w", err)
			}

			sawShared = true

		case LocalDBFilename:
			if err := writeArchiveMember(filepath.Join(destDir, LocalDBFilename), tarReader, hdr.Size); err != nil {
				return nil, fmt.Errorf("failed to write local.db: %w", err)
			}

			sawLocal = true

		default:
			return nil, fmt.Errorf("unexpected backup member %q", hdr.Name)
		}
	}

	if manifest == nil {
		return nil, errors.New("backup is missing manifest.json")
	}

	if !sawShared {
		return nil, fmt.Errorf("backup is missing %s", SharedDBFilename)
	}

	if !sawLocal {
		return nil, fmt.Errorf("backup is missing %s", LocalDBFilename)
	}

	return manifest, nil
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

// rollbackFromSafetyCopy restores both DBs from their safety copies and
// reopens connections. Called when a restore fails mid-overwrite.
func (db *Database) rollbackFromSafetyCopy(ctx context.Context) error {
	if err := copyWithinDir(db.Dir(), safetyCopySharedFilename, SharedDBFilename); err != nil {
		return fmt.Errorf("failed to restore shared.db from safety copy: %w", err)
	}

	if err := copyWithinDir(db.Dir(), safetyCopyLocalFilename, LocalDBFilename); err != nil {
		return fmt.Errorf("failed to restore local.db from safety copy: %w", err)
	}

	// Remove WAL/SHM that may be stale after the overwrite.
	for _, base := range []string{SharedDBFilename, LocalDBFilename} {
		_ = os.Remove(filepath.Join(db.Dir(), base+"-wal"))
		_ = os.Remove(filepath.Join(db.Dir(), base+"-shm"))
	}

	return db.reopenAfterRestore(ctx)
}

// reopenAfterRestore opens fresh SQLite connections, runs migrations on each,
// and re-prepares all sqlair statements.
func (db *Database) reopenAfterRestore(ctx context.Context) error {
	sharedConn, err := openSQLiteConnection(ctx, db.SharedPath(), SyncFull)
	if err != nil {
		return fmt.Errorf("failed to reopen shared database: %w", err)
	}

	if err := RunSharedMigrations(ctx, sharedConn); err != nil {
		_ = sharedConn.Close()
		return fmt.Errorf("shared schema migration after restore failed: %w", err)
	}

	localConn, err := openSQLiteConnection(ctx, db.LocalPath(), SyncNormal)
	if err != nil {
		_ = sharedConn.Close()
		return fmt.Errorf("failed to reopen local database: %w", err)
	}

	if err := RunLocalMigrations(ctx, localConn); err != nil {
		_ = localConn.Close()
		_ = sharedConn.Close()

		return fmt.Errorf("local schema migration after restore failed: %w", err)
	}

	db.shared = sqlair.NewDB(sharedConn)
	db.local = sqlair.NewDB(localConn)

	if err := db.PrepareStatements(); err != nil {
		return fmt.Errorf("failed to re-prepare statements after restore: %w", err)
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
// backup tar.gz in backupFile. The operation is atomic across both files: a
// safety copy of each is taken before the swap and rolled back together on
// failure.
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

	if _, err := extractBackupArchive(backupFile, stageDir); err != nil {
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

	// Take a safety copy of both live DBs before overwriting them.
	if err := db.takeSafetyCopies(ctx); err != nil {
		return fmt.Errorf("failed to create safety copies: %w", err)
	}

	defer db.removeSafetyCopies()

	// Close live connections and overwrite both files.
	if err := db.Close(); err != nil {
		return fmt.Errorf("failed to close database connections: %w", err)
	}

	if err := overwriteFile(stagedShared, db.SharedPath()); err != nil {
		if rbErr := db.rollbackFromSafetyCopy(ctx); rbErr != nil {
			logger.WithTrace(ctx, logger.DBLog).Error("Rollback after failed restore also failed", zap.Error(rbErr))
		}

		return fmt.Errorf("failed to install new shared.db: %w", err)
	}

	if err := overwriteFile(stagedLocal, db.LocalPath()); err != nil {
		if rbErr := db.rollbackFromSafetyCopy(ctx); rbErr != nil {
			logger.WithTrace(ctx, logger.DBLog).Error("Rollback after failed restore also failed", zap.Error(rbErr))
		}

		return fmt.Errorf("failed to install new local.db: %w", err)
	}

	// Remove stale WAL/SHM sidecars left over from the previous handles.
	for _, base := range []string{SharedDBFilename, LocalDBFilename} {
		for _, suffix := range []string{"-wal", "-shm"} {
			p := filepath.Join(db.Dir(), base+suffix)
			if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
				logger.WithTrace(ctx, logger.DBLog).Warn("Failed to remove stale sidecar file",
					zap.String("file", p), zap.Error(err))
			}
		}
	}

	if err := db.reopenAfterRestore(ctx); err != nil {
		if rbErr := db.rollbackFromSafetyCopy(ctx); rbErr != nil {
			logger.WithTrace(ctx, logger.DBLog).Error("Rollback after failed restore also failed", zap.Error(rbErr))
		}

		return err
	}

	return nil
}

// takeSafetyCopies VACUUMs both live databases into safety copy files in
// db.Dir(). Partial output is cleaned up on failure.
func (db *Database) takeSafetyCopies(ctx context.Context) error {
	sharedSafety := filepath.Join(db.Dir(), safetyCopySharedFilename)
	localSafety := filepath.Join(db.Dir(), safetyCopyLocalFilename)

	if _, err := db.shared.PlainDB().ExecContext(ctx, "VACUUM INTO ?", sharedSafety); err != nil {
		return fmt.Errorf("failed to VACUUM shared safety copy: %w", err)
	}

	if _, err := db.local.PlainDB().ExecContext(ctx, "VACUUM INTO ?", localSafety); err != nil {
		_ = os.Remove(sharedSafety)
		return fmt.Errorf("failed to VACUUM local safety copy: %w", err)
	}

	return nil
}

func (db *Database) removeSafetyCopies() {
	for _, name := range []string{safetyCopySharedFilename, safetyCopyLocalFilename} {
		if err := os.Remove(filepath.Join(db.Dir(), name)); err != nil && !os.IsNotExist(err) {
			logger.DBLog.Warn("Failed to remove safety copy", zap.String("file", name), zap.Error(err))
		}
	}
}

// overwriteFile atomically replaces dst with src (same filesystem).
func overwriteFile(src, dst string) error {
	return os.Rename(src, dst)
}
