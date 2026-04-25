// Copyright 2026 Ella Networks

package db

import (
	"context"
	"database/sql"
	"fmt"
)

// V10 introduces the fleet singleton table that tracks registration
// state with a Fleet control plane (URL, mTLS material, and last sync
// progress). The row is replicated via Raft so every node can establish
// mTLS with Fleet; only the leader applies config pushed back from Fleet.

const v10CreateFleetTable = `
	CREATE TABLE IF NOT EXISTS %s (
		singleton       BOOLEAN PRIMARY KEY DEFAULT TRUE,
		url             TEXT NOT NULL DEFAULT '',
		private_key     BLOB NOT NULL DEFAULT X'',
		certificate     BLOB NOT NULL DEFAULT X'',
		ca_certificate  BLOB NOT NULL DEFAULT X'',
		last_sync_at    TEXT NOT NULL DEFAULT '',
		config_revision INTEGER NOT NULL DEFAULT 0,
		CHECK (singleton)
)`

func migrateV10(ctx context.Context, tx *sql.Tx) error {
	stmt := fmt.Sprintf(v10CreateFleetTable, FleetTableName)
	if _, err := tx.ExecContext(ctx, stmt); err != nil {
		return fmt.Errorf("failed to execute %q: %w", stmt, err)
	}

	return nil
}
