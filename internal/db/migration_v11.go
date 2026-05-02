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
// old integer id) so every node materialises the same UUID for the
// same row when it runs this migration locally. Integer FK columns are
// rewritten in lockstep using the parent table's namespace, so a child
// row references the same UUID its parent will receive — even on a
// node that has not yet executed the parent migration step.
func migrateV11(ctx context.Context, tx *sql.Tx) error {
	steps := []func(context.Context, *sql.Tx) error{
		// Standalone tables (no FKs to the replicated set).
		migrateV11AuditLogs,
		migrateV11HomeNetworkKeys,
		migrateV11RetentionPolicies,

		// FK-target parents.
		migrateV11DataNetworks,
		migrateV11NetworkSlices,
		migrateV11Profiles,
		migrateV11Users,

		// Tables that depend on the parents above.
		migrateV11Policies,
		migrateV11Subscribers,
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

// fkRef describes how to rewrite an integer FK column into the parent's
// TEXT UUID, computed deterministically from the parent's namespace and
// the original integer FK value.
type fkRef struct {
	ParentTable string
	ParentNS    uuid.UUID
}

// rebuildTableUUID rebuilds `table` into `<table>_new` with the schema
// in `newSchema` (a printf-style template that takes the table name).
// For each old row:
//   - The new id is uuid.NewSHA1(idNamespace, "<table>:<oldID>") so
//     every node lands on the same value.
//   - Each integer FK column listed in `fks` is rewritten with the same
//     scheme using the parent table's namespace. NULL is preserved.
//
// `columns` lists every non-id column in copy order. The new table is
// renamed back to `table`. Caller is responsible for any indexes the
// rebuild dropped.
func rebuildTableUUID(
	ctx context.Context,
	tx *sql.Tx,
	table string,
	newSchema string,
	idNamespace uuid.UUID,
	columns []string,
	fks map[string]fkRef,
) error {
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

		args := make([]any, len(columns)+1)
		args[0] = uuid.NewSHA1(idNamespace, []byte(table+":"+strconv.FormatInt(oldID, 10))).String()

		for i, col := range columns {
			val := *(values[i+1].(*any))

			if fk, ok := fks[col]; ok {
				rewritten, err := rewriteFK(fk, val)
				if err != nil {
					_ = rows.Close()
					return fmt.Errorf("rewrite %s.%s (oldID=%d): %w", table, col, oldID, err)
				}

				args[i+1] = rewritten
			} else {
				args[i+1] = val
			}
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

// rewriteFK turns an integer FK value into the matching TEXT UUID, or
// preserves NULL.
func rewriteFK(fk fkRef, val any) (any, error) {
	if val == nil {
		return nil, nil
	}

	var asInt int64

	switch t := val.(type) {
	case int64:
		asInt = t
	case int:
		asInt = int64(t)
	default:
		return nil, fmt.Errorf("unexpected non-integer FK value %T", val)
	}

	return uuid.NewSHA1(fk.ParentNS, []byte(fk.ParentTable+":"+strconv.FormatInt(asInt, 10))).String(), nil
}

// Namespaces are stable per-table identifiers that combine with the
// row's old integer id to produce the deterministic UUIDv5 used during
// migration. They are also published as the FK reference namespaces
// for any child table whose FK column targets this parent. NEVER edit
// once shipped: every node in a cluster must agree on the value.
var (
	v11NSAuditLogs       = uuid.MustParse("a8f1e7c0-1d3a-4b9e-9c2f-0a4b7e5d1f01")
	v11NSHomeNetworkKeys = uuid.MustParse("3c1b2f9a-7e44-4d0e-9a82-5f6c8e1d7a02")
	v11NSRetentionPolicy = uuid.MustParse("d4e6c1f2-9b3a-4c5d-8e7f-0a1b2c3d4e02")
	v11NSNetworkRules    = uuid.MustParse("8b2c4d6e-1f3a-4b5c-9d7e-1a2b3c4d5e03")
	v11NSSessions        = uuid.MustParse("f5d8b2a1-7c4e-4f9b-8d6c-2a3b4c5d6e04")
	v11NSAPITokens       = uuid.MustParse("c1a3e5f7-9b8d-4e2c-9f1a-3b4c5d6e7f05")
	v11NSIPLeases        = uuid.MustParse("9e7c5b3a-1d2f-4a6b-8c5d-4a5b6c7d8e06")
	v11NSDataNetworks    = uuid.MustParse("4f7c1d8a-2b9e-4a1c-8e6d-5b2c3a4d5e07")
	v11NSNetworkSlices   = uuid.MustParse("a3d2c1b4-5e6f-4a7b-9c8d-1e2f3a4b5c08")
	v11NSProfiles        = uuid.MustParse("7e6d5c4b-3a2b-4c5d-9e6f-7a8b9c0d1e09")
	v11NSUsers           = uuid.MustParse("b8a7c6d5-4e3f-4a2b-9c1d-2e3f4a5b6c0a")
	v11NSPolicies        = uuid.MustParse("c9b8a7d6-5e4f-4a3b-8c2d-3e4f5a6b7c0b")
	v11NSSubscribers     = uuid.MustParse("d0c9b8a7-6f5e-4a3b-8c2d-4e5f6a7b8c0c")
)

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

	return rebuildTableUUID(ctx, tx, AuditLogsTableName, newSchema, v11NSAuditLogs,
		[]string{"timestamp", "level", "actor", "action", "ip", "details"}, nil)
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

	return rebuildTableUUID(ctx, tx, HomeNetworkKeysTableName, newSchema, v11NSHomeNetworkKeys,
		[]string{"key_identifier", "scheme", "private_key"}, nil)
}

func migrateV11RetentionPolicies(ctx context.Context, tx *sql.Tx) error {
	const newSchema = `
		CREATE TABLE %s_new (
			id              TEXT PRIMARY KEY,
			category        TEXT NOT NULL UNIQUE,
			retention_days  INTEGER NOT NULL CHECK (retention_days >= 1)
		)`

	return rebuildTableUUID(ctx, tx, RetentionPolicyTableName, newSchema, v11NSRetentionPolicy,
		[]string{"category", "retention_days"}, nil)
}

func migrateV11DataNetworks(ctx context.Context, tx *sql.Tx) error {
	const newSchema = `
		CREATE TABLE %s_new (
			id     TEXT PRIMARY KEY,
			name   TEXT NOT NULL UNIQUE,
			ipPool TEXT NOT NULL,
			dns    TEXT NOT NULL,
			mtu    INTEGER NOT NULL
		)`

	return rebuildTableUUID(ctx, tx, DataNetworksTableName, newSchema, v11NSDataNetworks,
		[]string{"name", "ipPool", "dns", "mtu"}, nil)
}

func migrateV11NetworkSlices(ctx context.Context, tx *sql.Tx) error {
	const newSchema = `
		CREATE TABLE %s_new (
			id   TEXT PRIMARY KEY,
			sst  INTEGER NOT NULL,
			sd   TEXT,
			name TEXT NOT NULL UNIQUE,
			UNIQUE(sst, sd)
		)`

	return rebuildTableUUID(ctx, tx, NetworkSlicesTableName, newSchema, v11NSNetworkSlices,
		[]string{"sst", "sd", "name"}, nil)
}

func migrateV11Profiles(ctx context.Context, tx *sql.Tx) error {
	const newSchema = `
		CREATE TABLE %s_new (
			id             TEXT PRIMARY KEY,
			name           TEXT NOT NULL UNIQUE,
			ueAmbrUplink   TEXT NOT NULL,
			ueAmbrDownlink TEXT NOT NULL
		)`

	return rebuildTableUUID(ctx, tx, ProfilesTableName, newSchema, v11NSProfiles,
		[]string{"name", "ueAmbrUplink", "ueAmbrDownlink"}, nil)
}

func migrateV11Users(ctx context.Context, tx *sql.Tx) error {
	const newSchema = `
		CREATE TABLE %s_new (
			id             TEXT PRIMARY KEY,
			email          TEXT NOT NULL UNIQUE,
			roleID         INTEGER NOT NULL,
			hashedPassword TEXT NOT NULL
		)`

	return rebuildTableUUID(ctx, tx, UsersTableName, newSchema, v11NSUsers,
		[]string{"email", "roleID", "hashedPassword"}, nil)
}

func migrateV11Policies(ctx context.Context, tx *sql.Tx) error {
	const newSchema = `
		CREATE TABLE %s_new (
			id                  TEXT PRIMARY KEY,
			name                TEXT NOT NULL UNIQUE,
			profileID           TEXT NOT NULL,
			sliceID             TEXT NOT NULL,
			dataNetworkID       TEXT NOT NULL,
			var5qi              INTEGER NOT NULL,
			arp                 INTEGER NOT NULL,
			sessionAmbrUplink   TEXT    NOT NULL,
			sessionAmbrDownlink TEXT    NOT NULL,
			FOREIGN KEY (profileID)     REFERENCES profiles (id) ON DELETE RESTRICT,
			FOREIGN KEY (sliceID)       REFERENCES network_slices (id) ON DELETE RESTRICT,
			FOREIGN KEY (dataNetworkID) REFERENCES data_networks (id) ON DELETE RESTRICT,
			UNIQUE(profileID, sliceID, dataNetworkID)
		)`

	return rebuildTableUUID(ctx, tx, PoliciesTableName, newSchema, v11NSPolicies,
		[]string{"name", "profileID", "sliceID", "dataNetworkID", "var5qi", "arp", "sessionAmbrUplink", "sessionAmbrDownlink"},
		map[string]fkRef{
			"profileID":     {ParentTable: ProfilesTableName, ParentNS: v11NSProfiles},
			"sliceID":       {ParentTable: NetworkSlicesTableName, ParentNS: v11NSNetworkSlices},
			"dataNetworkID": {ParentTable: DataNetworksTableName, ParentNS: v11NSDataNetworks},
		})
}

func migrateV11Subscribers(ctx context.Context, tx *sql.Tx) error {
	const newSchema = `
		CREATE TABLE %s_new (
			id             TEXT PRIMARY KEY,
			imsi           TEXT NOT NULL UNIQUE CHECK (length(imsi) BETWEEN 6 AND 15 AND imsi GLOB '[0-9]*'),
			sequenceNumber TEXT NOT NULL CHECK (length(sequenceNumber) = 12),
			permanentKey   TEXT NOT NULL CHECK (length(permanentKey) = 32),
			opc            TEXT NOT NULL CHECK (length(opc) = 32),
			profileID      TEXT NOT NULL,
			FOREIGN KEY (profileID) REFERENCES profiles (id) ON DELETE RESTRICT
		)`

	return rebuildTableUUID(ctx, tx, SubscribersTableName, newSchema, v11NSSubscribers,
		[]string{"imsi", "sequenceNumber", "permanentKey", "opc", "profileID"},
		map[string]fkRef{
			"profileID": {ParentTable: ProfilesTableName, ParentNS: v11NSProfiles},
		})
}

func migrateV11NetworkRules(ctx context.Context, tx *sql.Tx) error {
	const newSchema = `
		CREATE TABLE %s_new (
			id            TEXT PRIMARY KEY,
			policy_id     TEXT NOT NULL,
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

	return rebuildTableUUID(ctx, tx, NetworkRulesTableName, newSchema, v11NSNetworkRules,
		[]string{"policy_id", "description", "direction", "remote_prefix", "protocol", "port_low", "port_high", "action", "precedence", "created_at", "updated_at"},
		map[string]fkRef{
			"policy_id": {ParentTable: PoliciesTableName, ParentNS: v11NSPolicies},
		})
}

func migrateV11Sessions(ctx context.Context, tx *sql.Tx) error {
	const newSchema = `
		CREATE TABLE %s_new (
			id          TEXT PRIMARY KEY,
			user_id     TEXT NOT NULL,
			token_hash  BLOB    NOT NULL UNIQUE,
			created_at  INTEGER NOT NULL DEFAULT (strftime('%%s','now')),
			expires_at  INTEGER NOT NULL,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`

	return rebuildTableUUID(ctx, tx, SessionsTableName, newSchema, v11NSSessions,
		[]string{"user_id", "token_hash", "created_at", "expires_at"},
		map[string]fkRef{
			"user_id": {ParentTable: UsersTableName, ParentNS: v11NSUsers},
		})
}

func migrateV11APITokens(ctx context.Context, tx *sql.Tx) error {
	const newSchema = `
		CREATE TABLE %s_new (
			id          TEXT PRIMARY KEY,
			token_id    TEXT NOT NULL UNIQUE,
			name        TEXT NOT NULL,
			token_hash  TEXT NOT NULL,
			user_id     TEXT NOT NULL,
			expires_at  DATETIME,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			UNIQUE (name, user_id)
		)`

	return rebuildTableUUID(ctx, tx, APITokensTableName, newSchema, v11NSAPITokens,
		[]string{"token_id", "name", "token_hash", "user_id", "expires_at"},
		map[string]fkRef{
			"user_id": {ParentTable: UsersTableName, ParentNS: v11NSUsers},
		})
}

func migrateV11IPLeases(ctx context.Context, tx *sql.Tx) error {
	const newSchema = `
		CREATE TABLE %s_new (
			id          TEXT PRIMARY KEY,
			poolID      TEXT NOT NULL REFERENCES data_networks(id) ON DELETE CASCADE,
			addressBin  BLOB    NOT NULL,
			imsi        TEXT    NOT NULL REFERENCES subscribers(imsi) ON DELETE CASCADE,
			sessionID   INTEGER,
			type        TEXT    NOT NULL DEFAULT 'dynamic',
			createdAt   INTEGER NOT NULL,
			nodeID      INTEGER NOT NULL DEFAULT 0,
			UNIQUE(poolID, addressBin)
		)`

	if err := rebuildTableUUID(ctx, tx, IPLeasesTableName, newSchema, v11NSIPLeases,
		[]string{"poolID", "addressBin", "imsi", "sessionID", "type", "createdAt", "nodeID"},
		map[string]fkRef{
			"poolID": {ParentTable: DataNetworksTableName, ParentNS: v11NSDataNetworks},
		}); err != nil {
		return err
	}

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
