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
