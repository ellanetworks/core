// Copyright 2026 Ella Networks

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
		HomeNetworkKeysTableName,
		N3SettingsTableName,
		NATSettingsTableName,
		NetworkRulesTableName,
		NetworkSlicesTableName,
		OperatorTableName,
		PoliciesTableName,
		ProfilesTableName,
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
		"INSERT INTO %s (id, mcc, mnc, operatorCode) VALUES (1, '310', '260', 'testop')",
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
	if got != latestVersion() {
		t.Errorf("schema version = %d, want %d", got, latestVersion())
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

func applyV1(t *testing.T, db *sql.DB) {
	t.Helper()

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
		t.Fatalf("failed to commit migrateV1: %v", err)
	}
}

func applyV2(t *testing.T, db *sql.DB) {
	t.Helper()

	ctx := context.Background()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}

	if err := migrateV2(ctx, tx); err != nil {
		_ = tx.Rollback()

		t.Fatalf("migrateV2 failed: %v", err)
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("failed to commit migrateV2: %v", err)
	}
}

func applyV3(t *testing.T, db *sql.DB) {
	t.Helper()

	ctx := context.Background()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}

	if err := migrateV3(ctx, tx); err != nil {
		_ = tx.Rollback()

		t.Fatalf("migrateV3 failed: %v", err)
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("failed to commit migrateV3: %v", err)
	}
}

func TestMigrateV2_AddsColumns(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	applyV1(t, db)

	// Insert an operator row before V2.
	_, err := db.ExecContext(ctx, fmt.Sprintf(
		"INSERT INTO %s (id, mcc, mnc, operatorCode, sst, homeNetworkPrivateKey) VALUES (1, '001', '01', 'abc123', 1, 'deadbeef')",
		OperatorTableName))
	if err != nil {
		t.Fatalf("failed to insert operator: %v", err)
	}

	applyV2(t, db)

	var ciphering, integrity, spnFullName, spnShortName string

	err = db.QueryRowContext(ctx,
		fmt.Sprintf("SELECT ciphering, integrity, spnFullName, spnShortName FROM %s WHERE id=1", OperatorTableName),
	).Scan(&ciphering, &integrity, &spnFullName, &spnShortName)
	if err != nil {
		t.Fatalf("failed to query new columns: %v", err)
	}

	if want := `["NEA2","NEA1","NEA0"]`; ciphering != want {
		t.Errorf("ciphering = %q, want %q", ciphering, want)
	}

	if want := `["NIA2","NIA1","NIA0"]`; integrity != want {
		t.Errorf("integrity = %q, want %q", integrity, want)
	}

	if want := "Ella Networks"; spnFullName != want {
		t.Errorf("spnFullName = %q, want %q", spnFullName, want)
	}

	if want := "Ella"; spnShortName != want {
		t.Errorf("spnShortName = %q, want %q", spnShortName, want)
	}
}

func TestMigrateV2_MigratesExistingKey(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	applyV1(t, db)

	const testPrivateKey = "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"

	_, err := db.ExecContext(ctx, fmt.Sprintf(
		"INSERT INTO %s (id, mcc, mnc, operatorCode, sst, homeNetworkPrivateKey) VALUES (1, '001', '01', 'abc123', 1, ?)",
		OperatorTableName), testPrivateKey)
	if err != nil {
		t.Fatalf("failed to insert operator with private key: %v", err)
	}

	applyV2(t, db)

	var keyIdentifier int

	var scheme, privateKey string

	err = db.QueryRowContext(ctx,
		fmt.Sprintf("SELECT key_identifier, scheme, private_key FROM %s WHERE id=1", HomeNetworkKeysTableName),
	).Scan(&keyIdentifier, &scheme, &privateKey)
	if err != nil {
		t.Fatalf("failed to query migrated key: %v", err)
	}

	if keyIdentifier != 0 {
		t.Errorf("key_identifier = %d, want 0", keyIdentifier)
	}

	if scheme != "A" {
		t.Errorf("scheme = %q, want \"A\"", scheme)
	}

	if privateKey != testPrivateKey {
		t.Errorf("private_key = %q, want %q", privateKey, testPrivateKey)
	}
}

