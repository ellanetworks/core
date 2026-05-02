// Copyright 2026 Ella Networks

package db

import (
	"context"
	"database/sql"
	"fmt"
)

// V10 drops two columns that became dead after the HA design moved
// away from per-row capability caching:
//
//   - bgp_peers.nodeID — bgp_peers is now a local-only table (not
//     replicated via Raft), so per-node scoping via a column is
//     unnecessary.
//   - cluster_members.maxSchemaVersion — the migration gate now reads
//     each voter's binary capability live via /cluster/status instead
//     of trusting a cached row, so the column has no consumer.
//
// Safe to run as a coordinated post-baseline migration: by the time the
// gate proposes v10, every voter is on a binary whose UpsertClusterMember
// SQL no longer references maxSchemaVersion, so no captured changeset
// in flight can reference the dropped column at apply time.
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
