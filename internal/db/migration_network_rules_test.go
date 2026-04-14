// Copyright 2026 Ella Networks

package db

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestMigrateV5_CreatesNetworkRulesTable(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	applyV1(t, db)
	applyV2(t, db)
	applyV3(t, db)
	applyV4(t, db)
	applyV5(t, db)

	rows, err := db.QueryContext(ctx, "PRAGMA table_info(network_rules)")
	if err != nil {
		t.Fatalf("failed to query table info: %v", err)
	}

	defer func() { _ = rows.Close() }()

	columnMap := make(map[string]bool)
	expectedColumns := map[string]bool{
		"id":            true,
		"policy_id":     true,
		"description":   true,
		"direction":     true,
		"remote_prefix": true,
		"protocol":      true,
		"port_low":      true,
		"port_high":     true,
		"action":        true,
		"precedence":    true,
		"created_at":    true,
		"updated_at":    true,
	}

	for rows.Next() {
		var (
			cid       int
			name, typ string
			notnull   int
			dflt      sql.NullString
			pk        int
		)

		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			t.Fatalf("failed to scan column info: %v", err)
		}

		columnMap[name] = true
	}

	if err := rows.Err(); err != nil {
		t.Fatalf("error iterating column info: %v", err)
	}

	for col := range expectedColumns {
		if !columnMap[col] {
			t.Errorf("expected column %q not found in network_rules table", col)
		}
	}
}

func TestMigrateV5_NetworkRulesTableStructure(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	applyV1(t, db)
	applyV2(t, db)
	applyV3(t, db)
	applyV4(t, db)
	applyV5(t, db)

	rows, err := db.QueryContext(ctx, "PRAGMA table_info(network_rules)")
	if err != nil {
		t.Fatalf("failed to query table info: %v", err)
	}

	defer func() { _ = rows.Close() }()

	type columnInfo struct {
		cid     int
		name    string
		typ     string
		notnull int
		pk      int
	}

	var columns []columnInfo

	for rows.Next() {
		var (
			cid       int
			name, typ string
			notnull   int
			dflt      sql.NullString
			pk        int
		)

		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			t.Fatalf("failed to scan column info: %v", err)
		}

		columns = append(columns, columnInfo{cid, name, typ, notnull, pk})
	}

	if len(columns) != 12 {
		t.Errorf("expected 12 columns, got %d", len(columns))
	}

	tests := map[string]struct {
		typ string
		nn  int
	}{
		"id":            {"INTEGER", 0},
		"policy_id":     {"INTEGER", 1},
		"description":   {"TEXT", 1},
		"direction":     {"TEXT", 1},
		"remote_prefix": {"TEXT", 0},
		"protocol":      {"INTEGER", 0},
		"port_low":      {"INTEGER", 0},
		"port_high":     {"INTEGER", 0},
		"action":        {"TEXT", 1},
		"precedence":    {"INTEGER", 1},
		"created_at":    {"TIMESTAMP", 1},
		"updated_at":    {"TIMESTAMP", 1},
	}

	for _, col := range columns {
		if expected, ok := tests[col.name]; ok {
			if col.typ != expected.typ {
				t.Errorf("column %q has type %s, want %s", col.name, col.typ, expected.typ)
			}

			if col.notnull != expected.nn {
				t.Errorf("column %q notnull=%d, want %d", col.name, col.notnull, expected.nn)
			}
		}
	}
}

func TestMigrateV5_ForeignKeyConstraintPolicyID(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	applyV1(t, db)
	applyV2(t, db)
	applyV3(t, db)
	applyV4(t, db)
	applyV5(t, db)

	rows, err := db.QueryContext(ctx, "PRAGMA foreign_key_list(network_rules)")
	if err != nil {
		t.Fatalf("failed to query foreign keys: %v", err)
	}

	defer func() { _ = rows.Close() }()

	fkMap := make(map[int]map[string]any)

	for rows.Next() {
		var (
			id       int
			seq      int
			table    string
			from     string
			to       string
			onUpdate string
			onDelete string
			match    string
		)

		if err := rows.Scan(&id, &seq, &table, &from, &to, &onUpdate, &onDelete, &match); err != nil {
			t.Fatalf("failed to scan foreign key info: %v", err)
		}

		if fkMap[id] == nil {
			fkMap[id] = make(map[string]any)
		}

		fkMap[id]["table"] = table
		fkMap[id]["from"] = from
		fkMap[id]["to"] = to
		fkMap[id]["onDelete"] = onDelete
	}

	if len(fkMap) < 1 {
		t.Errorf("expected at least 1 foreign key, got %d", len(fkMap))
	}

	policyFKFound := false

	for _, fk := range fkMap {
		if table, ok := fk["table"].(string); ok {
			if table == PoliciesTableName && fk["from"] == "policy_id" {
				policyFKFound = true
			}
		}
	}

	if !policyFKFound {
		t.Error("foreign key constraint from policy_id to policies table not found")
	}
}

func TestMigrateV5_UniqueConstraint(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	applyV1(t, db)
	applyV2(t, db)
	applyV3(t, db)
	applyV4(t, db)
	applyV5(t, db)

	rows, err := db.QueryContext(ctx, "PRAGMA index_info(sqlite_autoindex_network_rules_1)")
	if err != nil {
		t.Fatalf("failed to query index_info for autoindex_1: %v", err)
	}

	foundColumns := 0
	for rows.Next() {
		foundColumns++
	}

	_ = rows.Close()

	if foundColumns < 3 {
		t.Errorf("expected UNIQUE(policy_id, precedence, direction) constraint with 3 columns, found %d", foundColumns)
	}
}

func TestRunMigrations_IncludesNetworkRulesTable(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	if err := runMigrations(ctx, db); err != nil {
		t.Fatalf("runMigrations failed: %v", err)
	}

	tables := allTableNames(t, db)
	tableSet := make(map[string]bool)

	for _, name := range tables {
		tableSet[name] = true
	}

	if !tableSet[NetworkRulesTableName] {
		t.Errorf("expected table %q not found after runMigrations", NetworkRulesTableName)
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

func applyV6(t *testing.T, db *sql.DB) {
	t.Helper()

	ctx := context.Background()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}

	if err := migrateV6(ctx, tx); err != nil {
		_ = tx.Rollback()

		t.Fatalf("migrateV6 failed: %v", err)
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("failed to commit migrateV6: %v", err)
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
