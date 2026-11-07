// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"fmt"
)

func migrateV26(ctx context.Context, tx *sql.Tx) error {
	if err := addSMSCs(ctx, tx); err != nil {
		return err
	}

	stmts := []string{
		fmt.Sprintf(`CREATE TABLE %s_new (
			id               TEXT    PRIMARY KEY,
			role             TEXT    NOT NULL CHECK (role IN ('%s', '%s')),
			diameterIdentity TEXT    NOT NULL,
			address          TEXT    NOT NULL,
			port             INTEGER NOT NULL,
			smscId           TEXT    REFERENCES %s(id) ON DELETE CASCADE,
			CHECK ((role = '%s') = (smscId IS NOT NULL))
		)`, DiameterPeersTableName, DiameterPeerRoleSMSC, DiameterPeerRoleRoaming, SMSCsTableName, DiameterPeerRoleSMSC),
		fmt.Sprintf(`INSERT INTO %s_new (id, role, diameterIdentity, address, port, smscId)
			SELECT id, role, diameterIdentity, address, port, CASE WHEN role = '%s' THEN id END FROM %s`, DiameterPeersTableName, DiameterPeerRoleSMSC, DiameterPeersTableName),
		fmt.Sprintf("DROP TABLE %s", DiameterPeersTableName),
		fmt.Sprintf("ALTER TABLE %s_new RENAME TO %s", DiameterPeersTableName, DiameterPeersTableName),
		fmt.Sprintf("CREATE UNIQUE INDEX idx_diameter_peers_identity ON %s(diameterIdentity COLLATE NOCASE) WHERE diameterIdentity != ''", DiameterPeersTableName),
		fmt.Sprintf("CREATE INDEX idx_diameter_peers_smsc ON %s(smscId)", DiameterPeersTableName),
		fmt.Sprintf(`CREATE TABLE %s_new (
			serviceCentre TEXT PRIMARY KEY,
			smscId        TEXT NOT NULL REFERENCES %s(id) ON DELETE CASCADE
		)`, SMSCServiceCentresTableName, SMSCsTableName),
		fmt.Sprintf("INSERT INTO %s_new (serviceCentre, smscId) SELECT serviceCentre, peerId FROM %s", SMSCServiceCentresTableName, SMSCServiceCentresTableName),
		fmt.Sprintf("DROP TABLE %s", SMSCServiceCentresTableName),
		fmt.Sprintf("ALTER TABLE %s_new RENAME TO %s", SMSCServiceCentresTableName, SMSCServiceCentresTableName),
		fmt.Sprintf("CREATE INDEX idx_sms_smsc_service_centres_smsc ON %s(smscId)", SMSCServiceCentresTableName),
		fmt.Sprintf(`CREATE TABLE %s (
			id                 TEXT PRIMARY KEY,
			name               TEXT NOT NULL UNIQUE CHECK (name != ''),
			profileID          TEXT NOT NULL REFERENCES %s(id) ON DELETE RESTRICT,
			seppFQDN           TEXT CHECK (seppFQDN != '' AND seppFQDN = lower(seppFQDN)),
			seppN32cURI        TEXT CHECK (seppN32cURI != ''),
			seppN32fURI        TEXT CHECK (seppN32fURI != ''),
			seppTrustAnchorPEM TEXT CHECK (seppTrustAnchorPEM != ''),
			diameterPeerID     TEXT REFERENCES %s(id) ON DELETE RESTRICT,
			withdrawingUntil   INTEGER NOT NULL DEFAULT 0,
			CHECK ((seppFQDN IS NULL) = (seppN32cURI IS NULL) AND (seppFQDN IS NULL) = (seppTrustAnchorPEM IS NULL)),
			CHECK (seppFQDN IS NOT NULL OR seppN32fURI IS NULL)
		)`, RoamingPartnersTableName, ProfilesTableName, DiameterPeersTableName),
		fmt.Sprintf("CREATE UNIQUE INDEX idx_roaming_partners_sepp_fqdn ON %s(seppFQDN) WHERE seppFQDN IS NOT NULL", RoamingPartnersTableName),
		fmt.Sprintf(`CREATE TABLE %s (
			partnerID  TEXT NOT NULL REFERENCES %s(id) ON DELETE CASCADE,
			mcc        TEXT NOT NULL CHECK (mcc GLOB '[0-9][0-9][0-9]'),
			mnc        TEXT NOT NULL CHECK (mnc GLOB '[0-9][0-9]' OR mnc GLOB '[0-9][0-9][0-9]'),
			nrfAPIRoot TEXT CHECK (nrfAPIRoot != ''),
			PRIMARY KEY (mcc, mnc)
		)`, RoamingPartnerPLMNsTableName, RoamingPartnersTableName),
		fmt.Sprintf("CREATE INDEX idx_roaming_partner_plmns_partner ON %s(partnerID)", RoamingPartnerPLMNsTableName),
		fmt.Sprintf(`CREATE TABLE %s (
			imsi            TEXT    PRIMARY KEY CHECK (length(imsi) BETWEEN 6 AND 15 AND imsi NOT GLOB '*[^0-9]*'),
			partnerID       TEXT    NOT NULL REFERENCES %s(id) ON DELETE RESTRICT,
			msisdn          TEXT    NOT NULL DEFAULT '' CHECK (msisdn = '' OR (length(msisdn) BETWEEN 1 AND 15 AND msisdn NOT GLOB '*[^0-9]*')),
			since           INTEGER NOT NULL,
			deregisterUntil INTEGER NOT NULL DEFAULT 0
		)`, VisitorsTableName, RoamingPartnersTableName),
		fmt.Sprintf("CREATE INDEX idx_visitors_partner ON %s(partnerID)", VisitorsTableName),
		fmt.Sprintf(`CREATE TABLE %s_new (
			imsi    TEXT    NOT NULL CHECK (length(imsi) BETWEEN 6 AND 15 AND imsi NOT GLOB '*[^0-9]*'),
			type    TEXT    NOT NULL,
			nodeID  TEXT    NOT NULL,
			purged  INTEGER NOT NULL DEFAULT 0,
			version INTEGER NOT NULL,
			PRIMARY KEY (imsi, type)
		)`, UERegistrationsTableName),
		fmt.Sprintf(`INSERT INTO %s_new (imsi, type, nodeID, purged, version)
			SELECT imsi, type, nodeID, purged, version FROM %s`, UERegistrationsTableName, UERegistrationsTableName),
		fmt.Sprintf("DROP TABLE %s", UERegistrationsTableName),
		fmt.Sprintf("ALTER TABLE %s_new RENAME TO %s", UERegistrationsTableName, UERegistrationsTableName),
		fmt.Sprintf("CREATE INDEX idx_ue_registrations_node ON %s(nodeID)", UERegistrationsTableName),
		fmt.Sprintf("CREATE INDEX idx_ue_registrations_version ON %s(version)", UERegistrationsTableName),
		fmt.Sprintf(`CREATE TABLE %s_new (
			id         TEXT    PRIMARY KEY,
			poolID     TEXT    NOT NULL REFERENCES %s(id) ON DELETE CASCADE,
			addressBin BLOB    NOT NULL,
			imsi       TEXT    NOT NULL CHECK (length(imsi) BETWEEN 6 AND 15 AND imsi NOT GLOB '*[^0-9]*'),
			sessionID  INTEGER,
			type       TEXT    NOT NULL DEFAULT 'dynamic',
			createdAt  INTEGER NOT NULL,
			nodeID     TEXT    NOT NULL DEFAULT '',
			poolType   TEXT    NOT NULL DEFAULT 'ipv4',
			UNIQUE(poolID, addressBin)
		)`, IPLeasesTableName, DataNetworksTableName),
		fmt.Sprintf(`INSERT INTO %s_new (id, poolID, addressBin, imsi, sessionID, type, createdAt, nodeID, poolType)
			SELECT id, poolID, addressBin, imsi, sessionID, type, createdAt, nodeID, poolType FROM %s`, IPLeasesTableName, IPLeasesTableName),
		fmt.Sprintf("DROP TABLE %s", IPLeasesTableName),
		fmt.Sprintf("ALTER TABLE %s_new RENAME TO %s", IPLeasesTableName, IPLeasesTableName),
		fmt.Sprintf("CREATE INDEX idx_leases_pool ON %s(poolID)", IPLeasesTableName),
		fmt.Sprintf("CREATE INDEX idx_leases_imsi ON %s(imsi)", IPLeasesTableName),
		fmt.Sprintf("CREATE INDEX idx_leases_session ON %s(sessionID)", IPLeasesTableName),
		fmt.Sprintf("CREATE INDEX idx_leases_node ON %s(nodeID)", IPLeasesTableName),
		fmt.Sprintf(`CREATE TABLE %s_new (
			id               INTEGER PRIMARY KEY AUTOINCREMENT,
			subscriber_id    TEXT    NOT NULL,
			source_ip        TEXT    NOT NULL,
			destination_ip   TEXT    NOT NULL,
			source_port      INTEGER NOT NULL DEFAULT 0,
			destination_port INTEGER NOT NULL DEFAULT 0,
			protocol         INTEGER NOT NULL,
			packets          INTEGER NOT NULL,
			bytes            INTEGER NOT NULL,
			start_time       INTEGER NOT NULL,
			end_time         INTEGER NOT NULL,
			direction        TEXT    NOT NULL,
			action           INT     NOT NULL DEFAULT 0
		)`, FlowReportsTableName),
		fmt.Sprintf(`INSERT INTO %s_new (id, subscriber_id, source_ip, destination_ip, source_port, destination_port, protocol, packets, bytes, start_time, end_time, direction, action)
			SELECT id, subscriber_id, source_ip, destination_ip, source_port, destination_port, protocol, packets, bytes, start_time, end_time, direction, action FROM %s`, FlowReportsTableName, FlowReportsTableName),
		fmt.Sprintf("DROP TABLE %s", FlowReportsTableName),
		fmt.Sprintf("ALTER TABLE %s_new RENAME TO %s", FlowReportsTableName, FlowReportsTableName),
		fmt.Sprintf("CREATE INDEX idx_flow_reports_subscriber_id ON %s (subscriber_id)", FlowReportsTableName),
		fmt.Sprintf("CREATE INDEX idx_flow_reports_end_time ON %s (end_time, start_time)", FlowReportsTableName),
		fmt.Sprintf("CREATE INDEX idx_flow_reports_protocol ON %s (protocol)", FlowReportsTableName),
		fmt.Sprintf("CREATE INDEX idx_flow_reports_source_ip ON %s (source_ip)", FlowReportsTableName),
		fmt.Sprintf("CREATE INDEX idx_flow_reports_destination_ip ON %s (destination_ip)", FlowReportsTableName),
		fmt.Sprintf(`CREATE TABLE %s_new (
			epochDay      INTEGER NOT NULL,
			imsi          TEXT    NOT NULL CHECK (length(imsi) BETWEEN 6 AND 15 AND imsi NOT GLOB '*[^0-9]*'),
			mcc           TEXT    CHECK (mcc GLOB '[0-9][0-9][0-9]'),
			mnc           TEXT    CHECK (mnc GLOB '[0-9][0-9]' OR mnc GLOB '[0-9][0-9][0-9]'),
			bytesUplink   INTEGER NOT NULL DEFAULT 0 CHECK (bytesUplink >= 0),
			bytesDownlink INTEGER NOT NULL DEFAULT 0 CHECK (bytesDownlink >= 0),
			PRIMARY KEY (epochDay, imsi),
			CHECK ((mcc IS NULL) = (mnc IS NULL)),
			CHECK (mcc IS NULL OR substr(imsi, 1, length(mcc) + length(mnc)) = mcc || mnc)
		)`, DailyUsageTableName),
		fmt.Sprintf(`INSERT INTO %s_new (epochDay, imsi, bytesUplink, bytesDownlink)
			SELECT epoch_day, imsi, bytes_uplink, bytes_downlink FROM %s`, DailyUsageTableName, DailyUsageTableName),
		fmt.Sprintf("DROP TABLE %s", DailyUsageTableName),
		fmt.Sprintf("ALTER TABLE %s_new RENAME TO %s", DailyUsageTableName, DailyUsageTableName),
		fmt.Sprintf(`CREATE TABLE %s (
			singleton BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (singleton),
			certPEM   TEXT    NOT NULL CHECK (certPEM != ''),
			keyPEM    TEXT    NOT NULL CHECK (keyPEM != '')
		)`, SEPPCATableName),
		fmt.Sprintf(`CREATE TABLE %s (
			seppID       INTEGER PRIMARY KEY CHECK (seppID BETWEEN 1 AND 63),
			fqdn         TEXT    NOT NULL UNIQUE CHECK (fqdn != '' AND fqdn = lower(fqdn)),
			source       TEXT    NOT NULL CHECK (source IN ('%s', '%s')),
			certChainPEM TEXT    NOT NULL CHECK (certChainPEM != ''),
			keyPEM       TEXT    NOT NULL CHECK (keyPEM != '')
		)`, SEPPIdentitiesTableName, SEPPIdentityGenerated, SEPPIdentityImported),
	}

	for _, stmt := range stmts {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("add roaming schema: %w", err)
		}
	}

	return nil
}

func addSMSCs(ctx context.Context, tx *sql.Tx) error {
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

	return nil
}
