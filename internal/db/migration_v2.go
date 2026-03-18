// Copyright 2026 Ella Networks

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// ---------------------------------------------------------------------------
// V2 migration — NAS security columns, home network keys table, SPN columns.
// ---------------------------------------------------------------------------

func migrateV2(ctx context.Context, tx *sql.Tx) error {
	// 1. Add NAS security algorithm configuration columns to operator.
	_, err := tx.ExecContext(ctx,
		fmt.Sprintf("ALTER TABLE %s ADD COLUMN ciphering TEXT NOT NULL DEFAULT '[\"NEA2\",\"NEA1\",\"NEA0\"]'", OperatorTableName))
	if err != nil {
		return fmt.Errorf("failed to add ciphering column: %w", err)
	}

	_, err = tx.ExecContext(ctx,
		fmt.Sprintf("ALTER TABLE %s ADD COLUMN integrity TEXT NOT NULL DEFAULT '[\"NIA2\",\"NIA1\",\"NIA0\"]'", OperatorTableName))
	if err != nil {
		return fmt.Errorf("failed to add integrity column: %w", err)
	}

	// 2. Create the home_network_keys table.
	_, err = tx.ExecContext(ctx, fmt.Sprintf(`
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

	// 3. Migrate existing Profile A key from operator table (if one exists).
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

	// 4. Drop the now-unused column from the operator table.
	//    Requires SQLite 3.35.0+ (bundled go-sqlite3 v1.14.34 ships 3.47+).
	_, err = tx.ExecContext(ctx,
		fmt.Sprintf("ALTER TABLE %s DROP COLUMN homeNetworkPrivateKey", OperatorTableName))
	if err != nil {
		return fmt.Errorf("failed to drop homeNetworkPrivateKey column: %w", err)
	}

	// 5. Add SPN (Service Provider Name) columns to operator.
	_, err = tx.ExecContext(ctx,
		fmt.Sprintf("ALTER TABLE %s ADD COLUMN spnFullName TEXT NOT NULL DEFAULT 'Ella Networks'", OperatorTableName))
	if err != nil {
		return fmt.Errorf("failed to add spnFullName column: %w", err)
	}

	_, err = tx.ExecContext(ctx,
		fmt.Sprintf("ALTER TABLE %s ADD COLUMN spnShortName TEXT NOT NULL DEFAULT 'Ella'", OperatorTableName))
	if err != nil {
		return fmt.Errorf("failed to add spnShortName column: %w", err)
	}

	return nil
}
