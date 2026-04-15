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
	"strings"

	"github.com/ellanetworks/core/internal/logger"
	ellaraft "github.com/ellanetworks/core/internal/raft"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

const (
	manifestArchiveName = "manifest.json"
	// maxBackupMemberSize caps a single tar member at 2 GiB; combined with
	// maxBackupTotalSize this defends against decompression bombs.
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

// extractBackupArchive reads a backup tar.gz from r and writes the database
// file into destDir. The manifest is parsed and validated but not returned.
// Unknown members, missing required members, oversize files, duplicate
// entries, and path traversal attempts are rejected.
func extractBackupArchive(r io.Reader, destDir string) error {
	gzReader, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("failed to open gzip stream: %w", err)
	}

	defer func() { _ = gzReader.Close() }()

	tarReader := tar.NewReader(gzReader)

	var (
		sawManifest    bool
		sawDB          bool
		totalExtracted int64
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

		case DBFilename:
			if sawDB {
				return fmt.Errorf("duplicate tar entry %q", hdr.Name)
			}

			if err := writeArchiveMember(filepath.Join(destDir, DBFilename), tarReader, hdr.Size); err != nil {
				return fmt.Errorf("failed to write %s: %w", DBFilename, err)
			}

			sawDB = true

		default:
			return fmt.Errorf("unexpected backup member %q", hdr.Name)
		}
	}

	if !sawManifest {
		return errors.New("backup is missing manifest.json")
	}

	if !sawDB {
		return fmt.Errorf("backup is missing %s", DBFilename)
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

func backupLocalOnlyTables(ctx context.Context, srcPath, destPath string) error {
	src, err := sql.Open("sqlite3", srcPath)
	if err != nil {
		return fmt.Errorf("open source database: %w", err)
	}

	defer func() { _ = src.Close() }()

	dest, err := sql.Open("sqlite3", destPath)
	if err != nil {
		return fmt.Errorf("open local-only backup database: %w", err)
	}

	defer func() { _ = dest.Close() }()

	for _, table := range localOnlyTables {
		createStmt, err := readTableDDL(ctx, src, table)
		if err != nil {
			return err
		}

		if _, err := dest.ExecContext(ctx, createStmt); err != nil {
			return fmt.Errorf("create local-only backup table %s: %w", table, err)
		}

		if err := copyTableRows(ctx, src, dest, table); err != nil {
			return err
		}
	}

	return nil
}

func restoreLocalOnlyTables(ctx context.Context, backupPath, destPath string) error {
	backup, err := sql.Open("sqlite3", backupPath)
	if err != nil {
		return fmt.Errorf("open local-only backup database: %w", err)
	}

	defer func() { _ = backup.Close() }()

	dest, err := sql.Open("sqlite3", destPath)
	if err != nil {
		return fmt.Errorf("open restored database: %w", err)
	}

	defer func() { _ = dest.Close() }()

	tx, err := dest.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin restore of local-only tables: %w", err)
	}

	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	for _, table := range localOnlyTables {
		if _, err := tx.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s", table)); err != nil {
			return fmt.Errorf("clear restored %s: %w", table, err)
		}

		if err := copyTableRowsTx(ctx, backup, tx, table); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit restore of local-only tables: %w", err)
	}

	tx = nil

	return nil
}

func readTableDDL(ctx context.Context, conn *sql.DB, table string) (string, error) {
	var ddl string

	if err := conn.QueryRowContext(ctx,
		"SELECT sql FROM sqlite_master WHERE type='table' AND name = ?", table).Scan(&ddl); err != nil {
		return "", fmt.Errorf("read DDL for %s: %w", table, err)
	}

	if strings.TrimSpace(ddl) == "" {
		return "", fmt.Errorf("table %s has empty DDL", table)
	}

	return ddl, nil
}

func copyTableRows(ctx context.Context, src, dest *sql.DB, table string) error {
	tx, err := dest.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin copy of %s: %w", table, err)
	}

	defer func() { _ = tx.Rollback() }()

	if err := copyTableRowsTx(ctx, src, tx, table); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit copy of %s: %w", table, err)
	}

	return nil
}

func copyTableRowsTx(ctx context.Context, src *sql.DB, dest *sql.Tx, table string) error {
	rows, err := src.QueryContext(ctx, fmt.Sprintf("SELECT * FROM %s", table))
	if err != nil {
		return fmt.Errorf("query rows for %s: %w", table, err)
	}

	defer func() { _ = rows.Close() }()

	columns, err := rows.Columns()
	if err != nil {
		return fmt.Errorf("list columns for %s: %w", table, err)
	}

	placeholders := make([]string, len(columns))
	for i := range placeholders {
		placeholders[i] = "?"
	}

	insertStmt := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", // #nosec: G201 -- table comes from the hardcoded localOnlyTables list; columns come from sqlite metadata
		table,
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "))

	stmt, err := dest.PrepareContext(ctx, insertStmt)
	if err != nil {
		return fmt.Errorf("prepare insert for %s: %w", table, err)
	}

	defer func() { _ = stmt.Close() }()

	values := make([]any, len(columns))

	scanArgs := make([]any, len(columns))
	for i := range values {
		scanArgs[i] = &values[i]
	}

	for rows.Next() {
		if err := rows.Scan(scanArgs...); err != nil {
			return fmt.Errorf("scan row for %s: %w", table, err)
		}

		if _, err := stmt.ExecContext(ctx, values...); err != nil {
			return fmt.Errorf("insert row for %s: %w", table, err)
		}
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate rows for %s: %w", table, err)
	}

	return nil
}

