// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"fmt"
)

// migrateV16 creates the subscriber_framed_routes table: operator-provisioned
// IP prefixes routed toward a subscriber's session on a given data network
// (TS 23.501 §5.6.14). prefix is a normalized CIDR and globally unique across
// all subscribers and data networks (single routing domain; the SMF resolves a
// framed prefix to one session by destination alone).
func migrateV16(ctx context.Context, tx *sql.Tx) error {
	stmt := fmt.Sprintf(`CREATE TABLE %s (
		id TEXT PRIMARY KEY,
		imsi TEXT NOT NULL,
		dataNetworkID TEXT NOT NULL,
		prefix TEXT NOT NULL UNIQUE,
		FOREIGN KEY (imsi) REFERENCES subscribers(imsi) ON DELETE CASCADE,
		FOREIGN KEY (dataNetworkID) REFERENCES data_networks(id) ON DELETE RESTRICT
	)`, FramedRoutesTableName)

	if _, err := tx.ExecContext(ctx, stmt); err != nil {
		return fmt.Errorf("failed to create subscriber_framed_routes table: %w", err)
	}

	// The SMF resolves a session's framed routes by (imsi, dataNetworkID) on
	// every establishment; the data network alone drives the API list and the
	// bidirectional UE-pool overlap check.
	if _, err := tx.ExecContext(ctx, fmt.Sprintf(
		"CREATE INDEX idx_framed_routes_imsi_dn ON %s(imsi, dataNetworkID)", FramedRoutesTableName)); err != nil {
		return fmt.Errorf("create subscriber_framed_routes imsi/data-network index: %w", err)
	}

	if _, err := tx.ExecContext(ctx, fmt.Sprintf(
		"CREATE INDEX idx_framed_routes_dn ON %s(dataNetworkID)", FramedRoutesTableName)); err != nil {
		return fmt.Errorf("create subscriber_framed_routes data-network index: %w", err)
	}

	return nil
}
