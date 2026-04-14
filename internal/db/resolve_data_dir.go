// Copyright 2026 Ella Networks

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

const (
	// splitStageSuffix names the in-place staging directory used during the
	// legacy → split migration. Sits next to the legacy file so the rename
	// at commit time stays on the same filesystem.
	splitStageSuffix = ".split-stage"

	// legacyBackupSuffix is appended to the legacy file's basename to mark
	// it as the post-migration safety copy.
	legacyBackupSuffix = ".sqlite.bak"

	// embeddedLegacyBackupName is the name the legacy backup is moved to
	// once it lives inside the new split directory.
	embeddedLegacyBackupName = "legacy.sqlite.bak"
)

// resolveDataDir interprets dataPath and returns the directory holding
// shared.db and local.db. dataPath is treated as the directory itself once
// migration is complete:
//
//   - directory ⇒ returned as-is.
//   - regular file ⇒ legacy single-file SQLite layout; migrated in place so
//     dataPath becomes the directory afterwards.
//   - missing ⇒ either crash recovery from an interrupted migration, or a
//     fresh install (directory created).
func resolveDataDir(ctx context.Context, dataPath string) (string, error) {
	if dataPath == "" {
		return "", errors.New("db.path is empty")
	}

	info, err := os.Stat(dataPath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("failed to stat db.path %q: %w", dataPath, err)
		}

		recovered, rErr := tryRecoverInterruptedMigration(ctx, dataPath)
		if rErr != nil {
			return "", rErr
		}

		if recovered {
			return dataPath, nil
		}

		if mkErr := os.MkdirAll(dataPath, 0o750); mkErr != nil {
			return "", fmt.Errorf("failed to create database directory %q: %w", dataPath, mkErr)
		}

		logger.WithTrace(ctx, logger.DBLog).Info("Created fresh database directory", zap.String("path", dataPath))

		return dataPath, nil
	}

	if info.IsDir() {
		return dataPath, nil
	}

	logger.WithTrace(ctx, logger.DBLog).Info(
		"Detected legacy single-file SQLite database, migrating to split layout",
		zap.String("legacyFile", dataPath),
	)

	if err := migrateLegacyToSplit(ctx, dataPath); err != nil {
		return "", fmt.Errorf("failed to migrate legacy database: %w", err)
	}

	return dataPath, nil
}

// tryRecoverInterruptedMigration handles the case where dataPath does not
// exist but a previous run already moved the legacy file aside. Refusing to
// fall through to the fresh-install branch is what protects operators from
// silent data loss across restarts.
func tryRecoverInterruptedMigration(ctx context.Context, dataPath string) (bool, error) {
	backupPath := dataPath + legacyBackupSuffix
	stagePath := dataPath + splitStageSuffix

	backupInfo, backupErr := os.Stat(backupPath)
	if backupErr != nil && !errors.Is(backupErr, os.ErrNotExist) {
		return false, fmt.Errorf("failed to stat %q: %w", backupPath, backupErr)
	}

	stageInfo, stageErr := os.Stat(stagePath)
	if stageErr != nil && !errors.Is(stageErr, os.ErrNotExist) {
		return false, fmt.Errorf("failed to stat %q: %w", stagePath, stageErr)
	}

	hasBackup := backupErr == nil && backupInfo.Mode().IsRegular()
	hasStage := stageErr == nil && stageInfo.IsDir()

	if !hasBackup && !hasStage {
		return false, nil
	}

	if !hasStage {
		return false, fmt.Errorf(
			"db.path %q is missing but legacy backup %q exists; "+
				"a previous migration was interrupted. Restore %q to %q manually and retry",
			dataPath, backupPath, backupPath, dataPath,
		)
	}

	// stage dir exists. Validate it has both expected files before promoting.
	for _, name := range []string{SharedDBFilename, LocalDBFilename} {
		p := filepath.Join(stagePath, name)
		if fi, err := os.Stat(p); err != nil || !fi.Mode().IsRegular() {
			return false, fmt.Errorf(
				"interrupted migration detected: stage directory %q is missing %s; "+
					"manual recovery required (legacy backup at %q)",
				stagePath, name, backupPath,
			)
		}
	}

	logger.WithTrace(ctx, logger.DBLog).Warn(
		"Resuming interrupted legacy → split migration",
		zap.String("dataPath", dataPath),
		zap.String("stagePath", stagePath),
		zap.Bool("hasBackup", hasBackup),
	)

	if err := os.Rename(stagePath, dataPath); err != nil {
		return false, fmt.Errorf("failed to promote stage directory: %w", err)
	}

	if hasBackup {
		moveErr := os.Rename(backupPath, filepath.Join(dataPath, embeddedLegacyBackupName))
		if moveErr != nil {
			logger.WithTrace(ctx, logger.DBLog).Warn(
				"Could not move legacy backup into split directory; leaving it in place",
				zap.String("backup", backupPath),
				zap.Error(moveErr),
			)
		}
	}

	return true, nil
}

