// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"fmt"
)

func migrateV26(ctx context.Context, tx *sql.Tx) error {
	for _, idx := range []struct{ name, table, column string }{
		{"idx_daily_usage_imsi", DailyUsageTableName, "imsi"},
		{"idx_ue_registrations_version", UERegistrationsTableName, "version"},
	} {
		stmt := fmt.Sprintf("CREATE INDEX %s ON %s (%s)", idx.name, idx.table, idx.column)

		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("create index on %s: %w", idx.table, err)
		}
	}

	return migrateSMSCNodes(ctx, tx)
}

func migrateSMSCNodes(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, fmt.Sprintf("SELECT id, diameterIdentity, address FROM %s WHERE role = 'smsc' ORDER BY id", DiameterPeersTableName))
	if err != nil {
		return fmt.Errorf("list SMSC peers: %w", err)
	}

	type peer struct{ id, identity, address string }

	var peers []peer

	for rows.Next() {
		var p peer
		if err := rows.Scan(&p.id, &p.identity, &p.address); err != nil {
			_ = rows.Close()
			return fmt.Errorf("scan SMSC peer: %w", err)
		}

		peers = append(peers, p)
	}

	if err := rows.Close(); err != nil {
		return fmt.Errorf("list SMSC peers: %w", err)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("list SMSC peers: %w", err)
	}

	stmts := []string{
		fmt.Sprintf(`CREATE TABLE %s (
			id   TEXT PRIMARY KEY,
			name TEXT NOT NULL
		)`, SMSCsTableName),
		fmt.Sprintf("CREATE UNIQUE INDEX idx_sms_smscs_name ON %s(name COLLATE NOCASE)", SMSCsTableName),
	}

	for _, stmt := range stmts {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("add SMSCs: %w", err)
		}
	}

	names := make([]string, 0, len(peers))

	for _, p := range peers {
		name := legacySMSCName(p.identity, p.address, names)
		names = append(names, name)

		if _, err := tx.ExecContext(ctx, fmt.Sprintf("INSERT INTO %s (id, name) VALUES (?, ?)", SMSCsTableName), p.id, name); err != nil {
			return fmt.Errorf("add SMSC for peer %s: %w", p.id, err)
		}
	}

	stmts = []string{
		fmt.Sprintf("ALTER TABLE %s ADD COLUMN smscId TEXT REFERENCES %s(id) ON DELETE CASCADE", DiameterPeersTableName, SMSCsTableName),
		fmt.Sprintf("UPDATE %s SET smscId = id WHERE role = 'smsc'", DiameterPeersTableName),
		fmt.Sprintf("CREATE INDEX idx_diameter_peers_smsc ON %s(smscId)", DiameterPeersTableName),
		fmt.Sprintf(`CREATE TABLE %s_new (
			serviceCentre TEXT PRIMARY KEY,
			smscId        TEXT NOT NULL REFERENCES %s(id) ON DELETE CASCADE
		)`, SMSCServiceCentresTableName, SMSCsTableName),
		fmt.Sprintf("INSERT INTO %s_new (serviceCentre, smscId) SELECT serviceCentre, peerId FROM %s", SMSCServiceCentresTableName, SMSCServiceCentresTableName),
		fmt.Sprintf("DROP TABLE %s", SMSCServiceCentresTableName),
		fmt.Sprintf("ALTER TABLE %s_new RENAME TO %s", SMSCServiceCentresTableName, SMSCServiceCentresTableName),
		fmt.Sprintf("CREATE INDEX idx_sms_smsc_service_centres_smsc ON %s(smscId)", SMSCServiceCentresTableName),
	}

	for _, stmt := range stmts {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("move SMSC peers to SMSC nodes: %w", err)
		}
	}

	return nil
}
