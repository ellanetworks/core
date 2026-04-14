// Copyright 2026 Ella Networks

package db

import (
	"context"
	"database/sql"
	"fmt"
)

// Shared V1 split-baseline DDL — FROZEN. Emits the end state of
// legacyMigrations v1..v8 for the shared tables. Once shipped, must not be
// modified; further changes go in sharedMigrations v2+.

const sharedV1CreateOperator = `
	CREATE TABLE IF NOT EXISTS %s (
		id            INTEGER PRIMARY KEY CHECK (id = 1),
		mcc           TEXT NOT NULL,
		mnc           TEXT NOT NULL,
		operatorCode  TEXT NOT NULL,
		supportedTACs TEXT DEFAULT '[]',
		ciphering     TEXT NOT NULL DEFAULT '["NEA2","NEA1"]',
		integrity     TEXT NOT NULL DEFAULT '["NIA2","NIA1"]',
		spnFullName   TEXT NOT NULL DEFAULT 'Ella Networks',
		spnShortName  TEXT NOT NULL DEFAULT 'Ella',
		amfRegionID   INTEGER NOT NULL DEFAULT 1,
		amfSetID      INTEGER NOT NULL DEFAULT 1,
		clusterID     TEXT NOT NULL DEFAULT ''
)`

const sharedV1CreateNetworkSlices = `
	CREATE TABLE IF NOT EXISTS %s (
		id   INTEGER PRIMARY KEY,
		sst  INTEGER NOT NULL,
		sd   TEXT,
		name TEXT NOT NULL UNIQUE,
		UNIQUE(sst, sd)
)`

const sharedV1CreateProfiles = `
	CREATE TABLE IF NOT EXISTS %s (
		id             INTEGER PRIMARY KEY,
		name           TEXT NOT NULL UNIQUE,
		ueAmbrUplink   TEXT NOT NULL,
		ueAmbrDownlink TEXT NOT NULL
)`

const sharedV1CreateDataNetworks = `
	CREATE TABLE IF NOT EXISTS %s (
		id     INTEGER PRIMARY KEY,
		name   TEXT NOT NULL UNIQUE,
		ipPool TEXT NOT NULL,
		dns    TEXT NOT NULL,
		mtu    INTEGER NOT NULL
)`

const sharedV1CreatePolicies = `
	CREATE TABLE IF NOT EXISTS %s (
		id                  INTEGER PRIMARY KEY,
		name                TEXT    NOT NULL UNIQUE,
		profileID           INTEGER NOT NULL,
		sliceID             INTEGER NOT NULL,
		dataNetworkID       INTEGER NOT NULL,
		var5qi              INTEGER NOT NULL,
		arp                 INTEGER NOT NULL,
		sessionAmbrUplink   TEXT    NOT NULL,
		sessionAmbrDownlink TEXT    NOT NULL,
		FOREIGN KEY (profileID)     REFERENCES %s (id) ON DELETE RESTRICT,
		FOREIGN KEY (sliceID)       REFERENCES %s (id) ON DELETE RESTRICT,
		FOREIGN KEY (dataNetworkID) REFERENCES %s (id) ON DELETE RESTRICT,
		UNIQUE(profileID, sliceID, dataNetworkID)
)`

const sharedV1CreateNetworkRules = `
	CREATE TABLE IF NOT EXISTS %s (
		id            INTEGER PRIMARY KEY,
		policy_id     INTEGER NOT NULL,
		description   TEXT NOT NULL,
		direction     TEXT NOT NULL,
		remote_prefix TEXT,
		protocol      INTEGER DEFAULT 255,
		port_low      INTEGER DEFAULT 0,
		port_high     INTEGER DEFAULT 0,
		action        TEXT NOT NULL,
		precedence    INTEGER NOT NULL,
		created_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (policy_id) REFERENCES %s (id) ON DELETE CASCADE,
		UNIQUE(policy_id, precedence, direction)
)`

const sharedV1CreateSubscribers = `
	CREATE TABLE IF NOT EXISTS %s (
		id             INTEGER PRIMARY KEY,
		imsi           TEXT NOT NULL UNIQUE CHECK (length(imsi) BETWEEN 6 AND 15 AND imsi GLOB '[0-9]*'),
		sequenceNumber TEXT NOT NULL CHECK (length(sequenceNumber) = 12),
		permanentKey   TEXT NOT NULL CHECK (length(permanentKey) = 32),
		opc            TEXT NOT NULL CHECK (length(opc) = 32),
		profileID      INTEGER NOT NULL,
		FOREIGN KEY (profileID) REFERENCES %s (id) ON DELETE RESTRICT
)`

const sharedV1CreateDailyUsage = `
	CREATE TABLE IF NOT EXISTS %s (
		epoch_day      INTEGER NOT NULL,
		imsi           TEXT NOT NULL,
		bytes_uplink   INTEGER NOT NULL DEFAULT 0 CHECK (bytes_uplink   >= 0),
		bytes_downlink INTEGER NOT NULL DEFAULT 0 CHECK (bytes_downlink >= 0),
		PRIMARY KEY (epoch_day, imsi),
		FOREIGN KEY (imsi) REFERENCES %s(imsi) ON DELETE CASCADE
)`

