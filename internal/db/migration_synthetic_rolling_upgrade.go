// Copyright 2026 Ella Networks

//go:build rolling_upgrade_test_synthetic

// Synthetic migrations for the rolling-upgrade integration test.
//
// Built with `-tags rolling_upgrade_test_synthetic`, this file appends
// three trivial no-op migrations to the registry — each adding a
// disposable column to the subscribers table. The version numbers are
// computed dynamically as `len(migrations)+1..+3` at init time, so
// when real migrations are added in the future the synthetic versions
// auto-track without manual maintenance.
//
// The integration test uses these to validate the rolling-upgrade
// machinery end-to-end:
//
//   - A baseline image (no tag) reports SchemaVersion() = N.
//   - A target image (with tag) reports SchemaVersion() = N+3.
//   - Swapping nodes one at a time exercises the post-baseline
//     migration gate (CheckPendingMigrations) and the visibility
//     fields (cluster.appliedSchemaVersion, cluster.pendingMigration).
//
// The migrations are otherwise inert: they add columns, never read
// from them, and ship with no associated ops. The lock-file test
// (TestOperationsRegistry_AppendOnly) is unaffected because no new op
// is registered.

package db

import (
	"context"
	"database/sql"
	"fmt"
)

func init() {
	base := len(migrations)

	migrations = append(migrations,
		migration{
			version:     base + 1,
			description: fmt.Sprintf("synthetic v%d for rolling-upgrade test", base+1),
			fn:          syntheticRollingUpgradeMigrate(base + 1),
		},
		migration{
			version:     base + 2,
			description: fmt.Sprintf("synthetic v%d for rolling-upgrade test", base+2),
			fn:          syntheticRollingUpgradeMigrate(base + 2),
		},
		migration{
			version:     base + 3,
			description: fmt.Sprintf("synthetic v%d for rolling-upgrade test", base+3),
			fn:          syntheticRollingUpgradeMigrate(base + 3),
		},
	)
}

// syntheticRollingUpgradeMigrate returns a migration function that
// adds a uniquely-named disposable column to the subscribers table.
// The column is never read; the migration's only job is to make the
// schema version advance and to exercise the apply path.
func syntheticRollingUpgradeMigrate(version int) func(context.Context, *sql.Tx) error {
	return func(ctx context.Context, tx *sql.Tx) error {
		col := fmt.Sprintf("rolling_upgrade_synth_v%d", version)
		stmt := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s INTEGER NOT NULL DEFAULT 0",
			SubscribersTableName, col)

		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("synthetic migration v%d: %w", version, err)
		}

		return nil
	}
}
