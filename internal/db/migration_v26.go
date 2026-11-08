// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/ellanetworks/core/internal/models"
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

	if err := migrateSMSCNodes(ctx, tx); err != nil {
		return err
	}

	return migrateDataNetworkForwarding(ctx, tx)
}

func migrateDataNetworkForwarding(ctx context.Context, tx *sql.Tx) error {
	localSwitch, natEnabled, err := migrationForwarding(ctx, tx)
	if err != nil {
		return err
	}

	stmts := []string{
		fmt.Sprintf(`CREATE TABLE %s (
			sliceID         TEXT NOT NULL REFERENCES %s(id) ON DELETE RESTRICT,
			dataNetworkID   TEXT NOT NULL REFERENCES %s(id) ON DELETE RESTRICT,
			nat             BOOLEAN NOT NULL CHECK (nat IN (0, 1)),
			localSwitch     BOOLEAN NOT NULL CHECK (localSwitch IN (0, 1)),
			networkInstance TEXT NOT NULL DEFAULT '%s',
			PRIMARY KEY (sliceID, dataNetworkID)
		)`, SliceDataNetworksTableName, NetworkSlicesTableName, DataNetworksTableName, models.DefaultNetworkInstanceName),
		fmt.Sprintf("CREATE INDEX idx_slice_data_networks_data_network ON %s(dataNetworkID)", SliceDataNetworksTableName),
	}

	for _, stmt := range stmts {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("add slice data networks: %w", err)
		}
	}

	pairs := fmt.Sprintf("SELECT DISTINCT p.sliceID, p.dataNetworkID, d.ims FROM %s p JOIN %s d ON d.id = p.dataNetworkID", PoliciesTableName, DataNetworksTableName)

	if _, err := tx.ExecContext(ctx, fmt.Sprintf("INSERT INTO %s (sliceID, dataNetworkID, nat, localSwitch) SELECT sliceID, dataNetworkID, ? AND NOT ims, ims OR ? FROM (%s)", SliceDataNetworksTableName, pairs), natEnabled, localSwitch); err != nil {
		return fmt.Errorf("serve the data networks of existing policies: %w", err)
	}

	return nil
}

func migrationForwarding(ctx context.Context, tx *sql.Tx) (bool, bool, error) {
	toggles, _ := ctx.Value(migrationTogglesKey{}).(migrationToggles)

	localSwitch, natEnabled := false, false

	if toggles.localSwitch != nil {
		localSwitch = *toggles.localSwitch
	} else {
		v, err := readLegacyToggle(ctx, tx, LocalSwitchSettingsTableName, false)
		if err != nil {
			return false, false, err
		}

		localSwitch = v
	}

	if toggles.nat != nil {
		natEnabled = *toggles.nat
	} else {
		v, err := readLegacyToggle(ctx, tx, NATSettingsTableName, NATDefaultEnabled)
		if err != nil {
			return false, false, err
		}

		natEnabled = v
	}

	return localSwitch, natEnabled, nil
}

func readLegacyToggle(ctx context.Context, q interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}, table string, fallback bool,
) (bool, error) {
	var exists int
	if err := q.QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&exists); err != nil {
		return false, fmt.Errorf("look up %s: %w", table, err)
	}

	if exists == 0 {
		return fallback, nil
	}

	enabled := fallback

	err := q.QueryRowContext(ctx, fmt.Sprintf("SELECT enabled FROM %s WHERE singleton = TRUE", table)).Scan(&enabled)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return false, fmt.Errorf("read %s: %w", table, err)
	}

	return enabled, nil
}

type (
	migrationTogglesKey struct{}
	migrationToggles    struct{ localSwitch, nat *bool }
)

func withMigrationToggles(ctx context.Context, localSwitch, nat *bool) context.Context {
	return context.WithValue(ctx, migrationTogglesKey{}, migrationToggles{localSwitch: localSwitch, nat: nat})
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
