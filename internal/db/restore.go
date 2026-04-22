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

	"go.opentelemetry.io/otel/trace"
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

// extractBackupArchive reads a backup tar.gz from r and writes the
// database file into destDir. The archive carries exactly two members:
// manifest.json and ella.db. Unknown members, missing required members,
// oversize files, duplicate entries, and path traversal attempts are
// rejected.
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

		// Reject path traversal.
		if strings.Contains(hdr.Name, "..") || strings.HasPrefix(hdr.Name, "/") {
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

			if m.Version < 1 || m.Version > BackupManifestVersion {
				return fmt.Errorf("unsupported backup manifest version %d", m.Version)
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

// ExtractForRestore extracts a backup bundle into destDir. Used by the
// offline first-boot recovery path before db.NewDatabase has run.
func ExtractForRestore(bundlePath, destDir string) error {
	f, err := os.Open(bundlePath) // #nosec: G304 -- path comes from the operator via fixed-path convention
	if err != nil {
		return fmt.Errorf("open bundle: %w", err)
	}

	defer func() { _ = f.Close() }()

	if err := os.MkdirAll(destDir, 0o700); err != nil {
		return fmt.Errorf("mkdir %s: %w", destDir, err)
	}

	return extractBackupArchive(f, destDir)
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
			continue
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
		var exists int
		if err := tx.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&exists); err != nil || exists == 0 {
			continue
		}

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

// BackupLocalTables implements ellaraft.Applier. It copies local-only tables
// from srcPath into destPath so they survive a full database file swap.
func (db *Database) BackupLocalTables(ctx context.Context, srcPath, destPath string) error {
	return backupLocalOnlyTables(ctx, srcPath, destPath)
}

// RestoreLocalTables implements ellaraft.Applier. It copies previously
// backed-up local-only tables from backupPath back into destPath.
func (db *Database) RestoreLocalTables(ctx context.Context, backupPath, destPath string) error {
	return restoreLocalOnlyTables(ctx, backupPath, destPath)
}

// Restore replaces the database file with the contents of the backup tar.gz
// in backupFile. In HA mode, the restored database is fed to raft as an
// external snapshot via raft.Restore; followers receive it via InstallSnapshot.
// In standalone mode, the file is swapped directly.
func (db *Database) Restore(ctx context.Context, backupFile *os.File) error {
	// Concurrency guard: only one restore at a time.
	if !db.restoreMu.TryLock() {
		return ErrRestoreInProgress
	}
	defer db.restoreMu.Unlock()

	_, span := tracer.Start(ctx, "db/restore", trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	if db.conn() == nil {
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

	// Stream the validated SQLite file as an external snapshot into Raft.
	// raft.Restore injects the snapshot, bumps the index past commitIndex,
	// and triggers FSM.Restore on every node (which handles local-table
	// preservation, file swap, and connection reopening). In HA mode the
	// leader also replicates the snapshot to followers via InstallSnapshot.
	f, err := os.Open(stagedDB) // #nosec: G304 — path is under stageDir
	if err != nil {
		return fmt.Errorf("open staged database for restore: %w", err)
	}

	defer func() { _ = f.Close() }()

	info, err := f.Stat()
	if err != nil {
		return fmt.Errorf("stat staged database: %w", err)
	}

	if err := db.raftManager.UserRestore(f, info.Size(), db.proposeTimeout); err != nil {
		return fmt.Errorf("raft restore: %w", err)
	}

	return nil
}
