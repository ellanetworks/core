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
		OperatorTableName,
		"network_slices",
		"profiles",
		"profile_network_configs",
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
		policiesTableName,
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
		fmt.Sprintf(v1CreatePoliciesTable, policiesTableName),
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

func applyV4(t *testing.T, db *sql.DB) {
	t.Helper()

	ctx := context.Background()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}

	if err := migrateV4(ctx, tx); err != nil {
		_ = tx.Rollback()

		t.Fatalf("migrateV4 failed: %v", err)
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("failed to commit migrateV4: %v", err)
	}
}

func applyV5(t *testing.T, db *sql.DB) {
	t.Helper()

	ctx := context.Background()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}

	if err := migrateV5(ctx, tx); err != nil {
		_ = tx.Rollback()

		t.Fatalf("migrateV5 failed: %v", err)
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("failed to commit migrateV5: %v", err)
	}
}

func applyAllBeforeV5(t *testing.T, db *sql.DB) {
	t.Helper()

	applyV1(t, db)
	applyV2(t, db)
	applyV3(t, db)
	applyV4(t, db)
}

func TestMigrateV5_CreatesNewTables(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	applyAllBeforeV5(t, db)

	// Seed operator with sst/sd so V5 can migrate them.
	_, err := db.ExecContext(ctx, fmt.Sprintf(
		"INSERT INTO %s (id, mcc, mnc, operatorCode, sst, sd) VALUES (1, '001', '01', 'abc123', 1, X'102030')",
		OperatorTableName))
	if err != nil {
		t.Fatalf("failed to insert operator: %v", err)
	}

	applyV5(t, db)

	// Verify new tables exist.
	tables := allTableNames(t, db)

	tableSet := make(map[string]bool, len(tables))
	for _, name := range tables {
		tableSet[name] = true
	}

	for _, expected := range []string{"network_slices", "profiles", "profile_network_configs"} {
		if !tableSet[expected] {
			t.Errorf("expected table %q not found; got tables: %v", expected, tables)
		}
	}

	// Verify policies table was dropped.
	if tableSet[policiesTableName] {
		t.Error("policies table should have been dropped by V5")
	}
}

func TestMigrateV5_MigratesSliceFromOperator(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	applyAllBeforeV5(t, db)

	_, err := db.ExecContext(ctx, fmt.Sprintf(
		"INSERT INTO %s (id, mcc, mnc, operatorCode, sst, sd) VALUES (1, '001', '01', 'abc123', 1, X'102030')",
		OperatorTableName))
	if err != nil {
		t.Fatalf("failed to insert operator: %v", err)
	}

	applyV5(t, db)

	// Verify network_slices got populated from operator.
	var (
		sst      int
		sd, name string
	)

	err = db.QueryRowContext(ctx, "SELECT sst, sd, name FROM network_slices LIMIT 1").Scan(&sst, &sd, &name)
	if err != nil {
		t.Fatalf("failed to query network_slices: %v", err)
	}

	if sst != 1 {
		t.Errorf("network_slices.sst = %d, want 1", sst)
	}

	if sd != "102030" {
		t.Errorf("network_slices.sd = %q, want %q", sd, "102030")
	}

	if name != "default" {
		t.Errorf("network_slices.name = %q, want %q", name, "default")
	}

	// Verify operator no longer has sst/sd columns.
	_, err = db.ExecContext(ctx, "SELECT sst FROM operator WHERE id=1")
	if err == nil {
		t.Error("expected error querying sst from operator (column should be removed)")
	}
}