// Restore replaces the database file with the contents of the backup tar.gz
// in backupFile. The database is replicated via Raft through a CmdRestore log
// entry so followers stay in sync.
func (db *Database) Restore(ctx context.Context, backupFile *os.File) error {
	// Concurrency guard: only one restore at a time.
	if !db.restoreMu.TryLock() {
		return ErrRestoreInProgress
	}
	defer db.restoreMu.Unlock()

	_, span := tracer.Start(ctx, "db/restore", trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	if db.conn == nil {
		return fmt.Errorf("database connection is not initialized")
	}

	if backupFile == nil {
		return fmt.Errorf("backup file is nil")
	}

	if _, err := backupFile.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("failed to rewind backup file: %w", err)
	}

	// Stage the archive and validate the embedded SQLite file.
	stageDir, err := os.MkdirTemp(db.Dir(), "restore-stage-*")
	if err != nil {
		return fmt.Errorf("failed to create restore stage directory: %w", err)
	}

	defer func() { _ = os.RemoveAll(stageDir) }()

	if err := extractBackupArchive(backupFile, stageDir); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidBackupFile, err)
	}

	stagedDB := filepath.Join(stageDir, DBFilename)

	if err := validateSQLiteFile(ctx, stagedDB); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidBackupFile, err)
	}

	// The database bytes are carried through the Raft log so that followers
	// and future replays of the log reconstruct the same state.
	dbBytes, err := os.ReadFile(stagedDB) // #nosec: G304 — path is under stageDir
	if err != nil {
		return fmt.Errorf("failed to read staged database: %w", err)
	}

	// Route through Raft: applyRestore swaps the file on every node and
	// re-opens the connection.
	if _, err := db.proposeIntent(ellaraft.CmdRestore, &bytesPayload{Value: dbBytes}); err != nil {
		return fmt.Errorf("propose restore: %w", err)
	}

	// CmdRestore carries the full database as a log entry. Force a snapshot
	// so the blob doesn't linger in the Raft log and get replicated to
	// followers that fall behind.
	if db.raftManager != nil {
		if err := db.raftManager.Snapshot(); err != nil {
			logger.WithTrace(ctx, logger.DBLog).Warn(
				"Failed to snapshot after restore; log retains db blob until next scheduled snapshot",
				zap.Error(err))
		}
	}

	return nil
}

// applyRestore is invoked by the FSM for each CmdRestore log entry (on the
// leader after propose, and on followers/replay). It writes the carried
// database bytes to a staged file, validates the SQLite image, atomically
// swaps it into place, reopens the connection, and re-seeds the ID counters
// so deterministic IDs pick up from MAX(id) in the restored state.
func (db *Database) applyRestore(ctx context.Context, p *bytesPayload) (any, error) {
	stagedPath := filepath.Join(db.Dir(), "restore-apply-staged.db")
	localOnlyPath := filepath.Join(db.Dir(), "restore_safety_local.db")

	if err := os.WriteFile(stagedPath, p.Value, 0o600); err != nil {
		return nil, fmt.Errorf("write staged database: %w", err)
	}

	defer func() {
		_ = os.Remove(stagedPath)
		_ = os.Remove(localOnlyPath)
	}()

	if err := validateSQLiteFile(ctx, stagedPath); err != nil {
		return nil, fmt.Errorf("validate staged database: %w", err)
	}

	if err := backupLocalOnlyTables(ctx, db.Path(), localOnlyPath); err != nil {
		return nil, fmt.Errorf("backup local-only tables before restore: %w", err)
	}

	if db.conn != nil {
		_ = db.conn.PlainDB().Close()
	}

	for _, suffix := range []string{"-wal", "-shm"} {
		_ = os.Remove(db.Path() + suffix)
	}

	if err := os.Rename(stagedPath, db.Path()); err != nil {
		return nil, fmt.Errorf("install new database: %w", err)
	}

	if err := restoreLocalOnlyTables(ctx, localOnlyPath, db.Path()); err != nil {
		return nil, fmt.Errorf("restore local-only tables: %w", err)
	}

	if err := db.Reopen(ctx); err != nil {
		return nil, fmt.Errorf("reopen database after restore apply: %w", err)
	}

	return nil, nil
}
