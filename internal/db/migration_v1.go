// Copyright 2026 Ella Networks

package db

import (
	"context"
	"database/sql"
	"fmt"
)

// ---------------------------------------------------------------------------
// V1 baseline DDL — FROZEN. Do not modify these constants.
// They are the historical record of the schema at V1.
// All future schema changes must go in V2+ migrations.
// ---------------------------------------------------------------------------

const v1CreateOperatorTable = `
	CREATE TABLE IF NOT EXISTS %s (
 		id INTEGER PRIMARY KEY CHECK (id = 1),

		mcc TEXT NOT NULL,
		mnc TEXT NOT NULL,
		operatorCode TEXT NOT NULL,
		supportedTACs TEXT DEFAULT '[]',
		sst INTEGER NOT NULL,
		sd BLOB NULLABLE,  -- 3 bytes
		homeNetworkPrivateKey TEXT NOT NULL
)`

const v1CreateRoutesTable = `
	CREATE TABLE IF NOT EXISTS %s (
 		id INTEGER PRIMARY KEY AUTOINCREMENT,
		destination TEXT NOT NULL,
		gateway TEXT NOT NULL,
		interface TEXT NOT NULL,
		metric INTEGER NOT NULL
)`

const v1CreateRetentionPolicyTable = `
	CREATE TABLE IF NOT EXISTS %s (
		id              INTEGER PRIMARY KEY AUTOINCREMENT,
		category        TEXT NOT NULL UNIQUE,
		retention_days  INTEGER NOT NULL CHECK (retention_days >= 1)
);`

const v1CreateNATSettingsTable = `
CREATE TABLE IF NOT EXISTS %s (
  singleton BOOLEAN PRIMARY KEY DEFAULT TRUE,
  enabled   BOOLEAN NOT NULL DEFAULT TRUE,
  CHECK (singleton)
);
`

const v1CreateFlowAccountingSettingsTable = `
CREATE TABLE IF NOT EXISTS %s (
  singleton BOOLEAN PRIMARY KEY DEFAULT TRUE,
  enabled   BOOLEAN NOT NULL DEFAULT TRUE,
  CHECK (singleton)
);
`

const v1CreateN3SettingsTable = `
CREATE TABLE IF NOT EXISTS %s (
  singleton BOOLEAN PRIMARY KEY DEFAULT TRUE,
  external_address   TEXT NOT NULL DEFAULT TRUE,
  CHECK (singleton)
);
`

const v1CreateAuditLogsTable = `
	CREATE TABLE IF NOT EXISTS %s (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp  TEXT NOT NULL,                      -- RFC3339
		level      TEXT NOT NULL,                      -- info|warn|error...
		actor      TEXT NOT NULL DEFAULT '',
		action     TEXT NOT NULL,
		ip         TEXT NOT NULL DEFAULT '',
		details    TEXT NOT NULL DEFAULT ''
);`

const v1CreateRadioEventsTable = `
	CREATE TABLE IF NOT EXISTS %s (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp  TEXT NOT NULL,                      -- RFC3339
		protocol      TEXT NOT NULL,
		message_type TEXT NOT NULL,
		direction	TEXT NOT NULL DEFAULT '',       -- inbound|outbound
		local_address TEXT NOT NULL DEFAULT '',
		remote_address TEXT NOT NULL DEFAULT '',
		raw			 BLOB NOT NULL,
		details    TEXT NOT NULL DEFAULT ''
);`

const v1CreateRadioEventsIndex = `
	CREATE INDEX IF NOT EXISTS idx_network_logs_protocol ON network_logs (protocol);
	CREATE INDEX IF NOT EXISTS idx_network_logs_timestamp ON network_logs (timestamp);
	CREATE INDEX IF NOT EXISTS idx_network_logs_message_type ON network_logs (message_type);
	CREATE INDEX IF NOT EXISTS idx_network_logs_direction ON network_logs (direction);
	CREATE INDEX IF NOT EXISTS idx_network_logs_local_address ON network_logs (local_address);
	CREATE INDEX IF NOT EXISTS idx_network_logs_remote_address ON network_logs (remote_address);
`

