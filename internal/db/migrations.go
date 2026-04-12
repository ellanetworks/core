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

// legacyMigrations is the FROZEN list of pre-split single-database migrations.
// Only invoked when upgrading a legacy single-file database via resolveDataDir;
// never run against shared.db or local.db. Append-never; entries are immutable.
// New schema changes belong in sharedMigrations or localMigrations.
var legacyMigrations = []migration{
	{1, "baseline schema", migrateV1},
	{2, "add NAS security columns, home network keys table, and SPN columns", migrateV2},
	{3, "add radio_name column to network_logs", migrateV3},
	{4, "add bgp_settings, bgp_peers, jwt_secret, ip_leases tables; drop ipAddress from subscribers", migrateV4},
	{5, "add network_rules and policy_network_rules tables", migrateV5},
	{6, "replace address TEXT with addressBin BLOB in ip_leases", migrateV6},
	{7, "data model redesign: profiles, policies, slices", migrateV7},
	{8, "add action to flow reports", migrateV8},
}

// sharedMigrations is the append-only schema migration registry for shared.db.
// v1 is the canonical end state of legacyMigrations v1..v8 for shared tables.
// Versions are sequential from 1 with no gaps; shipped entries are immutable.
var sharedMigrations = []migration{
	{1, "split baseline (shared)", migrateSharedV1},
}

// localMigrations is the append-only schema migration registry for local.db
// (network_logs, flow_reports).
var localMigrations = []migration{
	{1, "split baseline (local)", migrateLocalV1},
}

// runMigrations applies the given migration registry against sqlConn. The
// schema_version table is local to each database file, so shared and local
// each track their own version counter independently.
func runMigrations(ctx context.Context, sqlConn *sql.DB, registry []migration, label string) error {
	// Validate registry invariants.
	for i, m := range registry {
		if m.version != i+1 {
			return fmt.Errorf("%s migration registry error: expected version %d at index %d, got %d", label, i+1, i, m.version)
		}

		if m.fn == nil {
			return fmt.Errorf("%s migration registry error: migration %d has nil function", label, m.version)
		}
	}

	// Create the version tracking table (idempotent). The CHECK constraint
	// enforces exactly one row (singleton).
	_, err := sqlConn.ExecContext(ctx,
		`CREATE TABLE IF NOT EXISTS schema_version (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			version INTEGER NOT NULL
		)`)
	if err != nil {
		return fmt.Errorf("failed to create schema_version table: %w", err)
	}

	// Seed version 0 if no row exists.
	if _, err := sqlConn.ExecContext(ctx,
		"INSERT OR IGNORE INTO schema_version (id, version) VALUES (1, 0)"); err != nil {
		return fmt.Errorf("failed to seed schema_version: %w", err)
	}

	current := 0

	row := sqlConn.QueryRowContext(ctx, "SELECT version FROM schema_version WHERE id = 1")
	if err := row.Scan(&current); err != nil {
		return fmt.Errorf("failed to read schema version: %w", err)
	}

	for _, m := range registry {
		if m.version <= current {
			continue
		}

		logger.DBLog.Info("Applying migration",
			zap.String("registry", label),
			zap.Int("version", m.version),
			zap.String("description", m.description),
		)

		// PRAGMA foreign_keys is a no-op inside a transaction, so disable
		// FK enforcement on the connection before starting the migration tx.
		// This prevents DROP TABLE from cascade-deleting child rows during
		// table rebuilds. Re-enabled after commit.
		if _, err := sqlConn.ExecContext(ctx, "PRAGMA foreign_keys = OFF"); err != nil {
			return fmt.Errorf("failed to disable foreign keys for %s migration %d: %w", label, m.version, err)
		}

		tx, err := sqlConn.BeginTx(ctx, &sql.TxOptions{})
		if err != nil {
			return fmt.Errorf("failed to begin transaction for %s migration %d: %w", label, m.version, err)
		}

		if err := m.fn(ctx, tx); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("%s migration %d (%s) failed: %w", label, m.version, m.description, err)
		}

		if _, err := tx.ExecContext(ctx,
			"UPDATE schema_version SET version = ? WHERE id = 1", m.version); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("failed to update %s schema_version to %d: %w", label, m.version, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit %s migration %d: %w", label, m.version, err)
		}

		if _, err := sqlConn.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
			return fmt.Errorf("failed to re-enable foreign keys after %s migration %d: %w", label, m.version, err)
		}

		logger.DBLog.Info("Migration applied successfully",
			zap.String("registry", label),
			zap.Int("version", m.version),
			zap.String("description", m.description),
		)
	}

	return nil
}

// runSharedMigrations brings shared.db up to the latest sharedMigrations version.
func runSharedMigrations(ctx context.Context, sqlConn *sql.DB) error {
	return runMigrations(ctx, sqlConn, sharedMigrations, "shared")
}

// runLocalMigrations brings local.db up to the latest localMigrations version.
func runLocalMigrations(ctx context.Context, sqlConn *sql.DB) error {
	return runMigrations(ctx, sqlConn, localMigrations, "local")
}

// runLegacyMigrations brings a pre-split single-file database up to the v8
// frozen end state. Only used during one-shot legacy → split migration.
func runLegacyMigrations(ctx context.Context, sqlConn *sql.DB) error {
	return runMigrations(ctx, sqlConn, legacyMigrations, "legacy")
}
