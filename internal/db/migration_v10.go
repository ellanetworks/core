// Copyright 2026 Ella Networks

package db

import (
	"context"
	"database/sql"
	"fmt"
)

// V10 drops two columns whose data the current binary no longer reads
// or writes: bgp_peers.nodeID (table is local-only) and
// cluster_members.maxSchemaVersion (gate reads /cluster/status live).
func migrateV10(ctx context.Context, tx *sql.Tx) error {
	stmts := []string{
		fmt.Sprintf("ALTER TABLE %s DROP COLUMN nodeID", BGPPeersTableName),
		fmt.Sprintf("ALTER TABLE %s DROP COLUMN maxSchemaVersion", ClusterMembersTableName),
	}

	for _, stmt := range stmts {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("execute %q: %w", stmt, err)
		}
	}

	return nil
}