const v1CreateDataNetworksTable = `
	CREATE TABLE IF NOT EXISTS %s (
 		id INTEGER PRIMARY KEY AUTOINCREMENT,

		name TEXT NOT NULL UNIQUE,

		ipPool TEXT NOT NULL,
		dns TEXT NOT NULL,
		mtu INTEGER NOT NULL
)`

const v1CreatePoliciesTable = `
	CREATE TABLE IF NOT EXISTS %s (
 		id INTEGER PRIMARY KEY AUTOINCREMENT,

		name TEXT NOT NULL UNIQUE,

		bitrateUplink TEXT NOT NULL,
		bitrateDownlink TEXT NOT NULL,
		var5qi INTEGER NOT NULL,
		arp INTEGER NOT NULL,

		dataNetworkID INTEGER NOT NULL,

		FOREIGN KEY (dataNetworkID) REFERENCES data_networks (id) ON DELETE CASCADE
)`

const v1CreateSubscribersTable = `
	CREATE TABLE IF NOT EXISTS %s (
 		id INTEGER PRIMARY KEY AUTOINCREMENT,

		imsi TEXT NOT NULL UNIQUE CHECK (length(imsi) BETWEEN 6 AND 15 AND imsi GLOB '[0-9]*'),

		ipAddress TEXT UNIQUE,

		sequenceNumber TEXT NOT NULL CHECK (length(sequenceNumber) = 12),
		permanentKey TEXT NOT NULL CHECK (length(permanentKey) = 32),
		opc TEXT NOT NULL CHECK (length(opc) = 32),

		policyID INTEGER NOT NULL,

		FOREIGN KEY (policyID) REFERENCES policies (id) ON DELETE CASCADE
)`

const v1CreateUsersTable = `
	CREATE TABLE IF NOT EXISTS %s (
 		id INTEGER PRIMARY KEY AUTOINCREMENT,

		email TEXT NOT NULL UNIQUE,
		roleID INTEGER NOT NULL,
		hashedPassword TEXT NOT NULL
)`

const v1CreateSessionsTable = `
  CREATE TABLE IF NOT EXISTS sessions (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id     INTEGER NOT NULL,
  token_hash  BLOB    NOT NULL UNIQUE,
  created_at  INTEGER NOT NULL DEFAULT (strftime('%s','now')),
  expires_at  INTEGER NOT NULL,
  FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
)`

const v1CreateAPITokensTable = `
	CREATE TABLE IF NOT EXISTS %s (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
	token_id    TEXT NOT NULL UNIQUE,
  name        TEXT NOT NULL,
  token_hash  TEXT NOT NULL,
  user_id     INTEGER NOT NULL,
  expires_at  DATETIME,
  FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,

	UNIQUE (name, user_id)
);
` // #nosec: G101

const v1CreateDailyUsageTable = `
	CREATE TABLE IF NOT EXISTS %s (
		epoch_day INTEGER NOT NULL,

		imsi TEXT NOT NULL,
		bytes_uplink   INTEGER NOT NULL DEFAULT 0 CHECK (bytes_uplink   >= 0),
    bytes_downlink INTEGER NOT NULL DEFAULT 0 CHECK (bytes_downlink >= 0),

		PRIMARY KEY (epoch_day, imsi),

		FOREIGN KEY (imsi) REFERENCES subscribers(imsi) ON DELETE CASCADE
)`