func TestMigrateV5_MigratesPolicies(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	applyAllBeforeV5(t, db)

	// Seed operator and data network.
	_, err := db.ExecContext(ctx, fmt.Sprintf(
		"INSERT INTO %s (id, mcc, mnc, operatorCode, sst, sd) VALUES (1, '001', '01', 'abc123', 1, X'102030')",
		OperatorTableName))
	if err != nil {
		t.Fatalf("failed to insert operator: %v", err)
	}

	_, err = db.ExecContext(ctx,
		"INSERT INTO data_networks (id, name, ipPool, dns, mtu) VALUES (1, 'internet', '10.45.0.0/22', '8.8.8.8', 1400)")
	if err != nil {
		t.Fatalf("failed to insert data network: %v", err)
	}

	// Seed a policy.
	_, err = db.ExecContext(ctx,
		"INSERT INTO policies (id, name, bitrateUplink, bitrateDownlink, var5qi, arp, dataNetworkID) VALUES (1, 'gold', '100 Mbps', '200 Mbps', 9, 1, 1)")
	if err != nil {
		t.Fatalf("failed to insert policy: %v", err)
	}

	applyV5(t, db)

	// Verify profile was created from policy.
	var profName string

	err = db.QueryRowContext(ctx, "SELECT name FROM profiles WHERE name='gold'").
		Scan(&profName)
	if err != nil {
		t.Fatalf("failed to query profile: %v", err)
	}

	if profName != "gold" {
		t.Errorf("profile.name = %q, want %q", profName, "gold")
	}

	// Verify profile_network_configs was created.
	var (
		var5qi, arp      int
		sessUp, sessDown string
	)

	err = db.QueryRowContext(ctx,
		"SELECT var5qi, arp, sessionAmbrUplink, sessionAmbrDownlink FROM profile_network_configs LIMIT 1").
		Scan(&var5qi, &arp, &sessUp, &sessDown)
	if err != nil {
		t.Fatalf("failed to query profile_network_configs: %v", err)
	}

	if var5qi != 9 {
		t.Errorf("config.var5qi = %d, want 9", var5qi)
	}

	if arp != 1 {
		t.Errorf("config.arp = %d, want 1", arp)
	}

	if sessUp != "100 Mbps" {
		t.Errorf("config.sessionAmbrUplink = %q, want %q", sessUp, "100 Mbps")
	}

	if sessDown != "200 Mbps" {
		t.Errorf("config.sessionAmbrDownlink = %q, want %q", sessDown, "200 Mbps")
	}
}

func TestMigrateV5_MigratesSubscribers(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	applyAllBeforeV5(t, db)

	// Seed operator, data network, policy, subscriber.
	_, err := db.ExecContext(ctx, fmt.Sprintf(
		"INSERT INTO %s (id, mcc, mnc, operatorCode, sst, sd) VALUES (1, '001', '01', 'abc123', 1, X'102030')",
		OperatorTableName))
	if err != nil {
		t.Fatalf("failed to insert operator: %v", err)
	}

	_, err = db.ExecContext(ctx,
		"INSERT INTO data_networks (id, name, ipPool, dns, mtu) VALUES (1, 'internet', '10.45.0.0/22', '8.8.8.8', 1400)")
	if err != nil {
		t.Fatalf("failed to insert data network: %v", err)
	}

	_, err = db.ExecContext(ctx,
		"INSERT INTO policies (id, name, bitrateUplink, bitrateDownlink, var5qi, arp, dataNetworkID) VALUES (1, 'gold', '100 Mbps', '200 Mbps', 9, 1, 1)")
	if err != nil {
		t.Fatalf("failed to insert policy: %v", err)
	}

	_, err = db.ExecContext(ctx,
		"INSERT INTO subscribers (id, imsi, sequenceNumber, permanentKey, opc, policyID) VALUES (1, '001010000000001', '000000000001', '00112233445566778899aabbccddeeff', '00112233445566778899aabbccddeeff', 1)")
	if err != nil {
		t.Fatalf("failed to insert subscriber: %v", err)
	}

	applyV5(t, db)

	// Verify subscriber now has profileID (not policyID).
	var profileID int

	err = db.QueryRowContext(ctx, "SELECT profileID FROM subscribers WHERE imsi='001010000000001'").Scan(&profileID)
	if err != nil {
		t.Fatalf("failed to query subscriber profileID: %v", err)
	}

	// profileID should map to the profile that was created from the policy.
	var profName string

	err = db.QueryRowContext(ctx, "SELECT name FROM profiles WHERE id=?", profileID).Scan(&profName)
	if err != nil {
		t.Fatalf("failed to query profile by id: %v", err)
	}

	if profName != "gold" {
		t.Errorf("subscriber's profile name = %q, want %q", profName, "gold")
	}

	// Verify policyID column no longer exists.
	_, err = db.ExecContext(ctx, "SELECT policyID FROM subscribers LIMIT 1")
	if err == nil {
		t.Error("expected error querying policyID from subscribers (column should be removed)")
	}
}
