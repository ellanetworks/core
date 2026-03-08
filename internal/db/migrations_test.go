// Copyright 2024 Ella Networks

package db

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory database: %v", err)
	}

	db.SetMaxOpenConns(1)

	ctx := context.Background()
	pragmas := []string{
		"PRAGMA busy_timeout = 5000;",
		"PRAGMA journal_mode = WAL;",
		"PRAGMA synchronous = NORMAL;",
		"PRAGMA foreign_keys = ON;",
	}

	for _, p := range pragmas {
		if _, err := db.ExecContext(ctx, p); err != nil {
			t.Fatalf("failed to execute pragma %q: %v", p, err)
		}
	}

	t.Cleanup(func() { _ = db.Close() })

	return db
}

func allTableNames(t *testing.T, db *sql.DB) []string {
	t.Helper()

	ctx := context.Background()

	rows, err := db.QueryContext(ctx, "SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name")
	if err != nil {
		t.Fatalf("failed to query table names: %v", err)
	}

	defer func() { _ = rows.Close() }()

	var names []string

	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("failed to scan table name: %v", err)
		}

		names = append(names, name)
	}

	return names
}

func allIndexNames(t *testing.T, db *sql.DB) []string {
	t.Helper()

	ctx := context.Background()

	rows, err := db.QueryContext(ctx, "SELECT name FROM sqlite_master WHERE type='index' AND name NOT LIKE 'sqlite_%' ORDER BY name")
	if err != nil {
		t.Fatalf("failed to query index names: %v", err)
	}

	defer func() { _ = rows.Close() }()

	var names []string

	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("failed to scan index name: %v", err)
		}

		names = append(names, name)
	}

	return names
}

func schemaVersion(t *testing.T, db *sql.DB) int {
	t.Helper()

	ctx := context.Background()

	var v int

	err := db.QueryRowContext(ctx, "SELECT version FROM schema_version WHERE id = 1").Scan(&v)
	if err != nil {
		t.Fatalf("failed to read schema version: %v", err)
	}

	return v
}

func TestMigrationRegistryInvariants(t *testing.T) {
	if len(migrations) == 0 {
		t.Fatal("migrations slice is empty")
	}

	for i, m := range migrations {
		if m.version != i+1 {
			t.Errorf("migration at index %d has version %d, expected %d", i, m.version, i+1)
		}

		if m.description == "" {
			t.Errorf("migration %d has empty description", m.version)
		}

		if m.fn == nil {
			t.Errorf("migration %d has nil function", m.version)
		}
	}
}

func TestRunMigrations_FreshDatabase(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	if err := RunMigrations(ctx, db); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}

	got := schemaVersion(t, db)

	want := latestVersion()
	if got != want {
		t.Errorf("schema version = %d, want %d", got, want)
	}

	expectedTables := []string{
		APITokensTableName,
		AuditLogsTableName,
		DailyUsageTableName,
		DataNetworksTableName,
		FlowAccountingSettingsTableName,
		FlowReportsTableName,
		N3SettingsTableName,
		NATSettingsTableName,
		OperatorTableName,
		PoliciesTableName,
		RadioEventsTableName,
		RetentionPolicyTableName,
		RoutesTableName,
		"schema_version",
		SessionsTableName,
		SubscribersTableName,
		UsersTableName,
	}

	tables := allTableNames(t, db)

	tableSet := make(map[string]bool, len(tables))
	for _, name := range tables {
		tableSet[name] = true
	}

	for _, expected := range expectedTables {
		if !tableSet[expected] {
			t.Errorf("expected table %q not found; got tables: %v", expected, tables)
		}
	}
}

func TestRunMigrations_Idempotent(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	if err := RunMigrations(ctx, db); err != nil {
		t.Fatalf("first RunMigrations failed: %v", err)
	}

	tablesAfterFirst := allTableNames(t, db)
	versionAfterFirst := schemaVersion(t, db)

	if err := RunMigrations(ctx, db); err != nil {
		t.Fatalf("second RunMigrations failed: %v", err)
	}

	tablesAfterSecond := allTableNames(t, db)
	versionAfterSecond := schemaVersion(t, db)

	if versionAfterFirst != versionAfterSecond {
		t.Errorf("version changed: first=%d, second=%d", versionAfterFirst, versionAfterSecond)
	}

	if len(tablesAfterFirst) != len(tablesAfterSecond) {
		t.Errorf("table count changed: first=%d, second=%d", len(tablesAfterFirst), len(tablesAfterSecond))
	}

	for i := range tablesAfterFirst {
		if tablesAfterFirst[i] != tablesAfterSecond[i] {
			t.Errorf("table mismatch at index %d: first=%q, second=%q", i, tablesAfterFirst[i], tablesAfterSecond[i])
		}
	}
}

func TestRunMigrations_FailedMigrationRollsBack(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	if err := RunMigrations(ctx, db); err != nil {
		t.Fatalf("initial RunMigrations failed: %v", err)
	}

	versionBefore := schemaVersion(t, db)

	originalMigrations := migrations

	defer func() { migrations = originalMigrations }()

	migrations = append(migrations, migration{
		version:     latestVersion() + 1,
		description: "deliberately broken",
		fn: func(ctx context.Context, tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx, "CREATE TABLE this_is_invalid (")

			return err
		},
	})

	err := RunMigrations(ctx, db)
	if err == nil {
		t.Fatal("expected RunMigrations to fail, but it succeeded")
	}

	versionAfter := schemaVersion(t, db)
	if versionAfter != versionBefore {
		t.Errorf("version advanced despite failure: before=%d, after=%d", versionBefore, versionAfter)
	}

	tables := allTableNames(t, db)
	for _, name := range tables {
		if name == "this_is_invalid" {
			t.Error("broken migration's table was created despite rollback")
		}
	}
}