const v1CreateFlowReportsTable = `
	CREATE TABLE IF NOT EXISTS %s (
		id              INTEGER PRIMARY KEY AUTOINCREMENT,
		subscriber_id   TEXT NOT NULL,              -- IMSI (looked up from PDR ID)
		source_ip       TEXT NOT NULL,              -- IP address as string
		destination_ip  TEXT NOT NULL,              -- IP address as string
		source_port     INTEGER NOT NULL DEFAULT 0, -- 0 if N/A
		destination_port INTEGER NOT NULL DEFAULT 0,-- 0 if N/A
		protocol        INTEGER NOT NULL,           -- IP protocol number
		packets         INTEGER NOT NULL,           -- Total packets
		bytes           INTEGER NOT NULL,           -- Total bytes
		start_time      TEXT NOT NULL,              -- RFC3339
		end_time        TEXT NOT NULL,              -- RFC3339
		direction       TEXT NOT NULL,              -- 'uplink' or 'downlink'

		FOREIGN KEY (subscriber_id) REFERENCES subscribers(imsi) ON DELETE CASCADE
	);`

const v1CreateFlowReportsIndex = `
	CREATE INDEX IF NOT EXISTS idx_flow_reports_subscriber_id ON flow_reports (subscriber_id);
	CREATE INDEX IF NOT EXISTS idx_flow_reports_end_time ON flow_reports (end_time);
	CREATE INDEX IF NOT EXISTS idx_flow_reports_protocol ON flow_reports (protocol);
	CREATE INDEX IF NOT EXISTS idx_flow_reports_source_ip ON flow_reports (source_ip);
	CREATE INDEX IF NOT EXISTS idx_flow_reports_destination_ip ON flow_reports (destination_ip);
`

// migrateV1 creates the baseline schema. Every CREATE TABLE / CREATE INDEX
// uses IF NOT EXISTS so this migration is safe for both:
//   - Fresh databases (tables don't exist yet)
//   - Existing databases being migrated for the first time (tables already exist,
//     statements are no-ops)
//
// This function and the constants above are FROZEN — never modify them.
// All future schema changes must go in V2+ migrations.
func migrateV1(ctx context.Context, tx *sql.Tx) error {
	// Ordered so that tables with foreign key dependencies are created after
	// the tables they reference.
	stmts := []string{
		// Independent tables (no FK deps)
		fmt.Sprintf(v1CreateOperatorTable, OperatorTableName),
		fmt.Sprintf(v1CreateRoutesTable, RoutesTableName),
		fmt.Sprintf(v1CreateRetentionPolicyTable, RetentionPolicyTableName),
		fmt.Sprintf(v1CreateNATSettingsTable, NATSettingsTableName),
		fmt.Sprintf(v1CreateFlowAccountingSettingsTable, FlowAccountingSettingsTableName),
		fmt.Sprintf(v1CreateN3SettingsTable, N3SettingsTableName),
		fmt.Sprintf(v1CreateAuditLogsTable, AuditLogsTableName),

		// Radio events
		fmt.Sprintf(v1CreateRadioEventsTable, RadioEventsTableName),

		// Data networks → policies → subscribers (FK chain)
		fmt.Sprintf(v1CreateDataNetworksTable, DataNetworksTableName),
		fmt.Sprintf(v1CreatePoliciesTable, PoliciesTableName),
		fmt.Sprintf(v1CreateSubscribersTable, SubscribersTableName),

		// Users → sessions, api_tokens (FK chain)
		fmt.Sprintf(v1CreateUsersTable, UsersTableName),
		v1CreateSessionsTable,
		fmt.Sprintf(v1CreateAPITokensTable, APITokensTableName),

		// Tables depending on subscribers
		fmt.Sprintf(v1CreateDailyUsageTable, DailyUsageTableName),
		fmt.Sprintf(v1CreateFlowReportsTable, FlowReportsTableName),
	}

	for _, stmt := range stmts {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("failed to execute DDL: %w\nStatement: %s", err, stmt)
		}
	}

	// Index creation statements are multi-statement strings (separated by ;).
	// tx.ExecContext with go-sqlite3 supports multi-statement execution.
	indexStmts := []string{
		v1CreateRadioEventsIndex,
		v1CreateFlowReportsIndex,
	}

	for _, stmt := range indexStmts {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("failed to create indexes: %w\nStatement: %s", err, stmt)
		}
	}

	return nil
}
