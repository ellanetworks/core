// Copyright 2026 Ella Networks

package db

import (
	"context"
	"database/sql"
	"fmt"
)

// ---------------------------------------------------------------------------
// Local V1 split-baseline DDL — FROZEN.
//
// This migration emits the END STATE of legacyMigrations v1..v8 restricted
// to the tables that live in local.db (see spec_ha.md §3.2.1). It is a new
// function — not an edited copy of any historical migration. Once shipped,
// it MUST NOT be modified; further schema changes go in localMigrations v2+.
//
// Tables created here:
//
//   network_logs (radio events) — includes the radio_name column added in
//   legacy v3.
//
//   flow_reports — includes the action column added in legacy v8. The
//   subscriber_id column is intentionally NOT a foreign key here: subscribers
//   live in shared.db so cross-database FK enforcement is impossible. The
//   column remains a plain TEXT IMSI.
// ---------------------------------------------------------------------------

const localV1CreateNetworkLogs = `
	CREATE TABLE IF NOT EXISTS %s (
		id             INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp      TEXT NOT NULL,                    -- RFC3339
		protocol       TEXT NOT NULL,
		message_type   TEXT NOT NULL,
		direction      TEXT NOT NULL DEFAULT '',         -- inbound|outbound
		local_address  TEXT NOT NULL DEFAULT '',
		remote_address TEXT NOT NULL DEFAULT '',
		raw            BLOB NOT NULL,
		details        TEXT NOT NULL DEFAULT '',
		radio_name     TEXT NOT NULL DEFAULT ''
)`

const localV1CreateNetworkLogsIndexes = `
	CREATE INDEX IF NOT EXISTS idx_network_logs_protocol       ON network_logs (protocol);
	CREATE INDEX IF NOT EXISTS idx_network_logs_timestamp      ON network_logs (timestamp);
	CREATE INDEX IF NOT EXISTS idx_network_logs_message_type   ON network_logs (message_type);
	CREATE INDEX IF NOT EXISTS idx_network_logs_direction      ON network_logs (direction);
	CREATE INDEX IF NOT EXISTS idx_network_logs_local_address  ON network_logs (local_address);
	CREATE INDEX IF NOT EXISTS idx_network_logs_remote_address ON network_logs (remote_address);
	CREATE INDEX IF NOT EXISTS idx_network_logs_radio_name     ON network_logs (radio_name);
`

// flow_reports lives in local.db, so subscriber_id is a plain TEXT column
// (no FK) — its referent (subscribers) is in shared.db.
const localV1CreateFlowReports = `
	CREATE TABLE IF NOT EXISTS %s (
		id               INTEGER PRIMARY KEY AUTOINCREMENT,
		subscriber_id    TEXT NOT NULL,               -- IMSI (no FK across DBs)
		source_ip        TEXT NOT NULL,
		destination_ip   TEXT NOT NULL,
		source_port      INTEGER NOT NULL DEFAULT 0,
		destination_port INTEGER NOT NULL DEFAULT 0,
		protocol         INTEGER NOT NULL,
		packets          INTEGER NOT NULL,
		bytes            INTEGER NOT NULL,
		start_time       TEXT NOT NULL,               -- RFC3339
		end_time         TEXT NOT NULL,               -- RFC3339
		direction        TEXT NOT NULL,               -- 'uplink' or 'downlink'
		action           INT  NOT NULL DEFAULT 0
)`

const localV1CreateFlowReportsIndexes = `
	CREATE INDEX IF NOT EXISTS idx_flow_reports_subscriber_id  ON flow_reports (subscriber_id);
	CREATE INDEX IF NOT EXISTS idx_flow_reports_end_time       ON flow_reports (end_time);
	CREATE INDEX IF NOT EXISTS idx_flow_reports_protocol       ON flow_reports (protocol);
	CREATE INDEX IF NOT EXISTS idx_flow_reports_source_ip      ON flow_reports (source_ip);
	CREATE INDEX IF NOT EXISTS idx_flow_reports_destination_ip ON flow_reports (destination_ip);
`

func migrateLocalV1(ctx context.Context, tx *sql.Tx) error {
	stmts := []string{
		fmt.Sprintf(localV1CreateNetworkLogs, RadioEventsTableName),
		fmt.Sprintf(localV1CreateFlowReports, FlowReportsTableName),
	}

	for _, stmt := range stmts {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("failed to execute local DDL: %w\nStatement: %s", err, stmt)
		}
	}

	indexStmts := []string{
		localV1CreateNetworkLogsIndexes,
		localV1CreateFlowReportsIndexes,
	}

	for _, stmt := range indexStmts {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("failed to create local indexes: %w", err)
		}
	}

	return nil
}
