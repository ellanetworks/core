// Copyright 2026 Ella Networks

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

// sharedTablesInLegacyOrder is the FROZEN list of tables copied from a legacy
// database into shared.db, ordered parents-before-children for FK references.
// MUST stay in sync with migrateSharedV1.
var sharedTablesInLegacyOrder = []string{
	"operator",
	"network_slices",
	"profiles",
	"data_networks",
	"policies",
	"network_rules",
	"subscribers",
	"daily_usage",
	"ip_leases",
	"home_network_keys",
	"users",
	"sessions",
	"api_tokens",
	"jwt_secret",
	"bgp_settings",
	"bgp_peers",
	"bgp_import_prefixes",
	"routes",
	"nat_settings",
	"n3_settings",
	"flow_accounting_settings",
	"retention_policies",
	"audit_logs",
}

// localTablesInLegacyOrder lists the tables copied from a legacy database into
// local.db. MUST stay in sync with migrateLocalV1.
var localTablesInLegacyOrder = []string{
	"network_logs",
	"flow_reports",
}

// legacyV8Version is the schema_version a legacy single-file database must
// reach before it can be split.
const legacyV8Version = 8

// resolveDataDir interprets dataPath and returns the directory holding
// shared.db and local.db:
//
//   - directory ⇒ returned as-is.
//   - regular file ⇒ legacy single-SQLite layout; migrated into the directory
//     containing it.
//   - missing ⇒ fresh install: directory is created.
func resolveDataDir(ctx context.Context, dataPath string) (string, error) {
	if dataPath == "" {
		return "", errors.New("db.path is empty")
	}

	info, err := os.Stat(dataPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if mkErr := os.MkdirAll(dataPath, 0o750); mkErr != nil {
				return "", fmt.Errorf("failed to create database directory %q: %w", dataPath, mkErr)
			}

			logger.WithTrace(ctx, logger.DBLog).Info("Created fresh database directory", zap.String("path", dataPath))

			return dataPath, nil
		}

		return "", fmt.Errorf("failed to stat db.path %q: %w", dataPath, err)
	}

	if info.IsDir() {
		return dataPath, nil
	}

	legacyDir := filepath.Dir(dataPath)

	logger.WithTrace(ctx, logger.DBLog).Info(
		"Detected legacy single-file SQLite database, migrating to split layout",
		zap.String("legacyFile", dataPath),
		zap.String("dataDir", legacyDir),
	)

	if err := migrateLegacyToSplit(ctx, dataPath, legacyDir); err != nil {
		return "", fmt.Errorf("failed to migrate legacy database: %w", err)
	}

	return legacyDir, nil
}

