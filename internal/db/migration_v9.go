// Copyright 2026 Ella Networks

package db

import (
	"context"
	"database/sql"
	"fmt"
)

// V9 introduces the HA-related schema: node-identity columns, the
// cluster_members table, and the cluster PKI tables (roots,
// intermediates, issued certs, revoked certs, join tokens, pki state).

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

const v9CreateClusterPKIRoots = `
	CREATE TABLE IF NOT EXISTS %s (
		fingerprint TEXT PRIMARY KEY,
		certPEM     TEXT NOT NULL,
		keyPEM      BLOB,
		addedAt     INTEGER NOT NULL,
		status      TEXT NOT NULL DEFAULT 'active'
			CHECK (status IN ('active','verify-only','retired')),
		CHECK (
			(status = 'active'      AND keyPEM IS NOT NULL) OR
			(status IN ('verify-only','retired') AND keyPEM IS NULL)
		)
)`

const v9CreateClusterPKIIntermediates = `
	CREATE TABLE IF NOT EXISTS %s (
		fingerprint     TEXT PRIMARY KEY,
		certPEM         TEXT NOT NULL,
		keyPEM          BLOB,
		rootFingerprint TEXT NOT NULL,
		notAfter        INTEGER NOT NULL,
		status          TEXT NOT NULL DEFAULT 'active'
			CHECK (status IN ('active','verify-only','retired')),
		CHECK (
			(status = 'active'      AND keyPEM IS NOT NULL) OR
			(status IN ('verify-only','retired') AND keyPEM IS NULL)
		)
)`

const v9CreateClusterIssuedCerts = `
	CREATE TABLE IF NOT EXISTS %s (
		serial                  INTEGER PRIMARY KEY,
		nodeID                  INTEGER NOT NULL,
		notAfter                INTEGER NOT NULL,
		intermediateFingerprint TEXT NOT NULL,
		issuedAt                INTEGER NOT NULL
)`

const v9CreateClusterRevokedCerts = `
	CREATE TABLE IF NOT EXISTS %s (
		serial      INTEGER PRIMARY KEY,
		nodeID      INTEGER NOT NULL,
		revokedAt   INTEGER NOT NULL,
		reason      TEXT NOT NULL DEFAULT '',
		purgeAfter  INTEGER NOT NULL
)`

// #nosec G101 -- DDL statement, not a credential
const v9CreateClusterJoinTokens = `
	CREATE TABLE IF NOT EXISTS %s (
		id           TEXT PRIMARY KEY,
		nodeID       INTEGER NOT NULL,
		claimsJSON   TEXT NOT NULL,
		expiresAt    INTEGER NOT NULL,
		consumedAt   INTEGER NOT NULL DEFAULT 0,
		consumedBy   INTEGER NOT NULL DEFAULT 0
)`

const v9CreateClusterPKIState = `
	CREATE TABLE IF NOT EXISTS %s (
		id             INTEGER PRIMARY KEY CHECK (id = 1),
		hmacKey        BLOB    NOT NULL,
		serialCounter  INTEGER NOT NULL DEFAULT 0
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
		fmt.Sprintf(v9CreateClusterPKIRoots, ClusterPKIRootsTableName),
		fmt.Sprintf(v9CreateClusterPKIIntermediates, ClusterPKIIntermediatesTableName),
		fmt.Sprintf(v9CreateClusterIssuedCerts, ClusterIssuedCertsTableName),
		fmt.Sprintf(v9CreateClusterRevokedCerts, ClusterRevokedCertsTableName),
		fmt.Sprintf(v9CreateClusterJoinTokens, ClusterJoinTokensTableName),
		fmt.Sprintf(v9CreateClusterPKIState, ClusterPKIStateTableName),
		fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_issued_certs_nodeID ON %s(nodeID)", ClusterIssuedCertsTableName),
		fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_issued_certs_notAfter ON %s(notAfter)", ClusterIssuedCertsTableName),
		fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_revoked_certs_purgeAfter ON %s(purgeAfter)", ClusterRevokedCertsTableName),
		fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_join_tokens_expiresAt ON %s(expiresAt)", ClusterJoinTokensTableName),
	}

	for _, stmt := range stmts {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("failed to execute %q: %w", stmt, err)
		}
	}

	return nil
}
