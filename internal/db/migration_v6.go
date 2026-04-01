// Copyright 2026 Ella Networks

package db

import (
	"context"
	"database/sql"
	"fmt"
	"net/netip"
)

// ---------------------------------------------------------------------------
// V6 migration — add addressBin BLOB column to ip_leases for binary sort order
// ---------------------------------------------------------------------------

func migrateV6(ctx context.Context, tx *sql.Tx) error {
	// Add the column with a 16-byte zero default to satisfy NOT NULL.
	_, err := tx.ExecContext(ctx, fmt.Sprintf(
		`ALTER TABLE %s ADD COLUMN addressBin BLOB NOT NULL DEFAULT x'00000000000000000000000000000000'`,
		IPLeasesTableName))
	if err != nil {
		return fmt.Errorf("failed to add addressBin column: %w", err)
	}

	// Backfill existing rows with the correct binary representation.
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
		addr, err := netip.ParseAddr(r.address)
		if err != nil {
			return fmt.Errorf("invalid IP address %q in lease %d: %w", r.address, r.id, err)
		}

		b := addr.As16()

		_, err = tx.ExecContext(ctx,
			fmt.Sprintf("UPDATE %s SET addressBin = ? WHERE id = ?", IPLeasesTableName),
			b[:], r.id)
		if err != nil {
			return fmt.Errorf("failed to backfill addressBin for lease %d: %w", r.id, err)
		}
	}

	// Create composite index for the paged query.
	_, err = tx.ExecContext(ctx, fmt.Sprintf(
		`CREATE INDEX IF NOT EXISTS idx_leases_pool_address_bin ON %s(poolID, addressBin)`,
		IPLeasesTableName))
	if err != nil {
		return fmt.Errorf("failed to create idx_leases_pool_address_bin: %w", err)
	}

	return nil
}
