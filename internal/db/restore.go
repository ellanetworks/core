// Copyright 2024 Ella Networks

package db

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/canonical/sqlair"
	"github.com/ellanetworks/core/internal/logger"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// validateSQLiteFile opens the file as a SQLite database and runs
// PRAGMA integrity_check. It returns nil only when the file is a valid,
// non-corrupt SQLite database.
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

// safePath cleans the given path and verifies it resides under the
// database's parent directory. This prevents path-traversal when
// constructing file paths derived from db.filepath.
func (db *Database) safePath(p string) (string, error) {
	cleaned := filepath.Clean(p)
	allowedDir := filepath.Dir(db.filepath)

	if !strings.HasPrefix(cleaned, allowedDir+string(os.PathSeparator)) && cleaned != allowedDir {
		return "", fmt.Errorf("path %q is outside the database directory %q", cleaned, allowedDir)
	}

	return cleaned, nil
}

// rollbackFromSafetyCopy restores the original database from the safety copy
// and reopens the connection. It is called when the restore fails after the
// production database has already been overwritten.
func (db *Database) rollbackFromSafetyCopy(ctx context.Context, safetyCopyPath string) error {
	cleanPath, err := db.safePath(safetyCopyPath)
	if err != nil {
		return fmt.Errorf("invalid safety copy path: %w", err)
	}

	src, err := os.Open(cleanPath)
	if err != nil {
		return fmt.Errorf("failed to open safety copy: %w", err)
	}

	defer func() { _ = src.Close() }()

	dst, err := os.Create(db.filepath)
	if err != nil {
		return fmt.Errorf("failed to create destination for rollback: %w", err)
	}

	if _, err := io.Copy(dst, src); err != nil {
		_ = dst.Close()
		return fmt.Errorf("failed to copy safety copy back: %w", err)
	}

	if err := dst.Close(); err != nil {
		return fmt.Errorf("failed to close destination after rollback: %w", err)
	}

	// Remove WAL/SHM that may be stale after the overwrite.
	_ = os.Remove(db.filepath + "-wal")
	_ = os.Remove(db.filepath + "-shm")

	sqlConn, err := openSQLiteConnection(ctx, db.filepath)
	if err != nil {
		return fmt.Errorf("failed to reopen original database after rollback: %w", err)
	}

	db.conn = sqlair.NewDB(sqlConn)

	if err := db.PrepareStatements(); err != nil {
		return fmt.Errorf("failed to re-prepare statements after rollback: %w", err)
	}

	return nil
}

func (db *Database) Restore(ctx context.Context, backupFile *os.File) error {
	// Concurrency guard: only one restore at a time.
	if !db.restoreMu.TryLock() {
		return ErrRestoreInProgress
	}
	defer db.restoreMu.Unlock()

	_, span := tracer.Start(ctx, "DB Restore", trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	if db.conn == nil {
		return fmt.Errorf("database connection is not initialized")
	}

	if backupFile == nil {
		return fmt.Errorf("backup file is nil")
	}

	// ── Step 1: Validate the uploaded file before any destructive action. ──
	if err := validateSQLiteFile(ctx, backupFile.Name()); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidBackupFile, err)
	}

	// ── Step 2: Create a safety copy of the current database. ──
	safetyCopyPath := filepath.Join(filepath.Dir(db.filepath), "restore_safety_copy.db")

	cleanSafetyCopyPath, err := db.safePath(safetyCopyPath)
	if err != nil {
		return fmt.Errorf("invalid safety copy path: %w", err)
	}

	safetyCopyFile, err := os.Create(cleanSafetyCopyPath)
	if err != nil {
		return fmt.Errorf("failed to create safety copy file: %w", err)
	}

	if err := db.Backup(ctx, safetyCopyFile); err != nil {
		_ = safetyCopyFile.Close()
		_ = os.Remove(cleanSafetyCopyPath)

		return fmt.Errorf("failed to create safety copy of current database: %w", err)
	}

	_ = safetyCopyFile.Close()

	defer func() {
		_ = os.Remove(cleanSafetyCopyPath)
	}()

	// ── Step 3: Close the live connection and overwrite the DB file. ──
	if err := db.Close(); err != nil {
		return fmt.Errorf("failed to close the database connection: %v", err)
	}

	destinationFile, err := os.Create(db.filepath)
	if err != nil {
		if rbErr := db.rollbackFromSafetyCopy(ctx, cleanSafetyCopyPath); rbErr != nil {
			logger.WithTrace(ctx, logger.DBLog).Error("Rollback after failed restore also failed", zap.Error(rbErr))
		}

		return fmt.Errorf("failed to open destination database file: %v", err)
	}

	_, err = io.Copy(destinationFile, backupFile)
	if err != nil {
		_ = destinationFile.Close()

		if rbErr := db.rollbackFromSafetyCopy(ctx, cleanSafetyCopyPath); rbErr != nil {
			logger.WithTrace(ctx, logger.DBLog).Error("Rollback after failed restore also failed", zap.Error(rbErr))
		}

		return fmt.Errorf("failed to restore database file: %v", err)
	}

	if err := destinationFile.Close(); err != nil {
		if rbErr := db.rollbackFromSafetyCopy(ctx, cleanSafetyCopyPath); rbErr != nil {
			logger.WithTrace(ctx, logger.DBLog).Error("Rollback after failed restore also failed", zap.Error(rbErr))
		}

		return fmt.Errorf("failed to close destination database file: %w", err)
	}

	// ── Step 4: Remove stale WAL/SHM files. ──
	walFile := db.filepath + "-wal"
	shmFile := db.filepath + "-shm"

	if err := os.Remove(walFile); err != nil && !os.IsNotExist(err) {
		logger.WithTrace(ctx, logger.DBLog).Warn("Failed to remove old WAL file", zap.String("file", walFile), zap.Error(err))
	}

	if err := os.Remove(shmFile); err != nil && !os.IsNotExist(err) {
		logger.WithTrace(ctx, logger.DBLog).Warn("Failed to remove old SHM file", zap.String("file", shmFile), zap.Error(err))
	}

	// ── Step 5: Reopen, migrate, and re-prepare. ──
	sqlConnection, err := openSQLiteConnection(ctx, db.filepath)
	if err != nil {
		if rbErr := db.rollbackFromSafetyCopy(ctx, cleanSafetyCopyPath); rbErr != nil {
			logger.WithTrace(ctx, logger.DBLog).Error("Rollback after failed restore also failed", zap.Error(rbErr))
		}

		return fmt.Errorf("failed to reopen database after restore: %w", err)
	}

	// Migrate the restored database to the current schema. This handles
	// restoring a backup taken from an older version of the binary.
	if err := RunMigrations(ctx, sqlConnection); err != nil {
		_ = sqlConnection.Close()

		if rbErr := db.rollbackFromSafetyCopy(ctx, cleanSafetyCopyPath); rbErr != nil {
			logger.WithTrace(ctx, logger.DBLog).Error("Rollback after failed restore also failed", zap.Error(rbErr))
		}

		return fmt.Errorf("schema migration after restore failed: %w", err)
	}

	db.conn = sqlair.NewDB(sqlConnection)

	if err := db.PrepareStatements(); err != nil {
		if rbErr := db.rollbackFromSafetyCopy(ctx, cleanSafetyCopyPath); rbErr != nil {
			logger.WithTrace(ctx, logger.DBLog).Error("Rollback after failed restore also failed", zap.Error(rbErr))
		}

		return fmt.Errorf("failed to re-prepare statements after restore: %w", err)
	}

	return nil
}
