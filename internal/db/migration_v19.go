// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"fmt"
)

const toEpochMillis = `CAST(round(unixepoch(%s, 'subsec') * 1000) AS INTEGER)`

func epochMillisExpr(column string) string {
	return fmt.Sprintf(toEpochMillis, column)
}

type timestampTableRebuild struct {
	table      string
	newSchema  string
	selectCols string
	indices    []string
}

func migrateV19(ctx context.Context, tx *sql.Tx) error {
	rebuilds := []timestampTableRebuild{
		{
			table: AuditLogsTableName,
			newSchema: `CREATE TABLE %s_new (
				id         TEXT PRIMARY KEY,
				timestamp  INTEGER NOT NULL,
				level      TEXT NOT NULL,
				actor      TEXT NOT NULL DEFAULT '',
				action     TEXT NOT NULL,
				ip         TEXT NOT NULL DEFAULT '',
				details    TEXT NOT NULL DEFAULT ''
			)`,
			selectCols: "id, " + epochMillisExpr("timestamp") + ", level, actor, action, ip, details",
		},
		{
			table: RadioEventsTableName,
			newSchema: `CREATE TABLE %s_new (
				id             INTEGER PRIMARY KEY AUTOINCREMENT,
				timestamp      INTEGER NOT NULL,
				protocol       TEXT NOT NULL,
				message_type   TEXT NOT NULL,
				direction      TEXT NOT NULL DEFAULT '',
				local_address  TEXT NOT NULL DEFAULT '',
				remote_address TEXT NOT NULL DEFAULT '',
				raw            BLOB NOT NULL,
				details        TEXT NOT NULL DEFAULT '',
				radio_name     TEXT NOT NULL DEFAULT ''
			)`,
			selectCols: "id, " + epochMillisExpr("timestamp") + ", protocol, message_type, direction, local_address, remote_address, raw, details, radio_name",
			indices: []string{
				"CREATE INDEX idx_network_logs_protocol ON %s (protocol)",
				"CREATE INDEX idx_network_logs_timestamp ON %s (timestamp)",
				"CREATE INDEX idx_network_logs_message_type ON %s (message_type)",
				"CREATE INDEX idx_network_logs_direction ON %s (direction)",
				"CREATE INDEX idx_network_logs_local_address ON %s (local_address)",
				"CREATE INDEX idx_network_logs_remote_address ON %s (remote_address)",
				"CREATE INDEX idx_network_logs_radio_name ON %s (radio_name)",
			},
		},
		{
			table: FlowReportsTableName,
			newSchema: `CREATE TABLE %s_new (
				id               INTEGER PRIMARY KEY AUTOINCREMENT,
				subscriber_id    TEXT NOT NULL,
				source_ip        TEXT NOT NULL,
				destination_ip   TEXT NOT NULL,
				source_port      INTEGER NOT NULL DEFAULT 0,
				destination_port INTEGER NOT NULL DEFAULT 0,
				protocol         INTEGER NOT NULL,
				packets          INTEGER NOT NULL,
				bytes            INTEGER NOT NULL,
				start_time       INTEGER NOT NULL,
				end_time         INTEGER NOT NULL,
				direction        TEXT NOT NULL,
				action           INT NOT NULL DEFAULT 0,

				FOREIGN KEY (subscriber_id) REFERENCES subscribers(imsi) ON DELETE CASCADE
			)`,
			selectCols: "id, subscriber_id, source_ip, destination_ip, source_port, destination_port, protocol, packets, bytes, " +
				epochMillisExpr("start_time") + ", " + epochMillisExpr("end_time") + ", direction, action",
			indices: []string{
				"CREATE INDEX idx_flow_reports_subscriber_id ON %s (subscriber_id)",
				"CREATE INDEX idx_flow_reports_end_time ON %s (end_time, start_time)",
				"CREATE INDEX idx_flow_reports_protocol ON %s (protocol)",
				"CREATE INDEX idx_flow_reports_source_ip ON %s (source_ip)",
				"CREATE INDEX idx_flow_reports_destination_ip ON %s (destination_ip)",
			},
		},
	}

	for _, r := range rebuilds {
		if err := rebuildTimestampTable(ctx, tx, r); err != nil {
			return err
		}
	}

	return nil
}

func rebuildTimestampTable(ctx context.Context, tx *sql.Tx, r timestampTableRebuild) error {
	if _, err := tx.ExecContext(ctx, fmt.Sprintf(r.newSchema, r.table)); err != nil {
		return fmt.Errorf("failed to create %s_new: %w", r.table, err)
	}

	copyStmt := fmt.Sprintf("INSERT INTO %s_new SELECT %s FROM %s", r.table, r.selectCols, r.table)
	if _, err := tx.ExecContext(ctx, copyStmt); err != nil {
		return fmt.Errorf("failed to copy rows into %s_new: %w", r.table, err)
	}

	if _, err := tx.ExecContext(ctx, fmt.Sprintf("DROP TABLE %s", r.table)); err != nil {
		return fmt.Errorf("failed to drop %s: %w", r.table, err)
	}

	if _, err := tx.ExecContext(ctx, fmt.Sprintf("ALTER TABLE %s_new RENAME TO %s", r.table, r.table)); err != nil {
		return fmt.Errorf("failed to rename %s_new: %w", r.table, err)
	}

	for _, idx := range r.indices {
		if _, err := tx.ExecContext(ctx, fmt.Sprintf(idx, r.table)); err != nil {
			return fmt.Errorf("failed to recreate index on %s: %w", r.table, err)
		}
	}

	return nil
}
