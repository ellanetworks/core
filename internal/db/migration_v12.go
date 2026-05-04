// Copyright 2026 Ella Networks

package db

import (
	"context"
	"database/sql"
	"fmt"
)

// V12 replaces the chain-based cluster PKI (root + intermediate +
// leaf-issuance + revocation) with a flat fingerprint-pinning model
// (cluster_node_certs). The cluster TLS transport now trusts peers by
// SHA-256 of their self-signed cert. There is no CA. Removing a node
// from the cluster means deleting its row.
//
// Because the live cert material in the dropped tables would not chain
// against the new (absent) CA anyway, this migration is destructive on
// purpose: any existing PKI state from v9-v11 is discarded. Operators
// upgrading across this boundary must re-bootstrap by re-issuing
// join tokens for each cluster member.

const v12CreateClusterNodeCerts = `
	CREATE TABLE IF NOT EXISTS %s (
		nodeID      INTEGER PRIMARY KEY,
		fingerprint TEXT    NOT NULL UNIQUE,
		certPEM     TEXT    NOT NULL,
		addedAt     INTEGER NOT NULL
)`

// cluster_join_hmac is a one-row table (id is fixed at 1) that holds the
// 32-byte HMAC key used to authenticate join tokens. Lazily seeded by
// the leader before the first join token is minted.
const v12CreateClusterJoinHMAC = `
	CREATE TABLE IF NOT EXISTS %s (
		id      INTEGER PRIMARY KEY CHECK (id = 1),
		hmacKey BLOB    NOT NULL
)`

func migrateV12(ctx context.Context, tx *sql.Tx) error {
	stmts := []string{
		// Drop chain-PKI tables. cluster_join_tokens stays — still used by
		// the new register flow for replay protection.
		fmt.Sprintf("DROP TABLE IF EXISTS %s", ClusterPKIRootsTableName),
		fmt.Sprintf("DROP TABLE IF EXISTS %s", ClusterPKIIntermediatesTableName),
		fmt.Sprintf("DROP TABLE IF EXISTS %s", ClusterIssuedCertsTableName),
		fmt.Sprintf("DROP TABLE IF EXISTS %s", ClusterRevokedCertsTableName),
		fmt.Sprintf("DROP TABLE IF EXISTS %s", ClusterPKIStateTableName),

		fmt.Sprintf(v12CreateClusterNodeCerts, ClusterNodeCertsTableName),
		fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_node_certs_fingerprint ON %s(fingerprint)", ClusterNodeCertsTableName),

		fmt.Sprintf(v12CreateClusterJoinHMAC, ClusterJoinHMACTableName),
	}

	for _, stmt := range stmts {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("failed to execute %q: %w", stmt, err)
		}
	}

	return nil
}
