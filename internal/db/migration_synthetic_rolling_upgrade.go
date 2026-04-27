// Copyright 2026 Ella Networks

//go:build rolling_upgrade_test_synthetic

// Synthetic migrations for TestIntegrationHARollingUpgrade. Built with
// -tags rolling_upgrade_test_synthetic, this file appends three no-op
// migrations versioned dynamically as len(migrations)+1..+3, so real
// migrations added later auto-bump these without manual maintenance.

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
