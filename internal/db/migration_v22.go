// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"fmt"
)

func migrateV22(ctx context.Context, tx *sql.Tx) error {
	stmts := []string{
		fmt.Sprintf("ALTER TABLE %s ADD COLUMN msisdn TEXT NOT NULL DEFAULT ''", SubscribersTableName),
		fmt.Sprintf("CREATE UNIQUE INDEX idx_subscribers_msisdn ON %s(msisdn) WHERE msisdn != ''", SubscribersTableName),
		fmt.Sprintf(`CREATE TABLE %s (
			singleton   BOOLEAN PRIMARY KEY DEFAULT TRUE,
			smsNumber   TEXT    NOT NULL DEFAULT '',
			CHECK (singleton)
		)`, SMSSettingsTableName),
		fmt.Sprintf(`CREATE TABLE %s (
			id               TEXT    PRIMARY KEY,
			role             TEXT    NOT NULL CHECK (role IN ('smsc')),
			diameterIdentity TEXT    NOT NULL,
			address          TEXT    NOT NULL,
			port             INTEGER NOT NULL
		)`, DiameterPeersTableName),
		fmt.Sprintf("CREATE UNIQUE INDEX idx_diameter_peers_identity ON %s(diameterIdentity COLLATE NOCASE) WHERE diameterIdentity != ''", DiameterPeersTableName),
		fmt.Sprintf(`CREATE TABLE %s (
			serviceCentre TEXT PRIMARY KEY,
			peerId        TEXT NOT NULL REFERENCES %s(id) ON DELETE CASCADE
		)`, SMSCServiceCentresTableName, DiameterPeersTableName),
		fmt.Sprintf("CREATE INDEX idx_sms_smsc_service_centres_peer ON %s(peerId)", SMSCServiceCentresTableName),
		fmt.Sprintf(`CREATE TABLE %s (
			imsi      TEXT    PRIMARY KEY REFERENCES subscribers(imsi) ON DELETE CASCADE,
			mcef      INTEGER NOT NULL DEFAULT 0,
			updatedAt INTEGER NOT NULL
		)`, SMSWaitingTableName),
		fmt.Sprintf(`CREATE TABLE %s (
			imsi          TEXT NOT NULL REFERENCES %s(imsi) ON DELETE CASCADE,
			serviceCentre TEXT NOT NULL,
			PRIMARY KEY (imsi, serviceCentre)
		)`, SMSWaitingCentresTableName, SMSWaitingTableName),
		fmt.Sprintf("CREATE INDEX idx_sms_message_waiting_updated ON %s(updatedAt)", SMSWaitingTableName),
	}

	for _, stmt := range stmts {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("add SMS schema: %w", err)
		}
	}

	return nil
}