// migrateLegacyToSplit performs the one-shot legacy → two-DB migration.
func migrateLegacyToSplit(ctx context.Context, legacyFile, dataDir string) error {
	sharedTarget := filepath.Join(dataDir, SharedDBFilename)
	localTarget := filepath.Join(dataDir, LocalDBFilename)

	// Refuse to retry on top of partial output: a previous crashed run could
	// leave one of the targets behind, and rerunning would double-insert rows.
	for _, p := range []string{sharedTarget, localTarget} {
		if _, err := os.Stat(p); err == nil {
			return fmt.Errorf("both legacy file %q and target %q exist; remove the target manually after verifying its contents", legacyFile, p)
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("failed to stat target %q: %w", p, err)
		}
	}

	// Bring the legacy file up to v8.
	legacyConn, err := openSQLiteConnection(ctx, legacyFile, SyncFull)
	if err != nil {
		return fmt.Errorf("failed to open legacy database: %w", err)
	}

	if err := runLegacyMigrations(ctx, legacyConn); err != nil {
		_ = legacyConn.Close()
		return fmt.Errorf("legacy migrations failed: %w", err)
	}

	var legacyVersion int

	if err := legacyConn.QueryRowContext(ctx,
		"SELECT version FROM schema_version WHERE id = 1").Scan(&legacyVersion); err != nil {
		_ = legacyConn.Close()
		return fmt.Errorf("failed to read legacy schema_version: %w", err)
	}

	if legacyVersion != legacyV8Version {
		_ = legacyConn.Close()
		return fmt.Errorf("legacy database at version %d, expected %d", legacyVersion, legacyV8Version)
	}

	// Close the legacy connection so ATTACH DATABASE on the targets can open
	// it independently (go-sqlite3 checkpoints WAL on close).
	if err := legacyConn.Close(); err != nil {
		return fmt.Errorf("failed to close legacy database: %w", err)
	}

	// Create empty shared/local DBs at the split-baseline.
	sharedConn, err := openSQLiteConnection(ctx, sharedTarget, SyncFull)
	if err != nil {
		_ = os.Remove(sharedTarget)
		return fmt.Errorf("failed to open new shared.db: %w", err)
	}

	if err := RunSharedMigrations(ctx, sharedConn); err != nil {
		_ = sharedConn.Close()
		_ = os.Remove(sharedTarget)

		return fmt.Errorf("shared split-baseline migration failed: %w", err)
	}

	if err := copyTablesViaAttach(ctx, sharedConn, legacyFile, sharedTablesInLegacyOrder); err != nil {
		_ = sharedConn.Close()
		_ = os.Remove(sharedTarget)

		return fmt.Errorf("failed to copy shared tables: %w", err)
	}

	if err := sharedConn.Close(); err != nil {
		_ = os.Remove(sharedTarget)
		return fmt.Errorf("failed to close shared.db after copy: %w", err)
	}

	localConn, err := openSQLiteConnection(ctx, localTarget, SyncNormal)
	if err != nil {
		_ = os.Remove(sharedTarget)
		_ = os.Remove(localTarget)

		return fmt.Errorf("failed to open new local.db: %w", err)
	}

	if err := RunLocalMigrations(ctx, localConn); err != nil {
		_ = localConn.Close()
		_ = os.Remove(sharedTarget)
		_ = os.Remove(localTarget)

		return fmt.Errorf("local split-baseline migration failed: %w", err)
	}

	if err := copyTablesViaAttach(ctx, localConn, legacyFile, localTablesInLegacyOrder); err != nil {
		_ = localConn.Close()
		_ = os.Remove(sharedTarget)
		_ = os.Remove(localTarget)

		return fmt.Errorf("failed to copy local tables: %w", err)
	}

	if err := localConn.Close(); err != nil {
		_ = os.Remove(sharedTarget)
		_ = os.Remove(localTarget)

		return fmt.Errorf("failed to close local.db after copy: %w", err)
	}

	if err := verifyRowCounts(ctx, legacyFile, sharedTarget, sharedTablesInLegacyOrder); err != nil {
		_ = os.Remove(sharedTarget)
		_ = os.Remove(localTarget)

		return fmt.Errorf("shared row count verification failed: %w", err)
	}

	if err := verifyRowCounts(ctx, legacyFile, localTarget, localTablesInLegacyOrder); err != nil {
		_ = os.Remove(sharedTarget)
		_ = os.Remove(localTarget)

		return fmt.Errorf("local row count verification failed: %w", err)
	}

	// fsync the new files before renaming the legacy file out of the way.
	if err := fsyncFile(sharedTarget); err != nil {
		return fmt.Errorf("failed to fsync shared.db: %w", err)
	}

	if err := fsyncFile(localTarget); err != nil {
		return fmt.Errorf("failed to fsync local.db: %w", err)
	}

	backupPath := legacyBackupName(legacyFile)
	if err := os.Rename(legacyFile, backupPath); err != nil {
		return fmt.Errorf("failed to rename legacy file to %q: %w", backupPath, err)
	}

	// Best-effort cleanup of stale WAL/SHM sidecars.
	for _, suffix := range []string{"-wal", "-shm"} {
		_ = os.Remove(legacyFile + suffix)
	}

	logger.WithTrace(ctx, logger.DBLog).Info(
		"Legacy database migrated to split layout",
		zap.String("backup", backupPath),
	)

	return nil
}

