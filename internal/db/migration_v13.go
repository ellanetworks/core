// Copyright 2026 Ella Networks

package db

import (
	"context"
	"database/sql"
	"fmt"
)

// V13 adds IPv6 pool support to data networks and poolType column to ip_leases
// to distinguish IPv4 and IPv6 leases.
func migrateV13(ctx context.Context, tx *sql.Tx) error {
	stmt := fmt.Sprintf("ALTER TABLE %s ADD COLUMN ipv6Pool TEXT NOT NULL DEFAULT ''", DataNetworksTableName)
	if _, err := tx.ExecContext(ctx, stmt); err != nil {
		return fmt.Errorf("failed to add ipv6Pool column: %w", err)
	}

	stmt = fmt.Sprintf("ALTER TABLE %s ADD COLUMN poolType TEXT NOT NULL DEFAULT 'ipv4'", IPLeasesTableName)
	if _, err := tx.ExecContext(ctx, stmt); err != nil {
		return fmt.Errorf("failed to add poolType column: %w", err)
	}

	return nil
}
