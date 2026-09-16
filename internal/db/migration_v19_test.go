// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"path/filepath"
	"testing"
)

func TestMigrationV19ConvertsTimestampsToEpochMillis(t *testing.T) {
	tmp := t.TempDir()

	conn, err := openSQLiteConnection(context.Background(), filepath.Join(tmp, "db.sqlite3"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	defer func() { _ = conn.Close() }()

	ctx := context.Background()

	if err := runMigrations(ctx, conn, 18); err != nil {
		t.Fatalf("migrate to v18: %v", err)
	}

	if _, err := conn.ExecContext(ctx, "PRAGMA foreign_keys = OFF"); err != nil {
		t.Fatalf("disable foreign keys: %v", err)
	}

	seed := []string{
		`INSERT INTO audit_logs (id, timestamp, level, actor, action, ip, details)
		   VALUES ('a1', '2026-01-01T00:00:00Z', 'info', 'admin', 'create', '127.0.0.1', '')`,
		`INSERT INTO audit_logs (id, timestamp, level, actor, action, ip, details)
		   VALUES ('a2', '2026-01-01T00:00:00.5Z', 'info', 'admin', 'create', '127.0.0.1', '')`,
		`INSERT INTO audit_logs (id, timestamp, level, actor, action, ip, details)
		   VALUES ('a3', '2026-01-01T00:00:00.123456789Z', 'info', 'admin', 'create', '127.0.0.1', '')`,
		`INSERT INTO audit_logs (id, timestamp, level, actor, action, ip, details)
		   VALUES ('a4', '2025-12-31T19:00:00-05:00', 'info', 'admin', 'create', '127.0.0.1', '')`,
		`INSERT INTO network_logs (id, timestamp, protocol, message_type, direction, local_address, remote_address, raw, details, radio_name)
		   VALUES (7, '2026-01-01T00:00:01Z', 'NGAP', 'Setup', 'inbound', '1.1.1.1', '2.2.2.2', X'00', '', 'gnb')`,
		`INSERT INTO flow_reports (id, subscriber_id, source_ip, destination_ip, source_port, destination_port, protocol, packets, bytes, start_time, end_time, direction, action)
		   VALUES (3, '001010000000001', '10.0.0.1', '8.8.8.8', 1, 53, 17, 1, 1, '2026-01-01T00:00:00Z', '2026-01-01T00:00:02.25Z', 'uplink', 0)`,
	}

	for _, s := range seed {
		if _, err := conn.ExecContext(ctx, s); err != nil {
			t.Fatalf("seed %q: %v", s, err)
		}
	}

	if err := runMigrations(ctx, conn, 19); err != nil {
		t.Fatalf("migrate to v19: %v", err)
	}

	const jan1 = 1767225600000

	want := map[string]int64{
		"SELECT timestamp FROM audit_logs WHERE id = 'a1'": jan1,
		"SELECT timestamp FROM audit_logs WHERE id = 'a2'": jan1 + 500,
		"SELECT timestamp FROM audit_logs WHERE id = 'a3'": jan1 + 123,
		"SELECT timestamp FROM audit_logs WHERE id = 'a4'": jan1,
		"SELECT timestamp FROM network_logs WHERE id = 7":  jan1 + 1000,
		"SELECT start_time FROM flow_reports WHERE id = 3": jan1,
		"SELECT end_time FROM flow_reports WHERE id = 3":   jan1 + 2250,
	}

	for query, expected := range want {
		var got int64
		if err := conn.QueryRowContext(ctx, query).Scan(&got); err != nil {
			t.Fatalf("%s: %v", query, err)
		}

		if got != expected {
			t.Fatalf("%s: expected %d, got %d", query, expected, got)
		}
	}

	for _, spec := range []struct{ table, column string }{
		{AuditLogsTableName, "timestamp"},
		{RadioEventsTableName, "timestamp"},
		{FlowReportsTableName, "start_time"},
		{FlowReportsTableName, "end_time"},
	} {
		var typ string
		if err := conn.QueryRowContext(ctx,
			`SELECT type FROM pragma_table_info(?) WHERE name = ?`, spec.table, spec.column).Scan(&typ); err != nil {
			t.Fatalf("read %s.%s type: %v", spec.table, spec.column, err)
		}

		if typ != "INTEGER" {
			t.Fatalf("%s.%s: expected INTEGER, got %s", spec.table, spec.column, typ)
		}
	}
}

func TestMigrationV19PreservesIndexes(t *testing.T) {
	tmp := t.TempDir()

	conn, err := openSQLiteConnection(context.Background(), filepath.Join(tmp, "db.sqlite3"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	defer func() { _ = conn.Close() }()

	ctx := context.Background()

	if err := runMigrations(ctx, conn, 19); err != nil {
		t.Fatalf("migrate to v19: %v", err)
	}

	wanted := []string{
		"idx_network_logs_protocol",
		"idx_network_logs_timestamp",
		"idx_network_logs_message_type",
		"idx_network_logs_direction",
		"idx_network_logs_local_address",
		"idx_network_logs_remote_address",
		"idx_network_logs_radio_name",
		"idx_flow_reports_subscriber_id",
		"idx_flow_reports_end_time",
		"idx_flow_reports_protocol",
		"idx_flow_reports_source_ip",
		"idx_flow_reports_destination_ip",
	}

	for _, name := range wanted {
		var found string
		if err := conn.QueryRowContext(ctx,
			`SELECT name FROM sqlite_master WHERE type = 'index' AND name = ?`, name).Scan(&found); err != nil {
			t.Fatalf("index %s missing after v19: %v", name, err)
		}
	}
}
