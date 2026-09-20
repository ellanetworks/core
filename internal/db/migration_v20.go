// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"fmt"
)

// V20 retypes the raft node identity from INTEGER to TEXT everywhere it
// is stored, gives cluster_members its own amfPointer column, and drops
// cluster_join_tokens.nodeID now that a join token names no node. An
// existing member keeps its identity as the decimal text it already had,
// and its AMF Pointer is that same number.
func migrateV20(ctx context.Context, tx *sql.Tx) error {
	if err := migrateV20ClusterMembers(ctx, tx); err != nil {
		return err
	}

	if err := migrateV20IPLeases(ctx, tx); err != nil {
		return err
	}

	if err := migrateV20ClusterNodeCerts(ctx, tx); err != nil {
		return err
	}

	return migrateV20ClusterJoinTokens(ctx, tx)
}

func migrateV20ClusterNodeCerts(ctx context.Context, tx *sql.Tx) error {
	stmts := []string{
		fmt.Sprintf(`CREATE TABLE %s_new (
			nodeID      TEXT PRIMARY KEY,
			fingerprint TEXT    NOT NULL UNIQUE,
			certPEM     TEXT    NOT NULL,
			addedAt     INTEGER NOT NULL
		)`, ClusterNodeCertsTableName),
		fmt.Sprintf(`INSERT INTO %s_new (nodeID, fingerprint, certPEM, addedAt)
			SELECT CAST(nodeID AS TEXT), fingerprint, certPEM, addedAt FROM %s`,
			ClusterNodeCertsTableName, ClusterNodeCertsTableName),
		fmt.Sprintf("DROP TABLE %s", ClusterNodeCertsTableName),
		fmt.Sprintf("ALTER TABLE %s_new RENAME TO %s", ClusterNodeCertsTableName, ClusterNodeCertsTableName),
	}

	for _, stmt := range stmts {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("rebuild %s: %w", ClusterNodeCertsTableName, err)
		}
	}

	return nil
}

func migrateV20ClusterJoinTokens(ctx context.Context, tx *sql.Tx) error {
	stmts := []string{
		fmt.Sprintf(`CREATE TABLE %s_new (
			id           TEXT PRIMARY KEY,
			claimsJSON   TEXT NOT NULL,
			expiresAt    INTEGER NOT NULL,
			consumedAt   INTEGER NOT NULL DEFAULT 0,
			consumedBy   TEXT NOT NULL DEFAULT ''
		)`, ClusterJoinTokensTableName),
		fmt.Sprintf(`INSERT INTO %s_new (id, claimsJSON, expiresAt, consumedAt, consumedBy)
			SELECT id, claimsJSON, expiresAt, consumedAt,
				CASE WHEN consumedAt = 0 THEN '' ELSE CAST(consumedBy AS TEXT) END FROM %s`,
			ClusterJoinTokensTableName, ClusterJoinTokensTableName),
		fmt.Sprintf("DROP TABLE %s", ClusterJoinTokensTableName),
		fmt.Sprintf("ALTER TABLE %s_new RENAME TO %s", ClusterJoinTokensTableName, ClusterJoinTokensTableName),
	}

	for _, stmt := range stmts {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("rebuild %s: %w", ClusterJoinTokensTableName, err)
		}
	}

	return nil
}

func migrateV20ClusterMembers(ctx context.Context, tx *sql.Tx) error {
	stmts := []string{
		fmt.Sprintf(`CREATE TABLE %s_new (
			nodeID            TEXT PRIMARY KEY,
			raftAddress       TEXT NOT NULL,
			apiAddress        TEXT NOT NULL,
			binaryVersion     TEXT NOT NULL DEFAULT '',
			suffrage          TEXT NOT NULL DEFAULT 'voter',
			drainState        TEXT NOT NULL DEFAULT 'active'
				CHECK (drainState IN ('active','draining','drained')),
			drainUpdatedAt    INTEGER NOT NULL DEFAULT 0,
			amfPointer        INTEGER NOT NULL DEFAULT 0,
			displayName       TEXT NOT NULL DEFAULT ''
		)`, ClusterMembersTableName),
		fmt.Sprintf(`INSERT INTO %s_new (nodeID, raftAddress, apiAddress, binaryVersion, suffrage, amfPointer, drainState, drainUpdatedAt)
			SELECT CAST(nodeID AS TEXT), raftAddress, apiAddress, binaryVersion, suffrage, nodeID, drainState, drainUpdatedAt FROM %s`,
			ClusterMembersTableName, ClusterMembersTableName),
		fmt.Sprintf("DROP TABLE %s", ClusterMembersTableName),
		fmt.Sprintf("ALTER TABLE %s_new RENAME TO %s", ClusterMembersTableName, ClusterMembersTableName),
	}

	for _, stmt := range stmts {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("rebuild %s: %w", ClusterMembersTableName, err)
		}
	}

	return nil
}

func migrateV20IPLeases(ctx context.Context, tx *sql.Tx) error {
	stmts := []string{
		fmt.Sprintf(`CREATE TABLE %s_new (
			id          TEXT PRIMARY KEY,
			poolID      TEXT NOT NULL REFERENCES data_networks(id) ON DELETE CASCADE,
			addressBin  BLOB    NOT NULL,
			imsi        TEXT    NOT NULL REFERENCES subscribers(imsi) ON DELETE CASCADE,
			sessionID   INTEGER,
			type        TEXT    NOT NULL DEFAULT 'dynamic',
			createdAt   INTEGER NOT NULL,
			nodeID      TEXT NOT NULL DEFAULT '',
			poolType    TEXT NOT NULL DEFAULT 'ipv4',
			UNIQUE(poolID, addressBin)
		)`, IPLeasesTableName),
		fmt.Sprintf(`INSERT INTO %s_new (id, poolID, poolType, addressBin, imsi, sessionID, type, createdAt, nodeID)
			SELECT id, poolID, poolType, addressBin, imsi, sessionID, type, createdAt, CAST(nodeID AS TEXT) FROM %s`,
			IPLeasesTableName, IPLeasesTableName),
		fmt.Sprintf("DROP TABLE %s", IPLeasesTableName),
		fmt.Sprintf("ALTER TABLE %s_new RENAME TO %s", IPLeasesTableName, IPLeasesTableName),
		fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_leases_pool ON %s(poolID)", IPLeasesTableName),
		fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_leases_imsi ON %s(imsi)", IPLeasesTableName),
		fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_leases_session ON %s(sessionID)", IPLeasesTableName),
		fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_leases_node ON %s(nodeID)", IPLeasesTableName),
	}

	for _, stmt := range stmts {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("rebuild %s: %w", IPLeasesTableName, err)
		}
	}

	return nil
}
