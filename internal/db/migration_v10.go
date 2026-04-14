// Copyright 2026 Ella Networks

package db

import (
	"context"
	"database/sql"
	"fmt"
)

// V10 migration drops the legacy cluster_members.protocolVersion column.
func migrateV10(ctx context.Context, tx *sql.Tx) error {
	stmts := []string{
		fmt.Sprintf(`CREATE TABLE %s_new (
			nodeID        INTEGER PRIMARY KEY,
			raftAddress   TEXT NOT NULL,
			apiAddress    TEXT NOT NULL,
			binaryVersion TEXT NOT NULL DEFAULT '',
			suffrage      TEXT NOT NULL DEFAULT 'voter'
		)`, ClusterMembersTableName),
		fmt.Sprintf(`INSERT INTO %s_new (nodeID, raftAddress, apiAddress, binaryVersion, suffrage)
			SELECT nodeID, raftAddress, apiAddress, binaryVersion, suffrage FROM %s`, ClusterMembersTableName, ClusterMembersTableName),
		fmt.Sprintf("DROP TABLE %s", ClusterMembersTableName),
		fmt.Sprintf("ALTER TABLE %s_new RENAME TO %s", ClusterMembersTableName, ClusterMembersTableName),
	}

	for _, stmt := range stmts {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("failed to execute %q: %w", stmt, err)
		}
	}

	return nil
}