func TestMigrateV1_AllTablesCreated(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}

	if err := migrateV1(ctx, tx); err != nil {
		_ = tx.Rollback()

		t.Fatalf("migrateV1 failed: %v", err)
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	expectedTables := []string{
		APITokensTableName,
		AuditLogsTableName,
		DailyUsageTableName,
		DataNetworksTableName,
		FlowAccountingSettingsTableName,
		FlowReportsTableName,
		N3SettingsTableName,
		NATSettingsTableName,
		OperatorTableName,
		PoliciesTableName,
		RadioEventsTableName,
		RetentionPolicyTableName,
		RoutesTableName,
		SessionsTableName,
		SubscribersTableName,
		UsersTableName,
	}

	tables := allTableNames(t, db)

	tableSet := make(map[string]bool, len(tables))
	for _, name := range tables {
		tableSet[name] = true
	}

	for _, expected := range expectedTables {
		if !tableSet[expected] {
			t.Errorf("expected table %q not found; got tables: %v", expected, tables)
		}
	}

	expectedIndexes := []string{
		"idx_network_logs_protocol",
		"idx_network_logs_timestamp",
		"idx_network_logs_message_type",
		"idx_network_logs_direction",
		"idx_network_logs_local_address",
		"idx_network_logs_remote_address",
		"idx_flow_reports_subscriber_id",
		"idx_flow_reports_end_time",
		"idx_flow_reports_protocol",
		"idx_flow_reports_source_ip",
		"idx_flow_reports_destination_ip",
	}

	indexes := allIndexNames(t, db)

	indexSet := make(map[string]bool, len(indexes))
	for _, name := range indexes {
		indexSet[name] = true
	}

	for _, expected := range expectedIndexes {
		if !indexSet[expected] {
			t.Errorf("expected index %q not found; got indexes: %v", expected, indexes)
		}
	}
}

func TestMigrateV1_ExistingDatabase(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	oldStyleDDL := []string{
		fmt.Sprintf(v1CreateOperatorTable, OperatorTableName),
		fmt.Sprintf(v1CreateRoutesTable, RoutesTableName),
		fmt.Sprintf(v1CreateDataNetworksTable, DataNetworksTableName),
		fmt.Sprintf(v1CreatePoliciesTable, PoliciesTableName),
		fmt.Sprintf(v1CreateSubscribersTable, SubscribersTableName),
		fmt.Sprintf(v1CreateUsersTable, UsersTableName),
		fmt.Sprintf(v1CreateSessionsTable, SessionsTableName),
	}

	for _, stmt := range oldStyleDDL {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("failed to create pre-existing table: %v", err)
		}
	}

	_, err := db.ExecContext(ctx, fmt.Sprintf(
		"INSERT INTO %s (id, mcc, mnc, operatorCode, sst, homeNetworkPrivateKey) VALUES (1, '001', '01', 'abc123', 1, 'deadbeef')",
		OperatorTableName))
	if err != nil {
		t.Fatalf("failed to insert test data: %v", err)
	}

	if err := RunMigrations(ctx, db); err != nil {
		t.Fatalf("RunMigrations on existing database failed: %v", err)
	}

	var mcc string

	err = db.QueryRowContext(ctx, fmt.Sprintf("SELECT mcc FROM %s WHERE id=1", OperatorTableName)).Scan(&mcc)
	if err != nil {
		t.Fatalf("failed to query operator after migration: %v", err)
	}

	if mcc != "001" {
		t.Errorf("operator mcc = %q, want %q", mcc, "001")
	}

	got := schemaVersion(t, db)

	want := latestVersion()
	if got != want {
		t.Errorf("schema version = %d, want %d", got, want)
	}
}

func TestRunMigrations_Incremental(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	if err := RunMigrations(ctx, db); err != nil {
		t.Fatalf("initial RunMigrations failed: %v", err)
	}

	_, err := db.ExecContext(ctx, fmt.Sprintf(
		"INSERT INTO %s (id, mcc, mnc, operatorCode, sst, homeNetworkPrivateKey) VALUES (1, '310', '260', 'testop', 1, 'keyhex')",
		OperatorTableName))
	if err != nil {
		t.Fatalf("failed to insert test data: %v", err)
	}

	originalMigrations := migrations

	defer func() { migrations = originalMigrations }()

	migrations = append(migrations, migration{
		version:     latestVersion() + 1,
		description: "add test_column to operator",
		fn: func(ctx context.Context, tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx,
				fmt.Sprintf("ALTER TABLE %s ADD COLUMN test_column TEXT NOT NULL DEFAULT 'hello'", OperatorTableName))

			return err
		},
	})

	if err := RunMigrations(ctx, db); err != nil {
		t.Fatalf("incremental RunMigrations failed: %v", err)
	}

	got := schemaVersion(t, db)
	if got != 2 {
		t.Errorf("schema version = %d, want 2", got)
	}

	var testCol string

	err = db.QueryRowContext(ctx, fmt.Sprintf("SELECT test_column FROM %s WHERE id=1", OperatorTableName)).Scan(&testCol)
	if err != nil {
		t.Fatalf("failed to query new column: %v", err)
	}

	if testCol != "hello" {
		t.Errorf("test_column = %q, want %q", testCol, "hello")
	}
}

func TestLatestVersion(t *testing.T) {
	got := latestVersion()
	if got != len(migrations) {
		t.Errorf("latestVersion() = %d, want %d", got, len(migrations))
	}
}
