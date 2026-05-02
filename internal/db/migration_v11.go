// Copyright 2026 Ella Networks

package db

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

// V11 retypes every replicated table that previously used
// `INTEGER PRIMARY KEY AUTOINCREMENT` to `TEXT PRIMARY KEY`. Server-side
// AUTOINCREMENT in a replicated table is unsafe: the leader's
// capture/rollback cycle can pick the same id twice across the
// leader-takeover window, which sqlite3changeset_apply later rejects
// with CONFLICT (see spec_uuid.md).
//
// Existing rows get a deterministic UUIDv5 (table-scoped namespace +
// old integer rowid) so every node materialises the same UUID for the
// same row when it runs this migration locally.
//
// Tables with inbound id-FK dependencies are migrated together so the
// FK column is retyped in lockstep.
func migrateV11(ctx context.Context, tx *sql.Tx) error {
	steps := []func(context.Context, *sql.Tx) error{
		migrateV11AuditLogs,
		migrateV11HomeNetworkKeys,
		migrateV11RetentionPolicies,
		migrateV11NetworkRules,
		migrateV11Sessions,
		migrateV11APITokens,
		migrateV11IPLeases,
	}

	for _, step := range steps {
		if err := step(ctx, tx); err != nil {
			return err
		}
	}

	return nil
}

// retypeIDToUUID rebuilds a table whose PK is INTEGER AUTOINCREMENT id
// into a table whose PK is TEXT id. Caller supplies the new schema
// (CREATE TABLE %s_new (...)) and the list of non-id columns to copy.
// Each old row's new id is uuid.NewSHA1(namespace, "table:oldID"), so
// every node lands on the same value.
func retypeIDToUUID(ctx context.Context, tx *sql.Tx, table string, namespace uuid.UUID, newSchema string, columns []string) error {
	if _, err := tx.ExecContext(ctx, fmt.Sprintf(newSchema, table)); err != nil {
		return fmt.Errorf("create %s_new: %w", table, err)
	}

	colList := strings.Join(columns, ", ")

	rows, err := tx.QueryContext(ctx,
		fmt.Sprintf("SELECT id, %s FROM %s ORDER BY id", colList, table))
	if err != nil {
		return fmt.Errorf("select %s: %w", table, err)
	}

	placeholders := strings.Repeat(", ?", len(columns))

	insert, err := tx.PrepareContext(ctx,
		fmt.Sprintf("INSERT INTO %s_new (id, %s) VALUES (?%s)", table, colList, placeholders))
	if err != nil {
		_ = rows.Close()
		return fmt.Errorf("prepare insert for %s: %w", table, err)
	}

	defer func() { _ = insert.Close() }()

	scanArgs := make([]any, len(columns)+1)
	values := make([]any, len(columns)+1)

	var oldID int64

	scanArgs[0] = &oldID

	for i := range columns {
		var v any

		values[i+1] = &v
		scanArgs[i+1] = &v
	}

	for rows.Next() {
		if err := rows.Scan(scanArgs...); err != nil {
			_ = rows.Close()
			return fmt.Errorf("scan %s row: %w", table, err)
		}

		newID := uuid.NewSHA1(namespace, []byte(table+":"+strconv.FormatInt(oldID, 10))).String()

		args := make([]any, len(columns)+1)

		args[0] = newID
		for i := range columns {
			args[i+1] = *(values[i+1].(*any))
		}

		if _, err := insert.ExecContext(ctx, args...); err != nil {
			_ = rows.Close()
			return fmt.Errorf("insert %s_new row (oldID=%d): %w", table, oldID, err)
		}
	}

	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return fmt.Errorf("iterate %s: %w", table, err)
	}

	_ = rows.Close()

	for _, stmt := range []string{
		fmt.Sprintf("DROP TABLE %s", table),
		fmt.Sprintf("ALTER TABLE %s_new RENAME TO %s", table, table),
	} {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("execute %q: %w", stmt, err)
		}
	}

	return nil
}

func migrateV11AuditLogs(ctx context.Context, tx *sql.Tx) error {
	const newSchema = `
		CREATE TABLE %s_new (
			id         TEXT PRIMARY KEY,
			timestamp  TEXT NOT NULL,
			level      TEXT NOT NULL,
			actor      TEXT NOT NULL DEFAULT '',
			action     TEXT NOT NULL,
			ip         TEXT NOT NULL DEFAULT '',
			details    TEXT NOT NULL DEFAULT ''
		)`

	ns := uuid.MustParse("a8f1e7c0-1d3a-4b9e-9c2f-0a4b7e5d1f01")

	return retypeIDToUUID(ctx, tx, AuditLogsTableName, ns, newSchema,
		[]string{"timestamp", "level", "actor", "action", "ip", "details"})
}