// migrateLegacyToSplit performs the one-shot legacy → two-DB migration. The
// new files are staged in a sibling directory before any rename touches the
// legacy file, so a crash mid-migration leaves the legacy file in place.
func migrateLegacyToSplit(ctx context.Context, legacyFile string) error {
	if strings.ContainsRune(legacyFile, 0) {
		return fmt.Errorf("legacy file path contains NUL byte")
	}

	stagePath := legacyFile + splitStageSuffix
	backupPath := legacyFile + legacyBackupSuffix

	// Refuse to retry on top of partial state from a previous failed run.
	if _, err := os.Stat(stagePath); err == nil {
		return fmt.Errorf("stage directory %q already exists; remove it manually after verifying its contents", stagePath)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to stat %q: %w", stagePath, err)
	}

	if _, err := os.Stat(backupPath); err == nil {
		return fmt.Errorf("legacy backup %q already exists; remove it manually after verifying its contents", backupPath)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to stat %q: %w", backupPath, err)
	}

	if err := os.Mkdir(stagePath, 0o750); err != nil {
		return fmt.Errorf("failed to create stage directory: %w", err)
	}

	cleanupStage := func() {
		_ = os.RemoveAll(stagePath)
	}

	sharedTarget := filepath.Join(stagePath, SharedDBFilename)
	localTarget := filepath.Join(stagePath, LocalDBFilename)

	if err := buildSplitTargets(ctx, legacyFile, sharedTarget, localTarget); err != nil {
		cleanupStage()
		return err
	}

	// fsync the new files and the stage directory before any visible rename.
	if err := fsyncFile(sharedTarget); err != nil {
		cleanupStage()
		return fmt.Errorf("failed to fsync shared.db: %w", err)
	}

	if err := fsyncFile(localTarget); err != nil {
		cleanupStage()
		return fmt.Errorf("failed to fsync local.db: %w", err)
	}

	if err := fsyncDir(stagePath); err != nil {
		cleanupStage()
		return fmt.Errorf("failed to fsync stage directory: %w", err)
	}

	parentDir := filepath.Dir(legacyFile)

	// Critical section: move the legacy file aside, then promote the stage
	// directory into place. A crash between these two renames is recovered
	// on the next startup by tryRecoverInterruptedMigration.
	if err := os.Rename(legacyFile, backupPath); err != nil {
		cleanupStage()
		return fmt.Errorf("failed to rename legacy file to %q: %w", legacyFile, err)
	}

	if err := os.Rename(stagePath, legacyFile); err != nil {
		// Best-effort: try to restore the legacy file so the next startup
		// sees the original layout, then drop the stage directory.
		if rbErr := os.Rename(backupPath, legacyFile); rbErr != nil {
			logger.WithTrace(ctx, logger.DBLog).Error(
				"Failed to roll back legacy file rename after failed stage promotion",
				zap.Error(rbErr),
			)
		}

		cleanupStage()

		return fmt.Errorf("failed to promote stage directory to %q: %w", legacyFile, err)
	}

	// Best effort: tuck the backup inside the new directory so the operator
	// finds it next to its replacements.
	embeddedBackup := filepath.Join(legacyFile, embeddedLegacyBackupName)
	if err := os.Rename(backupPath, embeddedBackup); err != nil {
		logger.WithTrace(ctx, logger.DBLog).Warn(
			"Could not move legacy backup into split directory; leaving it in place",
			zap.String("backup", backupPath),
			zap.Error(err),
		)
	}

	if err := fsyncDir(parentDir); err != nil {
		logger.WithTrace(ctx, logger.DBLog).Warn(
			"Failed to fsync parent directory after migration",
			zap.String("parentDir", parentDir),
			zap.Error(err),
		)
	}

	// Best-effort cleanup of stale WAL/SHM sidecars left from the legacy
	// file. They live next to the original path, which is now a directory.
	for _, suffix := range []string{"-wal", "-shm"} {
		_ = os.Remove(legacyFile + legacyBackupSuffix + suffix)
	}

	logger.WithTrace(ctx, logger.DBLog).Info(
		"Legacy database migrated to split layout",
		zap.String("dataDir", legacyFile),
	)

	return nil
}