func TestMigrateV2_NoExistingKey(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	applyV1(t, db)

	_, err := db.ExecContext(ctx, fmt.Sprintf(
		"INSERT INTO %s (id, mcc, mnc, operatorCode, sst, homeNetworkPrivateKey) VALUES (1, '001', '01', 'abc123', 1, '')",
		OperatorTableName))
	if err != nil {
		t.Fatalf("failed to insert operator: %v", err)
	}

	applyV2(t, db)

	var count int

	err = db.QueryRowContext(ctx,
		fmt.Sprintf("SELECT COUNT(*) FROM %s", HomeNetworkKeysTableName),
	).Scan(&count)
	if err != nil {
		t.Fatalf("failed to count keys: %v", err)
	}

	if count != 0 {
		t.Errorf("expected 0 keys after migration with no prior key, got %d", count)
	}
}

// runMigrationsUpTo applies all registered migrations up to and including
// the given version, then stops. It temporarily truncates the global
// migrations slice so RunMigrations only sees the desired prefix.
func runMigrationsUpTo(t *testing.T, db *sql.DB, version int) {
	t.Helper()

	original := migrations

	defer func() { migrations = original }()

	if version > len(original) {
		t.Fatalf("requested version %d exceeds available migrations (%d)", version, len(original))
	}

	migrations = original[:version]

	if err := RunMigrations(context.Background(), db); err != nil {
		t.Fatalf("RunMigrations up to v%d failed: %v", version, err)
	}

	migrations = original
}