// copyTablesViaAttach ATTACHes the legacy file and copies the listed tables
// into the target within a single transaction. FKs are disabled on the target
// connection as defence-in-depth (the table list is already parents-first).
func copyTablesViaAttach(ctx context.Context, targetConn *sql.DB, legacyFile string, tables []string) error {
	if _, err := targetConn.ExecContext(ctx, "PRAGMA foreign_keys = OFF"); err != nil {
		return fmt.Errorf("failed to disable foreign keys: %w", err)
	}

	defer func() {
		_, _ = targetConn.ExecContext(ctx, "PRAGMA foreign_keys = ON")
	}()

	// ATTACH cannot be parameterised; the legacy file path comes from
	// trusted local config / OS-level startup, never user input.
	attachStmt := fmt.Sprintf("ATTACH DATABASE '%s' AS legacy", escapeSQLString(legacyFile))
	if _, err := targetConn.ExecContext(ctx, attachStmt); err != nil {
		return fmt.Errorf("failed to attach legacy database: %w", err)
	}

	defer func() {
		_, _ = targetConn.ExecContext(ctx, "DETACH DATABASE legacy")
	}()

	tx, err := targetConn.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin tx: %w", err)
	}

	for _, table := range tables {
		// Identifier-only interpolation of a hard-coded table name list —
		// no user input reaches this string.
		stmt := fmt.Sprintf("INSERT INTO main.%s SELECT * FROM legacy.%s", table, table) // #nosec: G201

		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("failed to copy table %q: %w", table, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit copy tx: %w", err)
	}

	return nil
}

// verifyRowCounts confirms that every table in the target database has the
// same row count as in the legacy database.
func verifyRowCounts(ctx context.Context, legacyFile, targetFile string, tables []string) error {
	legacyConn, err := sql.Open("sqlite3", legacyFile+"?_txlock=immediate")
	if err != nil {
		return fmt.Errorf("failed to open legacy for verify: %w", err)
	}

	defer func() { _ = legacyConn.Close() }()

	targetConn, err := sql.Open("sqlite3", targetFile+"?_txlock=immediate")
	if err != nil {
		return fmt.Errorf("failed to open target for verify: %w", err)
	}

	defer func() { _ = targetConn.Close() }()

	for _, table := range tables {
		var legacyCount, targetCount int64

		if err := legacyConn.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s", table)).Scan(&legacyCount); err != nil {
			return fmt.Errorf("failed to count legacy.%s: %w", table, err)
		}

		if err := targetConn.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s", table)).Scan(&targetCount); err != nil {
			return fmt.Errorf("failed to count target.%s: %w", table, err)
		}

		if legacyCount != targetCount {
			return fmt.Errorf("row count mismatch on %s: legacy=%d target=%d", table, legacyCount, targetCount)
		}
	}

	return nil
}

// fsyncFile flushes a file to disk.
func fsyncFile(path string) error {
	f, err := os.OpenFile(path, os.O_RDWR, 0o600) // #nosec: G304
	if err != nil {
		return err
	}

	syncErr := f.Sync()
	closeErr := f.Close()

	if syncErr != nil {
		return syncErr
	}

	return closeErr
}

// legacyBackupName returns the post-migration name of the legacy file.
func legacyBackupName(legacyFile string) string {
	return legacyFile + ".sqlite.bak"
}

// escapeSQLString doubles single quotes for use in a SQL string literal.
func escapeSQLString(s string) string {
	out := make([]byte, 0, len(s))

	for i := 0; i < len(s); i++ {
		if s[i] == '\'' {
			out = append(out, '\'', '\'')
			continue
		}

		out = append(out, s[i])
	}

	return string(out)
}
