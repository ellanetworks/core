// Copyright 2026 Ella Networks

package db

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"

	"github.com/google/uuid"
)

// auditLogIDNamespace seeds the deterministic UUID assigned to each
// pre-v11 audit_log row. The choice of namespace is arbitrary; what
// matters is that every node uses the same value so SHA1-based UUIDs
// computed from (namespace, old_rowid) match across replicas.
//
// Treat as a release-stable constant: changing it would break audit-log
// id stability across the migration.
var auditLogIDNamespace = uuid.MustParse("a8f1e7c0-1d3a-4b9e-9c2f-0a4b7e5d1f01")

// V11 retypes audit_logs.id from INTEGER AUTOINCREMENT to TEXT (UUID).
// Server-assigned AUTOINCREMENT in a replicated table races on leader
// takeover (capture picks an id from sqlite_sequence, which a pending
// previous-term entry has already claimed). UUIDs generated at the
// request handler eliminate the class.
//
// Existing rows get a deterministic UUIDv5 derived from their old id so
// every node ends up with identical bytes.
func migrateV11(ctx context.Context, tx *sql.Tx) error {
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

	if _, err := tx.ExecContext(ctx, fmt.Sprintf(newSchema, AuditLogsTableName)); err != nil {
		return fmt.Errorf("create audit_logs_new: %w", err)
	}

	rows, err := tx.QueryContext(ctx,
		fmt.Sprintf("SELECT id, timestamp, level, actor, action, ip, details FROM %s ORDER BY id", AuditLogsTableName))
	if err != nil {
		return fmt.Errorf("select audit_logs: %w", err)
	}

	insert, err := tx.PrepareContext(ctx,
		fmt.Sprintf("INSERT INTO %s_new (id, timestamp, level, actor, action, ip, details) VALUES (?, ?, ?, ?, ?, ?, ?)", AuditLogsTableName))
	if err != nil {
		_ = rows.Close()
		return fmt.Errorf("prepare insert: %w", err)
	}

	defer func() { _ = insert.Close() }()

	for rows.Next() {
		var (
			oldID                                        int64
			timestamp, level, actor, action, ip, details string
		)

		if err := rows.Scan(&oldID, &timestamp, &level, &actor, &action, &ip, &details); err != nil {
			_ = rows.Close()
			return fmt.Errorf("scan audit_logs row: %w", err)
		}

		newID := uuid.NewSHA1(auditLogIDNamespace, []byte("audit_logs:"+strconv.FormatInt(oldID, 10))).String()

		if _, err := insert.ExecContext(ctx, newID, timestamp, level, actor, action, ip, details); err != nil {
			_ = rows.Close()
			return fmt.Errorf("insert audit_logs_new row (oldID=%d): %w", oldID, err)
		}
	}

	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return fmt.Errorf("iterate audit_logs: %w", err)
	}

	_ = rows.Close()

	for _, stmt := range []string{
		fmt.Sprintf("DROP TABLE %s", AuditLogsTableName),
		fmt.Sprintf("ALTER TABLE %s_new RENAME TO %s", AuditLogsTableName, AuditLogsTableName),
	} {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("execute %q: %w", stmt, err)
		}
	}

	return nil
}
