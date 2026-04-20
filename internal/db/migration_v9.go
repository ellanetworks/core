// Copyright 2026 Ella Networks

package db

import (
	"context"
	"database/sql"
	"fmt"
)

// ---------------------------------------------------------------------------
// V9 migration — HA schema additions
//
// Adds the columns and tables introduced alongside Raft-based HA:
//   * operator.amfRegionID, amfSetID, clusterID
//   * ip_leases.nodeID + idx_leases_node
//   * bgp_peers.nodeID
//   * cluster_members table
// ---------------------------------------------------------------------------

const v9CreateClusterMembers = `
	CREATE TABLE IF NOT EXISTS %s (
		nodeID            INTEGER PRIMARY KEY,
		raftAddress       TEXT NOT NULL,
		apiAddress        TEXT NOT NULL,
		binaryVersion     TEXT NOT NULL DEFAULT '',
		suffrage          TEXT NOT NULL DEFAULT 'voter',
		maxSchemaVersion  INTEGER NOT NULL DEFAULT 0,
		drainState        TEXT NOT NULL DEFAULT 'active'
			CHECK (drainState IN ('active','draining','drained')),
		drainUpdatedAt    INTEGER NOT NULL DEFAULT 0
)`

func migrateV9(ctx context.Context, tx *sql.Tx) error {
	stmts := []string{
		fmt.Sprintf("ALTER TABLE %s ADD COLUMN amfRegionID INTEGER NOT NULL DEFAULT 1", OperatorTableName),
		fmt.Sprintf("ALTER TABLE %s ADD COLUMN amfSetID INTEGER NOT NULL DEFAULT 1", OperatorTableName),
		fmt.Sprintf("ALTER TABLE %s ADD COLUMN clusterID TEXT NOT NULL DEFAULT ''", OperatorTableName),
		fmt.Sprintf("ALTER TABLE %s ADD COLUMN nodeID INTEGER NOT NULL DEFAULT 0", IPLeasesTableName),
		fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_leases_node ON %s(nodeID)", IPLeasesTableName),
		fmt.Sprintf("ALTER TABLE %s ADD COLUMN nodeID INTEGER", BGPPeersTableName),
		fmt.Sprintf(v9CreateClusterMembers, ClusterMembersTableName),
	}

	for _, stmt := range stmts {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("failed to execute %q: %w", stmt, err)
		}
	}

	return nil
}