func TestMigrateV7_DataMigration(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	// Apply V1-V6 to get the pre-v7 schema.
	runMigrationsUpTo(t, db, 6)

	// Seed an operator with sst/sd (3-byte BLOB).
	_, err := db.ExecContext(ctx, fmt.Sprintf(
		"INSERT INTO %s (id, mcc, mnc, operatorCode, sst, sd, supportedTACs) VALUES (1, '001', '01', 'abc123', 1, X'102030', '[]')",
		OperatorTableName))
	if err != nil {
		t.Fatalf("failed to insert operator: %v", err)
	}

	// Seed a data network.
	_, err = db.ExecContext(ctx, fmt.Sprintf(
		"INSERT INTO %s (id, name, ipPool, dns, mtu) VALUES (1, 'internet', '10.0.0.0/24', '8.8.8.8', 1500)",
		DataNetworksTableName))
	if err != nil {
		t.Fatalf("failed to insert data network: %v", err)
	}

	// Seed two old-schema policies.
	_, err = db.ExecContext(ctx, fmt.Sprintf(
		"INSERT INTO %s (id, name, bitrateUplink, bitrateDownlink, var5qi, arp, dataNetworkID) VALUES (1, 'gold', '200 Mbps', '100 Mbps', 9, 1, 1)",
		PoliciesTableName))
	if err != nil {
		t.Fatalf("failed to insert policy 'gold': %v", err)
	}

	_, err = db.ExecContext(ctx, fmt.Sprintf(
		"INSERT INTO %s (id, name, bitrateUplink, bitrateDownlink, var5qi, arp, dataNetworkID) VALUES (2, 'silver', '50 Mbps', '25 Mbps', 8, 2, 1)",
		PoliciesTableName))
	if err != nil {
		t.Fatalf("failed to insert policy 'silver': %v", err)
	}

	// Seed a network rule referencing old policy 1 (gold).
	_, err = db.ExecContext(ctx, fmt.Sprintf(
		"INSERT INTO %s (id, policy_id, description, direction, protocol, port_low, port_high, action, precedence) VALUES (1, 1, 'allow all', 'outbound', 255, 0, 65535, 'allow', 100)",
		NetworkRulesTableName))
	if err != nil {
		t.Fatalf("failed to insert network rule: %v", err)
	}

	// Seed a subscriber referencing old policy 2 (silver).
	_, err = db.ExecContext(ctx, fmt.Sprintf(
		"INSERT INTO %s (id, imsi, sequenceNumber, permanentKey, opc, policyID) VALUES (1, '001010000000001', '000000000001', '00112233445566778899aabbccddeeff', '00112233445566778899aabbccddeeff', 2)",
		SubscribersTableName))
	if err != nil {
		t.Fatalf("failed to insert subscriber: %v", err)
	}

	// Seed daily_usage and flow_reports (child tables via FK to subscribers).
	_, err = db.ExecContext(ctx, fmt.Sprintf(
		"INSERT INTO %s (epoch_day, imsi, bytes_uplink, bytes_downlink) VALUES (20545, '001010000000001', 1000, 2000)",
		DailyUsageTableName))
	if err != nil {
		t.Fatalf("failed to insert daily_usage: %v", err)
	}

	_, err = db.ExecContext(ctx, fmt.Sprintf(
		"INSERT INTO %s (subscriber_id, source_ip, destination_ip, source_port, destination_port, protocol, packets, bytes, start_time, end_time, direction) VALUES ('001010000000001', '10.0.0.1', '8.8.8.8', 12345, 443, 6, 10, 500, '2026-04-02T10:00:00Z', '2026-04-02T10:00:05Z', 'uplink')",
		FlowReportsTableName))
	if err != nil {
		t.Fatalf("failed to insert flow_report: %v", err)
	}

	// Apply V7 migration. Disable FK like the runner does (PRAGMA is a no-op inside a tx).
	_, err = db.ExecContext(ctx, "PRAGMA foreign_keys = OFF")
	if err != nil {
		t.Fatalf("failed to disable foreign keys: %v", err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}

	if err := migrateV7(ctx, tx); err != nil {
		_ = tx.Rollback()

		t.Fatalf("migrateV7 failed: %v", err)
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("failed to commit migrateV7: %v", err)
	}

	_, err = db.ExecContext(ctx, "PRAGMA foreign_keys = ON")
	if err != nil {
		t.Fatalf("failed to re-enable foreign keys: %v", err)
	}

	// --- Assertions ---

	// 1. network_slices: one row with sst=1, sd='102030', name='default'
	var sliceSst int

	var sliceSd, sliceName string

	err = db.QueryRowContext(ctx,
		fmt.Sprintf("SELECT sst, sd, name FROM %s WHERE id = 1", NetworkSlicesTableName),
	).Scan(&sliceSst, &sliceSd, &sliceName)
	if err != nil {
		t.Fatalf("failed to query network_slices: %v", err)
	}

	if sliceSst != 1 {
		t.Errorf("network_slices.sst = %d, want 1", sliceSst)
	}

	if sliceSd != "102030" {
		t.Errorf("network_slices.sd = %q, want %q", sliceSd, "102030")
	}

	if sliceName != "default" {
		t.Errorf("network_slices.name = %q, want %q", sliceName, "default")
	}

	// 2. profiles: two rows (gold, silver) with UE-AMBR from old policies.
	var goldUplinkAmbr, goldDownlinkAmbr string

	err = db.QueryRowContext(ctx,
		fmt.Sprintf("SELECT ueAmbrUplink, ueAmbrDownlink FROM %s WHERE name = 'gold'", ProfilesTableName),
	).Scan(&goldUplinkAmbr, &goldDownlinkAmbr)
	if err != nil {
		t.Fatalf("failed to query profile 'gold': %v", err)
	}

	if goldUplinkAmbr != "200 Mbps" {
		t.Errorf("profiles[gold].ueAmbrUplink = %q, want %q", goldUplinkAmbr, "200 Mbps")
	}

	if goldDownlinkAmbr != "100 Mbps" {
		t.Errorf("profiles[gold].ueAmbrDownlink = %q, want %q", goldDownlinkAmbr, "100 Mbps")
	}

	// 3. policies (new schema): two rows with profileID, sliceID, dataNetworkID.
	var newPolicyProfileID, newPolicySliceID, newPolicyDnID, newPolicyVar5qi, newPolicyArp int

	var newPolicySessionUp, newPolicySessionDown string

	err = db.QueryRowContext(ctx,
		fmt.Sprintf("SELECT profileID, sliceID, dataNetworkID, var5qi, arp, sessionAmbrUplink, sessionAmbrDownlink FROM %s WHERE name = 'gold'", PoliciesTableName),
	).Scan(&newPolicyProfileID, &newPolicySliceID, &newPolicyDnID, &newPolicyVar5qi, &newPolicyArp, &newPolicySessionUp, &newPolicySessionDown)
	if err != nil {
		t.Fatalf("failed to query new policy 'gold': %v", err)
	}

	if newPolicyVar5qi != 9 {
		t.Errorf("policies[gold].var5qi = %d, want 9", newPolicyVar5qi)
	}

	if newPolicyArp != 1 {
		t.Errorf("policies[gold].arp = %d, want 1", newPolicyArp)
	}

	if newPolicySessionUp != "200 Mbps" {
		t.Errorf("policies[gold].sessionAmbrUplink = %q, want %q", newPolicySessionUp, "200 Mbps")
	}

	// profileID should reference the 'gold' profile.
	var profileName string

	err = db.QueryRowContext(ctx,
		fmt.Sprintf("SELECT name FROM %s WHERE id = ?", ProfilesTableName), newPolicyProfileID,
	).Scan(&profileName)
	if err != nil {
		t.Fatalf("failed to look up profile by ID %d: %v", newPolicyProfileID, err)
	}

	if profileName != "gold" {
		t.Errorf("policy 'gold' references profile %q, want %q", profileName, "gold")
	}

	// 4. network_rules: policy_id should reference the new 'gold' policy.
	var ruleNewPolicyID int

	err = db.QueryRowContext(ctx,
		fmt.Sprintf("SELECT policy_id FROM %s WHERE id = 1", NetworkRulesTableName),
	).Scan(&ruleNewPolicyID)
	if err != nil {
		t.Fatalf("failed to query network_rule: %v", err)
	}

	var rulePolicyName string

	err = db.QueryRowContext(ctx,
		fmt.Sprintf("SELECT name FROM %s WHERE id = ?", PoliciesTableName), ruleNewPolicyID,
	).Scan(&rulePolicyName)
	if err != nil {
		t.Fatalf("failed to look up policy by ID %d: %v", ruleNewPolicyID, err)
	}

	if rulePolicyName != "gold" {
		t.Errorf("network_rule references policy %q, want %q", rulePolicyName, "gold")
	}

	// 5. subscribers: profileID should reference the 'silver' profile.
	var subProfileID int

	err = db.QueryRowContext(ctx,
		fmt.Sprintf("SELECT profileID FROM %s WHERE id = 1", SubscribersTableName),
	).Scan(&subProfileID)
	if err != nil {
		t.Fatalf("failed to query subscriber: %v", err)
	}

	var subProfileName string

	err = db.QueryRowContext(ctx,
		fmt.Sprintf("SELECT name FROM %s WHERE id = ?", ProfilesTableName), subProfileID,
	).Scan(&subProfileName)
	if err != nil {
		t.Fatalf("failed to look up profile by ID %d: %v", subProfileID, err)
	}

	if subProfileName != "silver" {
		t.Errorf("subscriber references profile %q, want %q", subProfileName, "silver")
	}

	// 6. operator: sst and sd columns should no longer exist.
	_, err = db.QueryContext(ctx, fmt.Sprintf("SELECT sst FROM %s", OperatorTableName))
	if err == nil {
		t.Error("expected error querying operator.sst (column should be removed), but got nil")
	}

	// Verify core operator data is preserved.
	var opMcc, opMnc string

	err = db.QueryRowContext(ctx,
		fmt.Sprintf("SELECT mcc, mnc FROM %s WHERE id = 1", OperatorTableName),
	).Scan(&opMcc, &opMnc)
	if err != nil {
		t.Fatalf("failed to query operator: %v", err)
	}

	if opMcc != "001" || opMnc != "01" {
		t.Errorf("operator mcc/mnc = %q/%q, want 001/01", opMcc, opMnc)
	}

	// 7. policies_old should not exist.
	var count int

	err = db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='policies_old'",
	).Scan(&count)
	if err != nil {
		t.Fatalf("failed to check for policies_old table: %v", err)
	}

	if count != 0 {
		t.Error("policies_old table still exists after migration")
	}

	// 8. daily_usage rows must survive the subscribers table rebuild.
	var usageCount int

	err = db.QueryRowContext(ctx,
		fmt.Sprintf("SELECT COUNT(*) FROM %s", DailyUsageTableName),
	).Scan(&usageCount)
	if err != nil {
		t.Fatalf("failed to count daily_usage: %v", err)
	}

	if usageCount != 1 {
		t.Errorf("daily_usage row count = %d, want 1", usageCount)
	}

	// 9. flow_reports rows must survive the subscribers table rebuild.
	var flowCount int

	err = db.QueryRowContext(ctx,
		fmt.Sprintf("SELECT COUNT(*) FROM %s", FlowReportsTableName),
	).Scan(&flowCount)
	if err != nil {
		t.Fatalf("failed to count flow_reports: %v", err)
	}

	if flowCount != 1 {
		t.Errorf("flow_reports row count = %d, want 1", flowCount)
	}
}

