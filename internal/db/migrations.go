// Copyright 2026 Ella Networks

package db

import (
	"context"
	"database/sql"
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
	{2, "add NAS security columns, home network keys table, and SPN columns", migrateV2},
	{3, "add radio_name column to network_logs", migrateV3},
	{4, "add bgp_settings, bgp_peers, jwt_secret, ip_leases tables; drop ipAddress from subscribers", migrateV4},
	{5, "add network_rules and policy_network_rules tables", migrateV5},
	{6, "replace address TEXT with addressBin BLOB in ip_leases", migrateV6},
	{7, "data model redesign: profiles, policies, slices", migrateV7},
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
// exceeds the current one. Each migration runs inside its own IMMEDIATE
// transaction (via the _txlock=immediate DSN parameter) so a failure
// rolls back cleanly and leaves the database at the last successful version.
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
	// The CHECK constraint enforces exactly one row (singleton).
	_, err := sqlConn.ExecContext(ctx,
		`CREATE TABLE IF NOT EXISTS schema_version (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			version INTEGER NOT NULL
		)`)
	if err != nil {
		return fmt.Errorf("failed to create schema_version table: %w", err)
	}

	// Seed version 0 if no row exists. INSERT OR IGNORE is atomic and
	// safe against concurrent execution (the singleton PK prevents dupes).
	if _, err := sqlConn.ExecContext(ctx,
		"INSERT OR IGNORE INTO schema_version (id, version) VALUES (1, 0)"); err != nil {
		return fmt.Errorf("failed to seed schema_version: %w", err)
	}

	// Read current schema version.
	current := 0

	row := sqlConn.QueryRowContext(ctx, "SELECT version FROM schema_version WHERE id = 1")
	if err := row.Scan(&current); err != nil {
		return fmt.Errorf("failed to read schema version: %w", err)
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

		// PRAGMA foreign_keys is a no-op inside a transaction, so disable
		// FK enforcement on the connection before starting the migration tx.
		// This prevents DROP TABLE from cascade-deleting child rows during
		// table rebuilds. Re-enabled after commit.
		if _, err := sqlConn.ExecContext(ctx, "PRAGMA foreign_keys = OFF"); err != nil {
			return fmt.Errorf("failed to disable foreign keys for migration %d: %w", m.version, err)
		}

		// The connection uses _txlock=immediate, so BeginTx acquires a
		// write lock up front, preventing a second process from entering
		// the same migration concurrently.
		tx, err := sqlConn.BeginTx(ctx, &sql.TxOptions{})
		if err != nil {
			return fmt.Errorf("failed to begin transaction for migration %d: %w", m.version, err)
		}

		if err := m.fn(ctx, tx); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migration %d (%s) failed: %w", m.version, m.description, err)
		}

		if _, err := tx.ExecContext(ctx,
			"UPDATE schema_version SET version = ? WHERE id = 1", m.version); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("failed to update schema_version to %d: %w", m.version, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit migration %d: %w", m.version, err)
		}

		if _, err := sqlConn.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
			return fmt.Errorf("failed to re-enable foreign keys after migration %d: %w", m.version, err)
		}

		logger.DBLog.Info("Migration applied successfully",
			zap.Int("version", m.version),
			zap.String("description", m.description),
		)
	}

	return nil
}
