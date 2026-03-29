// Copyright 2026 Ella Networks

package db

import (
	"context"
	"database/sql"
	"fmt"
)

// ---------------------------------------------------------------------------
// V4 migration — add bgp_settings, bgp_peers, jwt_secret, and ip_leases
// tables; drop ipAddress column from subscribers.
// ---------------------------------------------------------------------------

func migrateV4(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			singleton     BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (singleton = TRUE),
			enabled       BOOLEAN NOT NULL DEFAULT FALSE,
			localAS       INTEGER NOT NULL DEFAULT 64512,
			routerID      TEXT    NOT NULL DEFAULT '',
			listenAddress TEXT    NOT NULL DEFAULT ':179'
		)`, BGPSettingsTableName))
	if err != nil {
		return fmt.Errorf("failed to create bgp_settings table: %w", err)
	}

	_, err = tx.ExecContext(ctx, fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			address     TEXT    NOT NULL UNIQUE,
			remoteAS    INTEGER NOT NULL,
			holdTime    INTEGER NOT NULL DEFAULT 90,
			password    TEXT    NOT NULL DEFAULT '',
			description TEXT    NOT NULL DEFAULT ''
		)`, BGPPeersTableName))
	if err != nil {
		return fmt.Errorf("failed to create bgp_peers table: %w", err)
	}

	_, err = tx.ExecContext(ctx, fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id        INTEGER PRIMARY KEY AUTOINCREMENT,
			peerID    INTEGER NOT NULL REFERENCES %s(id) ON DELETE CASCADE,
			prefix    TEXT    NOT NULL,
			maxLength INTEGER NOT NULL
		)`, BGPImportPrefixesTableName, BGPPeersTableName))
	if err != nil {
		return fmt.Errorf("failed to create bgp_import_prefixes table: %w", err)
	}

	_, err = tx.ExecContext(ctx, fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			singleton BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (singleton = TRUE),
			secret    BLOB    NOT NULL
		)`, JWTSecretTableName))
	if err != nil {
		return fmt.Errorf("failed to create jwt_secret table: %w", err)
	}

	// --- ip_leases table ---------------------------------------------------
	_, err = tx.ExecContext(ctx, fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			poolID      INTEGER NOT NULL REFERENCES %s(id) ON DELETE RESTRICT,
			address     TEXT    NOT NULL,
			imsi        TEXT    NOT NULL REFERENCES %s(imsi) ON DELETE RESTRICT,
			sessionID   INTEGER,
			type        TEXT    NOT NULL DEFAULT 'dynamic',
			createdAt   INTEGER NOT NULL,
			UNIQUE(poolID, address)
		)`, IPLeasesTableName, DataNetworksTableName, SubscribersTableName))
	if err != nil {
		return fmt.Errorf("failed to create ip_leases table: %w", err)
	}

	_, err = tx.ExecContext(ctx, fmt.Sprintf(
		`CREATE INDEX IF NOT EXISTS idx_leases_pool ON %s(poolID)`, IPLeasesTableName))
	if err != nil {
		return fmt.Errorf("failed to create idx_leases_pool: %w", err)
	}

	_, err = tx.ExecContext(ctx, fmt.Sprintf(
		`CREATE INDEX IF NOT EXISTS idx_leases_imsi ON %s(imsi)`, IPLeasesTableName))
	if err != nil {
		return fmt.Errorf("failed to create idx_leases_imsi: %w", err)
	}

	_, err = tx.ExecContext(ctx, fmt.Sprintf(
		`CREATE INDEX IF NOT EXISTS idx_leases_session ON %s(sessionID)`, IPLeasesTableName))
	if err != nil {
		return fmt.Errorf("failed to create idx_leases_session: %w", err)
	}

	// --- Drop ipAddress column from subscribers ----------------------------
	// SQLite's ALTER TABLE DROP COLUMN cannot drop UNIQUE columns. Use the
	// table-rebuild approach: create new table, copy data, drop old, rename.
	_, err = tx.ExecContext(ctx, fmt.Sprintf(`
		CREATE TABLE %s_new (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			imsi TEXT NOT NULL UNIQUE CHECK (length(imsi) BETWEEN 6 AND 15 AND imsi GLOB '[0-9]*'),
			sequenceNumber TEXT NOT NULL CHECK (length(sequenceNumber) = 12),
			permanentKey TEXT NOT NULL CHECK (length(permanentKey) = 32),
			opc TEXT NOT NULL CHECK (length(opc) = 32),
			policyID INTEGER NOT NULL,
			FOREIGN KEY (policyID) REFERENCES %s (id) ON DELETE CASCADE
		)`, SubscribersTableName, PoliciesTableName))
	if err != nil {
		return fmt.Errorf("failed to create subscribers_new table: %w", err)
	}

	_, err = tx.ExecContext(ctx, fmt.Sprintf(`
		INSERT INTO %s_new (id, imsi, sequenceNumber, permanentKey, opc, policyID)
		SELECT id, imsi, sequenceNumber, permanentKey, opc, policyID
		FROM %s`, SubscribersTableName, SubscribersTableName))
	if err != nil {
		return fmt.Errorf("failed to copy subscribers data: %w", err)
	}

	_, err = tx.ExecContext(ctx, fmt.Sprintf(`DROP TABLE %s`, SubscribersTableName))
	if err != nil {
		return fmt.Errorf("failed to drop old subscribers table: %w", err)
	}

	_, err = tx.ExecContext(ctx, fmt.Sprintf(
		`ALTER TABLE %s_new RENAME TO %s`, SubscribersTableName, SubscribersTableName))
	if err != nil {
		return fmt.Errorf("failed to rename subscribers_new to subscribers: %w", err)
	}

	return nil
}
