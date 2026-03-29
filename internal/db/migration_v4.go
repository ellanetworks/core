// Copyright 2026 Ella Networks

package db

import (
	"context"
	"database/sql"
	"fmt"
)

// ---------------------------------------------------------------------------
// V4 migration — add bgp_settings and bgp_peers tables.
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

	return nil
}
