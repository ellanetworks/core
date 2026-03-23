// Copyright 2026 Ella Networks

package db

import (
	"context"
	"database/sql"
	"fmt"
)

// ---------------------------------------------------------------------------
// V5 migration — add network_rules table
// ---------------------------------------------------------------------------

func migrateV5(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			policy_id INTEGER NOT NULL,
			description TEXT NOT NULL,
			direction TEXT NOT NULL,
			remote_prefix TEXT,
			protocol INTEGER DEFAULT 0,
			port_low INTEGER DEFAULT 0,
			port_high INTEGER DEFAULT 0,
			action TEXT NOT NULL,
			precedence INTEGER NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (policy_id) REFERENCES %s (id) ON DELETE CASCADE,
			UNIQUE(policy_id, precedence, direction)
		)`, NetworkRulesTableName, PoliciesTableName))
	if err != nil {
		return fmt.Errorf("failed to create network_rules table: %w", err)
	}

	return nil
}
