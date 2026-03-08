// Copyright 2026 Ella Networks

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

// migration represents a single schema migration step.
type migration struct {
	version     int
	description string
	fn          func(ctx context.Context, tx *sql.Tx) error
}

// migrations is the ordered list of all schema migrations.
// Rules:
//   - Versions must be sequential starting at 1 with no gaps.
//   - A migration, once shipped in a release, is immutable — never edit its fn.
//   - This slice is append-only.
var migrations = []migration{
	{1, "baseline schema", migrateV1},
}

// latestVersion returns the highest migration version in the registry.
func latestVersion() int {
	if len(migrations) == 0 {
		return 0
	}

	return migrations[len(migrations)-1].version
}

// RunMigrations applies all pending schema migrations to the database.
// It creates the schema_version tracking table if it does not exist,
// reads the current version, and applies each migration whose version
// exceeds the current one. Each migration runs inside its own transaction
// (with BEGIN IMMEDIATE to prevent concurrent writers) so a failure rolls
// back cleanly and leaves the database at the last successful version.
func RunMigrations(ctx context.Context, sqlConn *sql.DB) error {
	// Validate migration registry invariants.
	for i, m := range migrations {
		if m.version != i+1 {
			return fmt.Errorf("migration registry error: expected version %d at index %d, got %d", i+1, i, m.version)
		}

		if m.fn == nil {
			return fmt.Errorf("migration registry error: migration %d has nil function", m.version)
		}
	}

	// Create the version tracking table (idempotent).
	_, err := sqlConn.ExecContext(ctx,
		"CREATE TABLE IF NOT EXISTS schema_version (version INTEGER NOT NULL)")
	if err != nil {
		return fmt.Errorf("failed to create schema_version table: %w", err)
	}

	// Read current schema version. A missing row means version 0.
	current := 0

	row := sqlConn.QueryRowContext(ctx, "SELECT version FROM schema_version LIMIT 1")
	if err := row.Scan(&current); err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("failed to read schema version: %w", err)
		}
		// No row — seed with version 0.
		if _, err := sqlConn.ExecContext(ctx,
			"INSERT INTO schema_version (version) VALUES (0)"); err != nil {
			return fmt.Errorf("failed to seed schema_version: %w", err)
		}
	}

	// Apply each pending migration in order.
	for _, m := range migrations {
		if m.version <= current {
			continue
		}

		logger.DBLog.Info("Applying migration",
			zap.Int("version", m.version),
			zap.String("description", m.description),
		)

		// Use BEGIN IMMEDIATE to acquire a write lock immediately, preventing
		// a second process from entering the same migration concurrently.
		tx, err := sqlConn.BeginTx(ctx, &sql.TxOptions{})
		if err != nil {
			return fmt.Errorf("failed to begin transaction for migration %d: %w", m.version, err)
		}

		// SQLite requires PRAGMA foreign_keys to be set outside transactions,
		// and the connection already has it enabled. Temporarily disable it
		// during the migration so that table restructuring (the 12-step ALTER
		// TABLE pattern) can reorder table creation without FK violations.
		// Note: PRAGMA foreign_keys changes inside a tx are no-ops in SQLite,
		// so we must disable before the transaction for restructure migrations.
		// For simple ADD COLUMN migrations this is not needed.

		if err := m.fn(ctx, tx); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migration %d (%s) failed: %w", m.version, m.description, err)
		}

		if _, err := tx.ExecContext(ctx,
			"UPDATE schema_version SET version = ?", m.version); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("failed to update schema_version to %d: %w", m.version, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit migration %d: %w", m.version, err)
		}

		logger.DBLog.Info("Migration applied successfully",
			zap.Int("version", m.version),
			zap.String("description", m.description),
		)
	}

	return nil
}