const sharedV1CreateIPLeases = `
	CREATE TABLE IF NOT EXISTS %s (
		id          INTEGER PRIMARY KEY,
		poolID      INTEGER NOT NULL REFERENCES %s(id) ON DELETE CASCADE,
		addressBin  BLOB    NOT NULL,
		imsi        TEXT    NOT NULL REFERENCES %s(imsi) ON DELETE CASCADE,
		sessionID   INTEGER,
		type        TEXT    NOT NULL DEFAULT 'dynamic',
		createdAt   INTEGER NOT NULL,
		nodeID      INTEGER NOT NULL DEFAULT 0,
		UNIQUE(poolID, addressBin)
)`

const sharedV1CreateIPLeasesIndexes = `
	CREATE INDEX IF NOT EXISTS idx_leases_pool ON ip_leases(poolID);
	CREATE INDEX IF NOT EXISTS idx_leases_imsi ON ip_leases(imsi);
	CREATE INDEX IF NOT EXISTS idx_leases_session ON ip_leases(sessionID);
	CREATE INDEX IF NOT EXISTS idx_leases_pool_address_bin ON ip_leases(poolID, addressBin);
	CREATE INDEX IF NOT EXISTS idx_leases_node ON ip_leases(nodeID);
`

const sharedV1CreateHomeNetworkKeys = `
	CREATE TABLE IF NOT EXISTS %s (
		id              INTEGER PRIMARY KEY,
		key_identifier  INTEGER NOT NULL CHECK (key_identifier >= 0 AND key_identifier <= 255),
		scheme          TEXT    NOT NULL CHECK (scheme IN ('A', 'B')),
		private_key     TEXT    NOT NULL,
		UNIQUE(key_identifier, scheme)
)`

const sharedV1CreateUsers = `
	CREATE TABLE IF NOT EXISTS %s (
		id             INTEGER PRIMARY KEY,
		email          TEXT NOT NULL UNIQUE,
		roleID         INTEGER NOT NULL,
		hashedPassword TEXT NOT NULL
)`

const sharedV1CreateSessions = `
	CREATE TABLE IF NOT EXISTS %s (
		id          INTEGER PRIMARY KEY,
		user_id     INTEGER NOT NULL,
		token_hash  BLOB    NOT NULL UNIQUE,
		created_at  INTEGER NOT NULL DEFAULT (strftime('%%s','now')),
		expires_at  INTEGER NOT NULL,
		FOREIGN KEY (user_id) REFERENCES %s(id) ON DELETE CASCADE
)`

const sharedV1CreateAPITokens = `
	CREATE TABLE IF NOT EXISTS %s (
		id          INTEGER PRIMARY KEY,
		token_id    TEXT NOT NULL UNIQUE,
		name        TEXT NOT NULL,
		token_hash  TEXT NOT NULL,
		user_id     INTEGER NOT NULL,
		expires_at  DATETIME,
		FOREIGN KEY (user_id) REFERENCES %s(id) ON DELETE CASCADE,
		UNIQUE (name, user_id)
)` // #nosec: G101

const sharedV1CreateJWTSecret = `
	CREATE TABLE IF NOT EXISTS %s (
		singleton BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (singleton = TRUE),
		secret    BLOB    NOT NULL
)` // #nosec: G101

const sharedV1CreateBGPSettings = `
	CREATE TABLE IF NOT EXISTS %s (
		singleton     BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (singleton = TRUE),
		enabled       BOOLEAN NOT NULL DEFAULT FALSE,
		localAS       INTEGER NOT NULL DEFAULT 64512,
		routerID      TEXT    NOT NULL DEFAULT '',
		listenAddress TEXT    NOT NULL DEFAULT ':179'
)`

const sharedV1CreateBGPPeers = `
	CREATE TABLE IF NOT EXISTS %s (
		id          INTEGER PRIMARY KEY,
		address     TEXT    NOT NULL UNIQUE,
		remoteAS    INTEGER NOT NULL,
		holdTime    INTEGER NOT NULL DEFAULT 90,
		password    TEXT    NOT NULL DEFAULT '',
		description TEXT    NOT NULL DEFAULT '',
		nodeID      INTEGER
)`

const sharedV1CreateBGPImportPrefixes = `
	CREATE TABLE IF NOT EXISTS %s (
		id        INTEGER PRIMARY KEY,
		peerID    INTEGER NOT NULL REFERENCES %s(id) ON DELETE CASCADE,
		prefix    TEXT    NOT NULL,
		maxLength INTEGER NOT NULL
)`

const sharedV1CreateRoutes = `
	CREATE TABLE IF NOT EXISTS %s (
		id          INTEGER PRIMARY KEY,
		destination TEXT NOT NULL,
		gateway     TEXT NOT NULL,
		interface   TEXT NOT NULL,
		metric      INTEGER NOT NULL
)`

