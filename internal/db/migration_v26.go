// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"fmt"
)

func migrateV26(ctx context.Context, tx *sql.Tx) error {
	stmts := []string{
		fmt.Sprintf(`CREATE TABLE %s_new (
			id               TEXT    PRIMARY KEY,
			role             TEXT    NOT NULL CHECK (role IN ('%s', '%s')),
			diameterIdentity TEXT    NOT NULL,
			address          TEXT    NOT NULL,
			port             INTEGER NOT NULL
		)`, DiameterPeersTableName, DiameterPeerRoleSMSC, DiameterPeerRoleRoaming),
		fmt.Sprintf(`INSERT INTO %s_new (id, role, diameterIdentity, address, port)
			SELECT id, role, diameterIdentity, address, port FROM %s`, DiameterPeersTableName, DiameterPeersTableName),
		fmt.Sprintf("DROP TABLE %s", DiameterPeersTableName),
		fmt.Sprintf("ALTER TABLE %s_new RENAME TO %s", DiameterPeersTableName, DiameterPeersTableName),
		fmt.Sprintf("CREATE UNIQUE INDEX idx_diameter_peers_identity ON %s(diameterIdentity COLLATE NOCASE) WHERE diameterIdentity != ''", DiameterPeersTableName),
		fmt.Sprintf(`CREATE TABLE %s (
			id                 TEXT PRIMARY KEY,
			name               TEXT NOT NULL UNIQUE CHECK (name != ''),
			profileID          TEXT NOT NULL REFERENCES %s(id) ON DELETE RESTRICT,
			seppFQDN           TEXT CHECK (seppFQDN != '' AND seppFQDN = lower(seppFQDN)),
			seppN32cURI        TEXT CHECK (seppN32cURI != ''),
			seppN32fURI        TEXT CHECK (seppN32fURI != ''),
			seppTrustAnchorPEM TEXT CHECK (seppTrustAnchorPEM != ''),
			nrfAPIRoot         TEXT CHECK (nrfAPIRoot != ''),
			diameterPeerID     TEXT REFERENCES %s(id) ON DELETE RESTRICT,
			CHECK ((seppFQDN IS NULL) = (seppN32cURI IS NULL) AND (seppFQDN IS NULL) = (seppTrustAnchorPEM IS NULL)),
			CHECK (seppFQDN IS NOT NULL OR (seppN32fURI IS NULL AND nrfAPIRoot IS NULL))
		)`, RoamingPartnersTableName, ProfilesTableName, DiameterPeersTableName),
		fmt.Sprintf("CREATE UNIQUE INDEX idx_roaming_partners_sepp_fqdn ON %s(seppFQDN) WHERE seppFQDN IS NOT NULL", RoamingPartnersTableName),
		fmt.Sprintf(`CREATE TABLE %s (
			partnerID TEXT NOT NULL REFERENCES %s(id) ON DELETE CASCADE,
			mcc       TEXT NOT NULL CHECK (mcc GLOB '[0-9][0-9][0-9]'),
			mnc       TEXT NOT NULL CHECK (mnc GLOB '[0-9][0-9]' OR mnc GLOB '[0-9][0-9][0-9]'),
			PRIMARY KEY (mcc, mnc)
		)`, RoamingPartnerPLMNsTableName, RoamingPartnersTableName),
		fmt.Sprintf("CREATE INDEX idx_roaming_partner_plmns_partner ON %s(partnerID)", RoamingPartnerPLMNsTableName),
		fmt.Sprintf(`CREATE TABLE %s_new (
			id               TEXT PRIMARY KEY,
			imsi             TEXT NOT NULL UNIQUE CHECK (length(imsi) BETWEEN 6 AND 15 AND imsi GLOB '[0-9]*'),
			sequenceNumber   TEXT NOT NULL,
			permanentKey     TEXT NOT NULL,
			opc              TEXT NOT NULL,
			profileID        TEXT NOT NULL REFERENCES %s(id) ON DELETE RESTRICT,
			description      TEXT NOT NULL DEFAULT '',
			msisdn           TEXT NOT NULL DEFAULT '',
			roamingPartnerID TEXT REFERENCES %s(id) ON DELETE RESTRICT,
			CHECK ((roamingPartnerID IS NULL AND length(sequenceNumber) = 12 AND length(permanentKey) = 32 AND length(opc) = 32)
				OR (roamingPartnerID IS NOT NULL AND sequenceNumber = '' AND permanentKey = '' AND opc = '' AND msisdn = ''))
		)`, SubscribersTableName, ProfilesTableName, RoamingPartnersTableName),
		fmt.Sprintf(`INSERT INTO %s_new (id, imsi, sequenceNumber, permanentKey, opc, profileID, description, msisdn)
			SELECT id, imsi, sequenceNumber, permanentKey, opc, profileID, description, msisdn FROM %s`, SubscribersTableName, SubscribersTableName),
		fmt.Sprintf("DROP TABLE %s", SubscribersTableName),
		fmt.Sprintf("ALTER TABLE %s_new RENAME TO %s", SubscribersTableName, SubscribersTableName),
		fmt.Sprintf("CREATE UNIQUE INDEX idx_subscribers_msisdn ON %s(msisdn) WHERE msisdn != ''", SubscribersTableName),
		fmt.Sprintf("CREATE INDEX idx_subscribers_roaming_partner ON %s(roamingPartnerID) WHERE roamingPartnerID IS NOT NULL", SubscribersTableName),
		fmt.Sprintf(`CREATE TABLE %s (
			epochDay      INTEGER NOT NULL,
			imsi          TEXT    NOT NULL CHECK (length(imsi) BETWEEN 6 AND 15 AND imsi GLOB '[0-9]*'),
			mcc           TEXT    NOT NULL CHECK (mcc GLOB '[0-9][0-9][0-9]'),
			mnc           TEXT    NOT NULL CHECK (mnc GLOB '[0-9][0-9]' OR mnc GLOB '[0-9][0-9][0-9]'),
			bytesUplink   INTEGER NOT NULL DEFAULT 0 CHECK (bytesUplink >= 0),
			bytesDownlink INTEGER NOT NULL DEFAULT 0 CHECK (bytesDownlink >= 0),
			PRIMARY KEY (epochDay, imsi),
			CHECK (substr(imsi, 1, length(mcc) + length(mnc)) = mcc || mnc)
		)`, RoamingDailyUsageTableName),
		fmt.Sprintf("CREATE INDEX idx_roaming_daily_usage_plmn ON %s(mcc, mnc)", RoamingDailyUsageTableName),
		fmt.Sprintf(`CREATE TABLE %s (
			singleton    BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (singleton),
			fqdn         TEXT    NOT NULL CHECK (fqdn != '' AND fqdn = lower(fqdn)),
			certChainPEM TEXT    NOT NULL CHECK (certChainPEM != ''),
			keyPEM       TEXT    NOT NULL CHECK (keyPEM != '')
		)`, SEPPIdentityTableName),
	}

	for _, stmt := range stmts {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("add roaming schema: %w", err)
		}
	}

	return nil
}
