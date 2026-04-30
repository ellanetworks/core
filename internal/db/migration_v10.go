// Copyright 2026 Ella Networks

package db

import (
	"context"
	"database/sql"
	"fmt"
)

// V10 drops the bgp_peers.nodeID column. The bgp_peers table is now
// local-only (not replicated via Raft), so per-node scoping via a nodeID
// column is unnecessary.
func migrateV10(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, fmt.Sprintf("ALTER TABLE %s DROP COLUMN nodeID", BGPPeersTableName))
	return err
}
