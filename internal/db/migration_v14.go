// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"fmt"
)

// V14 adds 4G/LTE configuration: the MME identity (GUMMEI = MME Group ID + MME
// Code, TS 23.003 §2.8.1) on the operator; per-profile allowed access (4G/5G,
// TS 23.501 §5.3.4 Core Network type restriction); and a default data-network
// binding on policies (the default APN/DNN, TS 23.401 §3.1 / TS 23.501 §5.15).
//
// Defaults reproduce current behaviour exactly: MME group/code = 1 (matching the
// previously hard-coded values), access unrestricted, and each profile's
// earliest policy marked default (matching the prior policies[0] selection).
func migrateV14(ctx context.Context, tx *sql.Tx) error {
	stmts := []string{
		fmt.Sprintf("ALTER TABLE %s ADD COLUMN allow4G INTEGER NOT NULL DEFAULT 1", ProfilesTableName),
		fmt.Sprintf("ALTER TABLE %s ADD COLUMN allow5G INTEGER NOT NULL DEFAULT 1", ProfilesTableName),
		fmt.Sprintf("ALTER TABLE %s ADD COLUMN isDefault INTEGER NOT NULL DEFAULT 0", PoliciesTableName),
	}

	for _, stmt := range stmts {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("v14: %q: %w", stmt, err)
		}
	}

	// Mark each profile's earliest policy (lowest UUIDv7 id = first created, the
	// prior policies[0]) as its default data-network binding.
	backfill := fmt.Sprintf(
		"UPDATE %s SET isDefault = 1 WHERE id IN (SELECT MIN(id) FROM %s GROUP BY profileID)",
		PoliciesTableName, PoliciesTableName,
	)
	if _, err := tx.ExecContext(ctx, backfill); err != nil {
		return fmt.Errorf("v14: backfill default policy: %w", err)
	}

	// Rewrite the NAS security algorithm lists from the 5G-flavoured names to the
	// RAT-neutral algorithm identities shared by EPS and 5G (TS 24.301 §9.9.3.23 ≡
	// TS 24.501 §9.11.3.34): NEA0/NIA0→NULL, NEA1/NIA1→SNOW3G, NEA2/NIA2→AES.
	rewrite := fmt.Sprintf(
		"UPDATE %s SET "+
			"ciphering = REPLACE(REPLACE(REPLACE(ciphering, 'NEA0', 'NULL'), 'NEA1', 'SNOW3G'), 'NEA2', 'AES'), "+
			"integrity = REPLACE(REPLACE(REPLACE(integrity, 'NIA0', 'NULL'), 'NIA1', 'SNOW3G'), 'NIA2', 'AES')",
		OperatorTableName,
	)
	if _, err := tx.ExecContext(ctx, rewrite); err != nil {
		return fmt.Errorf("v14: rewrite NAS algorithm names: %w", err)
	}

	return nil
}
