// Copyright 2026 Ella Networks

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// ---------------------------------------------------------------------------
// V3 migration — creates home_network_keys table and migrates existing key.
// ---------------------------------------------------------------------------

func migrateV3(ctx context.Context, tx *sql.Tx) error {
	// 1. Create the home_network_keys table.
	_, err := tx.ExecContext(ctx, fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id              INTEGER PRIMARY KEY AUTOINCREMENT,
			key_identifier  INTEGER NOT NULL CHECK (key_identifier >= 0 AND key_identifier <= 255),
			scheme          TEXT    NOT NULL CHECK (scheme IN ('A', 'B')),
			private_key     TEXT    NOT NULL,
			UNIQUE(key_identifier, scheme)
		)`, HomeNetworkKeysTableName))
	if err != nil {
		return fmt.Errorf("failed to create home_network_keys table: %w", err)
	}

	// 2. Migrate existing Profile A key from operator table (if one exists).
	var privateKey string

	err = tx.QueryRowContext(ctx,
		fmt.Sprintf("SELECT homeNetworkPrivateKey FROM %s WHERE id=1", OperatorTableName),
	).Scan(&privateKey)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("failed to read existing home network private key: %w", err)
	}

	if err == nil && privateKey != "" {
		_, err = tx.ExecContext(ctx,
			fmt.Sprintf("INSERT INTO %s (key_identifier, scheme, private_key) VALUES (0, 'A', ?)", HomeNetworkKeysTableName),
			privateKey)
		if err != nil {
			return fmt.Errorf("failed to migrate existing key to home_network_keys: %w", err)
		}
	}

	// 3. Drop the now-unused column from the operator table.
	//    Requires SQLite 3.35.0+ (bundled go-sqlite3 v1.14.34 ships 3.47+).
	_, err = tx.ExecContext(ctx,
		fmt.Sprintf("ALTER TABLE %s DROP COLUMN homeNetworkPrivateKey", OperatorTableName))
	if err != nil {
		return fmt.Errorf("failed to drop homeNetworkPrivateKey column: %w", err)
	}

	return nil
}