func migrateV11HomeNetworkKeys(ctx context.Context, tx *sql.Tx) error {
	const newSchema = `
		CREATE TABLE %s_new (
			id              TEXT PRIMARY KEY,
			key_identifier  INTEGER NOT NULL CHECK (key_identifier >= 0 AND key_identifier <= 255),
			scheme          TEXT    NOT NULL CHECK (scheme IN ('A', 'B')),
			private_key     TEXT    NOT NULL,
			UNIQUE(key_identifier, scheme)
		)`

	ns := uuid.MustParse("3c1b2f9a-7e44-4d0e-9a82-5f6c8e1d7a02")

	return retypeIDToUUID(ctx, tx, HomeNetworkKeysTableName, ns, newSchema,
		[]string{"key_identifier", "scheme", "private_key"})
}

func migrateV11RetentionPolicies(ctx context.Context, tx *sql.Tx) error {
	const newSchema = `
		CREATE TABLE %s_new (
			id              TEXT PRIMARY KEY,
			category        TEXT NOT NULL UNIQUE,
			retention_days  INTEGER NOT NULL CHECK (retention_days >= 1)
		)`

	ns := uuid.MustParse("d4e6c1f2-9b3a-4c5d-8e7f-0a1b2c3d4e02")

	return retypeIDToUUID(ctx, tx, RetentionPolicyTableName, ns, newSchema,
		[]string{"category", "retention_days"})
}

func migrateV11NetworkRules(ctx context.Context, tx *sql.Tx) error {
	const newSchema = `
		CREATE TABLE %s_new (
			id            TEXT PRIMARY KEY,
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
			FOREIGN KEY (policy_id) REFERENCES policies (id) ON DELETE CASCADE,
			UNIQUE(policy_id, precedence, direction)
		)`

	ns := uuid.MustParse("8b2c4d6e-1f3a-4b5c-9d7e-1a2b3c4d5e03")

	return retypeIDToUUID(ctx, tx, NetworkRulesTableName, ns, newSchema,
		[]string{"policy_id", "description", "direction", "remote_prefix", "protocol", "port_low", "port_high", "action", "precedence", "created_at", "updated_at"})
}

func migrateV11Sessions(ctx context.Context, tx *sql.Tx) error {
	const newSchema = `
		CREATE TABLE %s_new (
			id          TEXT PRIMARY KEY,
			user_id     INTEGER NOT NULL,
			token_hash  BLOB    NOT NULL UNIQUE,
			created_at  INTEGER NOT NULL DEFAULT (strftime('%%s','now')),
			expires_at  INTEGER NOT NULL,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`

	ns := uuid.MustParse("f5d8b2a1-7c4e-4f9b-8d6c-2a3b4c5d6e04")

	return retypeIDToUUID(ctx, tx, SessionsTableName, ns, newSchema,
		[]string{"user_id", "token_hash", "created_at", "expires_at"})
}

func migrateV11APITokens(ctx context.Context, tx *sql.Tx) error {
	const newSchema = `
		CREATE TABLE %s_new (
			id          TEXT PRIMARY KEY,
			token_id    TEXT NOT NULL UNIQUE,
			name        TEXT NOT NULL,
			token_hash  TEXT NOT NULL,
			user_id     INTEGER NOT NULL,
			expires_at  DATETIME,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			UNIQUE (name, user_id)
		)`

	ns := uuid.MustParse("c1a3e5f7-9b8d-4e2c-9f1a-3b4c5d6e7f05")

	return retypeIDToUUID(ctx, tx, APITokensTableName, ns, newSchema,
		[]string{"token_id", "name", "token_hash", "user_id", "expires_at"})
}

func migrateV11IPLeases(ctx context.Context, tx *sql.Tx) error {
	const newSchema = `
		CREATE TABLE %s_new (
			id          TEXT PRIMARY KEY,
			poolID      INTEGER NOT NULL REFERENCES data_networks(id) ON DELETE CASCADE,
			addressBin  BLOB    NOT NULL,
			imsi        TEXT    NOT NULL REFERENCES subscribers(imsi) ON DELETE CASCADE,
			sessionID   INTEGER,
			type        TEXT    NOT NULL DEFAULT 'dynamic',
			createdAt   INTEGER NOT NULL,
			nodeID      INTEGER NOT NULL DEFAULT 0,
			UNIQUE(poolID, addressBin)
		)`

	ns := uuid.MustParse("9e7c5b3a-1d2f-4a6b-8c5d-4a5b6c7d8e06")

	if err := retypeIDToUUID(ctx, tx, IPLeasesTableName, ns, newSchema,
		[]string{"poolID", "addressBin", "imsi", "sessionID", "type", "createdAt", "nodeID"}); err != nil {
		return err
	}

	// Indexes were dropped with the old table; recreate.
	for _, stmt := range []string{
		"CREATE INDEX IF NOT EXISTS idx_leases_pool ON " + IPLeasesTableName + "(poolID)",
		"CREATE INDEX IF NOT EXISTS idx_leases_imsi ON " + IPLeasesTableName + "(imsi)",
		"CREATE INDEX IF NOT EXISTS idx_leases_session ON " + IPLeasesTableName + "(sessionID)",
		"CREATE INDEX IF NOT EXISTS idx_leases_node ON " + IPLeasesTableName + "(nodeID)",
	} {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("recreate ip_leases index %q: %w", stmt, err)
		}
	}

	return nil
}