// buildSplitTargets brings the legacy file up to v8, then writes the split
// shared.db and local.db inside the stage directory.
func buildSplitTargets(ctx context.Context, legacyFile, sharedTarget, localTarget string) error {
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

	// Close so ATTACH DATABASE on the targets can open the legacy file
	// independently (go-sqlite3 checkpoints WAL on close).
	if err := legacyConn.Close(); err != nil {
		return fmt.Errorf("failed to close legacy database: %w", err)
	}

	sharedConn, err := openSQLiteConnection(ctx, sharedTarget, SyncFull)
	if err != nil {
		return fmt.Errorf("failed to open new shared.db: %w", err)
	}

	if err := runSharedMigrations(ctx, sharedConn); err != nil {
		_ = sharedConn.Close()
		return fmt.Errorf("shared split-baseline migration failed: %w", err)
	}

	if err := copyTablesViaAttach(ctx, sharedConn, legacyFile, sharedTablesInLegacyOrder); err != nil {
		_ = sharedConn.Close()
		return fmt.Errorf("failed to copy shared tables: %w", err)
	}

	if err := sharedConn.Close(); err != nil {
		return fmt.Errorf("failed to close shared.db after copy: %w", err)
	}

	localConn, err := openSQLiteConnection(ctx, localTarget, SyncNormal)
	if err != nil {
		return fmt.Errorf("failed to open new local.db: %w", err)
	}

	if err := runLocalMigrations(ctx, localConn); err != nil {
		_ = localConn.Close()
		return fmt.Errorf("local split-baseline migration failed: %w", err)
	}

	if err := copyTablesViaAttach(ctx, localConn, legacyFile, localTablesInLegacyOrder); err != nil {
		_ = localConn.Close()
		return fmt.Errorf("failed to copy local tables: %w", err)
	}

	if err := localConn.Close(); err != nil {
		return fmt.Errorf("failed to close local.db after copy: %w", err)
	}

	if err := verifyRowCounts(ctx, legacyFile, sharedTarget, sharedTablesInLegacyOrder); err != nil {
		return fmt.Errorf("shared row count verification failed: %w", err)
	}

	if err := verifyRowCounts(ctx, legacyFile, localTarget, localTablesInLegacyOrder); err != nil {
		return fmt.Errorf("local row count verification failed: %w", err)
	}

	return nil
}

// copyTablesViaAttach ATTACHes the legacy file and copies the listed tables
// into the target within a single transaction. FKs are disabled on the target
// connection as defence-in-depth (the table list is already parents-first).
func copyTablesViaAttach(ctx context.Context, targetConn *sql.DB, legacyFile string, tables []string) error {
	if strings.ContainsRune(legacyFile, 0) {
		return fmt.Errorf("legacy file path contains NUL byte")
	}

	if _, err := targetConn.ExecContext(ctx, "PRAGMA foreign_keys = OFF"); err != nil {
		return fmt.Errorf("failed to disable foreign keys: %w", err)
	}

	defer func() {
		_, _ = targetConn.ExecContext(ctx, "PRAGMA foreign_keys = ON")
	}()

	// ATTACH DATABASE accepts the filename as a bound parameter, so we
	// avoid string interpolation entirely.
	if _, err := targetConn.ExecContext(ctx, "ATTACH DATABASE ? AS legacy", legacyFile); err != nil {
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
		// Discover the legacy table's columns so the INSERT names them
		// explicitly. New columns added in the split DDL (that don't exist
		// in the legacy schema) fill from their DEFAULT values.
		cols, cErr := legacyColumnNames(ctx, tx, table)
		if cErr != nil {
			_ = tx.Rollback()
			return fmt.Errorf("failed to read columns for table %q: %w", table, cErr)
		}

		colList := strings.Join(cols, ", ")
		// Identifier-only interpolation of hard-coded table names and
		// column names read from PRAGMA — no user input reaches this string.
		stmt := fmt.Sprintf("INSERT INTO main.%s (%s) SELECT %s FROM legacy.%s", table, colList, colList, table) // #nosec: G201

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

// legacyColumnNames returns the column names of the given table in the
// "legacy" attached database. The caller must already be inside a transaction
// that has the legacy DB attached.
func legacyColumnNames(ctx context.Context, tx *sql.Tx, table string) ([]string, error) {
	// PRAGMA table_info works through ATTACH aliases.
	rows, err := tx.QueryContext(ctx, fmt.Sprintf("PRAGMA legacy.table_info(%s)", table)) // #nosec: G201
	if err != nil {
		return nil, err
	}

	defer func() { _ = rows.Close() }()

	var cols []string

	for rows.Next() {
		var (
			cid           int
			name, colType string
			notNull       int
			dfltValue     *string
			pk            int
		)

		if err := rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk); err != nil {
			return nil, err
		}

		cols = append(cols, name)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(cols) == 0 {
		return nil, fmt.Errorf("table %q has no columns (or does not exist)", table)
	}

	return cols, nil
}

// verifyRowCounts confirms that every table in the target database has the
// same row count as in the legacy database.
func verifyRowCounts(ctx context.Context, legacyFile, targetFile string, tables []string) error {
	legacyConn, err := openSQLiteConnection(ctx, legacyFile, SyncFull)
	if err != nil {
		return fmt.Errorf("failed to open legacy for verify: %w", err)
	}

	defer func() { _ = legacyConn.Close() }()

	targetConn, err := openSQLiteConnection(ctx, targetFile, SyncFull)
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

// fsyncDir flushes a directory entry to disk so that renames inside it are
// durable across crashes.
func fsyncDir(path string) error {
	d, err := os.Open(path) // #nosec: G304
	if err != nil {
		return err
	}

	syncErr := d.Sync()
	closeErr := d.Close()

	if syncErr != nil {
		return syncErr
	}

	return closeErr
}
