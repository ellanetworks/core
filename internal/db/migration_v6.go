// Copyright 2026 Ella Networks

package db

import (
	"context"
	"database/sql"
	"fmt"
	"net/netip"
)

// ---------------------------------------------------------------------------
// V6 migration — replace address TEXT with addressBin BLOB in ip_leases
//
// Steps:
//  1. Add addressBin BLOB column to existing table.
//  2. Backfill from the address TEXT column (tolerates empty/invalid values).
//  3. Rebuild table without the address TEXT column.
//  4. Recreate indexes.
// ---------------------------------------------------------------------------

func migrateV6(ctx context.Context, tx *sql.Tx) error {
	// 1. Add the column with a 16-byte zero default to satisfy NOT NULL.
	_, err := tx.ExecContext(ctx, fmt.Sprintf(
		`ALTER TABLE %s ADD COLUMN addressBin BLOB NOT NULL DEFAULT x'00000000000000000000000000000000'`,
		IPLeasesTableName))
	if err != nil {
		return fmt.Errorf("failed to add addressBin column: %w", err)
	}

	// 2. Backfill existing rows with the correct binary representation.
	rows, err := tx.QueryContext(ctx, fmt.Sprintf("SELECT id, address FROM %s", IPLeasesTableName))
	if err != nil {
		return fmt.Errorf("failed to query leases for backfill: %w", err)
	}

	defer func() { _ = rows.Close() }()

	type leaseRow struct {
		id      int
		address string
	}

	var pending []leaseRow

	for rows.Next() {
		var r leaseRow

		if err := rows.Scan(&r.id, &r.address); err != nil {
			return fmt.Errorf("failed to scan lease row: %w", err)
		}

		pending = append(pending, r)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("rows iteration error: %w", err)
	}

	for _, r := range pending {
		var b [16]byte

		if r.address != "" {
			addr, parseErr := netip.ParseAddr(r.address)
			if parseErr != nil {
				// Tolerate invalid addresses: leave as zeroed bytes.
				continue
			}

			b = addr.As16()
		}

		_, err = tx.ExecContext(ctx,
			fmt.Sprintf("UPDATE %s SET addressBin = ? WHERE id = ?", IPLeasesTableName),
			b[:], r.id)
		if err != nil {
			return fmt.Errorf("failed to backfill addressBin for lease %d: %w", r.id, err)
		}
	}

	// 3. Rebuild table without the address TEXT column.
	_, err = tx.ExecContext(ctx, fmt.Sprintf(`
		CREATE TABLE %s_new (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			poolID      INTEGER NOT NULL REFERENCES %s(id) ON DELETE CASCADE,
			addressBin  BLOB    NOT NULL,
			imsi        TEXT    NOT NULL REFERENCES %s(imsi) ON DELETE CASCADE,
			sessionID   INTEGER,
			type        TEXT    NOT NULL DEFAULT 'dynamic',
			createdAt   INTEGER NOT NULL,
			UNIQUE(poolID, addressBin)
		)`, IPLeasesTableName, DataNetworksTableName, SubscribersTableName))
	if err != nil {
		return fmt.Errorf("failed to create new ip_leases table: %w", err)
	}

	_, err = tx.ExecContext(ctx, fmt.Sprintf(
		`INSERT INTO %s_new (id, poolID, addressBin, imsi, sessionID, type, createdAt)
		 SELECT id, poolID, addressBin, imsi, sessionID, type, createdAt FROM %s`,
		IPLeasesTableName, IPLeasesTableName))
	if err != nil {
		return fmt.Errorf("failed to copy data to new ip_leases table: %w", err)
	}

	_, err = tx.ExecContext(ctx, fmt.Sprintf(`DROP TABLE %s`, IPLeasesTableName))
	if err != nil {
		return fmt.Errorf("failed to drop old ip_leases table: %w", err)
	}

	_, err = tx.ExecContext(ctx, fmt.Sprintf(
		`ALTER TABLE %s_new RENAME TO %s`, IPLeasesTableName, IPLeasesTableName))
	if err != nil {
		return fmt.Errorf("failed to rename new ip_leases table: %w", err)
	}

	// 4. Recreate indexes.
	indexes := []struct {
		name string
		cols string
	}{
		{"idx_leases_pool", "poolID"},
		{"idx_leases_imsi", "imsi"},
		{"idx_leases_session", "sessionID"},
		{"idx_leases_pool_address_bin", "poolID, addressBin"},
	}

	for _, idx := range indexes {
		_, err = tx.ExecContext(ctx, fmt.Sprintf(
			`CREATE INDEX IF NOT EXISTS %s ON %s(%s)`, idx.name, IPLeasesTableName, idx.cols))
		if err != nil {
			return fmt.Errorf("failed to create index %s: %w", idx.name, err)
		}
	}

	return nil
}