func TestMigrateV8_DataMigration(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	// Apply V1-V6 to get the pre-v7 schema.
	runMigrationsUpTo(t, db, 7)

	// Seed a data network.
	_, err := db.ExecContext(ctx, fmt.Sprintf(
		"INSERT INTO %s (id, name, ipPool, dns, mtu) VALUES (1, 'internet', '10.0.0.0/24', '8.8.8.8', 1500)",
		DataNetworksTableName))
	if err != nil {
		t.Fatalf("failed to insert data network: %v", err)
	}

	// Seed a slice.
	_, err = db.ExecContext(ctx, fmt.Sprintf(
		"INSERT INTO %s (id, sst, sd, name) VALUES (1, 1, '102030', 'iot')",
		NetworkSlicesTableName))
	if err != nil {
		t.Fatalf("failed to insert slice 1 'iot': %v", err)
	}

	// Seed a profile.
	_, err = db.ExecContext(ctx, fmt.Sprintf(
		"INSERT INTO %s (id, name, ueAmbrUplink, ueAmbrDownlink) VALUES (1, 'silver', '50 Mbps', '25 Mbps')",
		ProfilesTableName))
	if err != nil {
		t.Fatalf("failed to insert profile 1 'silver': %v", err)
	}

	// Seed a policy.
	_, err = db.ExecContext(ctx, fmt.Sprintf(
		"INSERT INTO %s (id, name, profileID, sliceID, dataNetworkID, var5qi, arp, dataNetworkID, sessionAmbrUplink, sessionAmbrDownlink) VALUES (1, 'silver', 1, 1, 1, 8, 2, 1, '50 Mbps', '25 Mbps')",
		PoliciesTableName))
	if err != nil {
		t.Fatalf("failed to insert policy 1 'silver': %v", err)
	}

	// Seed a subscriber referencing policy 1 (silver).
	_, err = db.ExecContext(ctx, fmt.Sprintf(
		"INSERT INTO %s (id, imsi, sequenceNumber, permanentKey, opc, profileID) VALUES (1, '001010000000001', '000000000001', '00112233445566778899aabbccddeeff', '00112233445566778899aabbccddeeff', 1)",
		SubscribersTableName))
	if err != nil {
		t.Fatalf("failed to insert subscriber: %v", err)
	}

	_, err = db.ExecContext(ctx, fmt.Sprintf(
		"INSERT INTO %s (subscriber_id, source_ip, destination_ip, source_port, destination_port, protocol, packets, bytes, start_time, end_time, direction) VALUES ('001010000000001', '10.0.0.1', '8.8.8.8', 12345, 443, 6, 10, 500, '2026-04-02T10:00:00Z', '2026-04-02T10:00:05Z', 'uplink')",
		FlowReportsTableName))
	if err != nil {
		t.Fatalf("failed to insert flow_report: %v", err)
	}

	// Apply V8 migration. Disable FK like the runner does (PRAGMA is a no-op inside a tx).
	_, err = db.ExecContext(ctx, "PRAGMA foreign_keys = OFF")
	if err != nil {
		t.Fatalf("failed to disable foreign keys: %v", err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}

	if err := migrateV8(ctx, tx); err != nil {
		_ = tx.Rollback()

		t.Fatalf("migrateV8 failed: %v", err)
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("failed to commit migrateV8: %v", err)
	}

	_, err = db.ExecContext(ctx, "PRAGMA foreign_keys = ON")
	if err != nil {
		t.Fatalf("failed to re-enable foreign keys: %v", err)
	}

	// --- Assertions ---

	var flowAction int

	err = db.QueryRowContext(ctx,
		fmt.Sprintf("SELECT action FROM %s WHERE id = 1", FlowReportsTableName),
	).Scan(&flowAction)
	if err != nil {
		t.Fatalf("failed to read action from flow_reports: %v", err)
	}

	if flowAction != 0 {
		t.Errorf("flow_reports action = %d, want 0", flowAction)
	}
}