const sharedV1CreateNATSettings = `
	CREATE TABLE IF NOT EXISTS %s (
		singleton BOOLEAN PRIMARY KEY DEFAULT TRUE,
		enabled   BOOLEAN NOT NULL DEFAULT TRUE,
		CHECK (singleton)
)`

const sharedV1CreateN3Settings = `
	CREATE TABLE IF NOT EXISTS %s (
		singleton        BOOLEAN PRIMARY KEY DEFAULT TRUE,
		external_address TEXT NOT NULL DEFAULT TRUE,
		CHECK (singleton)
)`

const sharedV1CreateFlowAccountingSettings = `
	CREATE TABLE IF NOT EXISTS %s (
		singleton BOOLEAN PRIMARY KEY DEFAULT TRUE,
		enabled   BOOLEAN NOT NULL DEFAULT TRUE,
		CHECK (singleton)
)`

const sharedV1CreateRetentionPolicies = `
	CREATE TABLE IF NOT EXISTS %s (
		id             INTEGER PRIMARY KEY,
		category       TEXT NOT NULL UNIQUE,
		retention_days INTEGER NOT NULL CHECK (retention_days >= 1)
)`

const sharedV1CreateAuditLogs = `
	CREATE TABLE IF NOT EXISTS %s (
		id        INTEGER PRIMARY KEY,
		timestamp TEXT NOT NULL,
		level     TEXT NOT NULL,
		actor     TEXT NOT NULL DEFAULT '',
		action    TEXT NOT NULL,
		ip        TEXT NOT NULL DEFAULT '',
		details   TEXT NOT NULL DEFAULT ''
)`

const sharedV1CreateClusterMembers = `
	CREATE TABLE IF NOT EXISTS %s (
		nodeID      INTEGER PRIMARY KEY,
		raftAddress TEXT NOT NULL,
		apiAddress  TEXT NOT NULL
)`

func migrateSharedV1(ctx context.Context, tx *sql.Tx) error {
	// Order matters: parents before children for FK references.
	stmts := []string{
		fmt.Sprintf(sharedV1CreateOperator, OperatorTableName),
		fmt.Sprintf(sharedV1CreateNetworkSlices, NetworkSlicesTableName),
		fmt.Sprintf(sharedV1CreateProfiles, ProfilesTableName),
		fmt.Sprintf(sharedV1CreateDataNetworks, DataNetworksTableName),
		fmt.Sprintf(sharedV1CreatePolicies,
			PoliciesTableName,
			ProfilesTableName,
			NetworkSlicesTableName,
			DataNetworksTableName),
		fmt.Sprintf(sharedV1CreateNetworkRules, NetworkRulesTableName, PoliciesTableName),
		fmt.Sprintf(sharedV1CreateSubscribers, SubscribersTableName, ProfilesTableName),
		fmt.Sprintf(sharedV1CreateDailyUsage, DailyUsageTableName, SubscribersTableName),
		fmt.Sprintf(sharedV1CreateIPLeases,
			IPLeasesTableName,
			DataNetworksTableName,
			SubscribersTableName),
		fmt.Sprintf(sharedV1CreateHomeNetworkKeys, HomeNetworkKeysTableName),
		fmt.Sprintf(sharedV1CreateUsers, UsersTableName),
		fmt.Sprintf(sharedV1CreateSessions, SessionsTableName, UsersTableName),
		fmt.Sprintf(sharedV1CreateAPITokens, APITokensTableName, UsersTableName),
		fmt.Sprintf(sharedV1CreateJWTSecret, JWTSecretTableName),
		fmt.Sprintf(sharedV1CreateBGPSettings, BGPSettingsTableName),
		fmt.Sprintf(sharedV1CreateBGPPeers, BGPPeersTableName),
		fmt.Sprintf(sharedV1CreateBGPImportPrefixes, BGPImportPrefixesTableName, BGPPeersTableName),
		fmt.Sprintf(sharedV1CreateRoutes, RoutesTableName),
		fmt.Sprintf(sharedV1CreateNATSettings, NATSettingsTableName),
		fmt.Sprintf(sharedV1CreateN3Settings, N3SettingsTableName),
		fmt.Sprintf(sharedV1CreateFlowAccountingSettings, FlowAccountingSettingsTableName),
		fmt.Sprintf(sharedV1CreateRetentionPolicies, RetentionPolicyTableName),
		fmt.Sprintf(sharedV1CreateAuditLogs, AuditLogsTableName),
		fmt.Sprintf(sharedV1CreateClusterMembers, ClusterMembersTableName),
	}

	for _, stmt := range stmts {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("failed to execute shared DDL: %w\nStatement: %s", err, stmt)
		}
	}

	// Multi-statement index creation.
	if _, err := tx.ExecContext(ctx, sharedV1CreateIPLeasesIndexes); err != nil {
		return fmt.Errorf("failed to create ip_leases indexes: %w", err)
	}

	return nil
}
