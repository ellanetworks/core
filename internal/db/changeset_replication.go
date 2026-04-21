// Copyright 2026 Ella Networks

// Changeset replication: captures SQLite session changesets on the leader,
// replicates them through the Raft log, and applies them on all nodes.
//
// Write path (leader only — proposeChangeset / captureChangeset):
//   1. Obtain a pinned *sql.Conn (bypasses sqlair's pool) and start a
//      sqlite3_session on the replicated tables.
//   2. Execute the mutating SQL inside a transaction.
//   3. Capture the changeset bytes from the session, then ROLLBACK the
//      transaction — the local database is NOT yet modified.
//   4. Propose the changeset through Raft (raft.Apply).
//   5. Raft commits the log entry; the FSM calls applyChangeset on
//      every node (including the leader) to replay the changeset.
//
// This capture→rollback→replicate→apply pattern doubles the write cost
// on the leader but guarantees that every node applies the identical
// changeset bytes, keeping SQLite state deterministic across the cluster.
//
// Apply path (all nodes — applyChangeset):
//   Foreign keys are disabled during changeset application because
//   sqlite3session applies rows in an arbitrary order that may
//   temporarily violate FK constraints. FK integrity is enforced at
//   capture time on the leader.

package db

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"

	"github.com/canonical/sqlair"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/mattn/go-sqlite3"
	"go.uber.org/zap"
)

var replicatedChangesetTables = []string{
	SubscribersTableName,
	PoliciesTableName,
	ProfilesTableName,
	DataNetworksTableName,
	NetworkSlicesTableName,
	NetworkRulesTableName,
	IPLeasesTableName,
	AuditLogsTableName,
	UsersTableName,
	APITokensTableName,
	SessionsTableName,
	HomeNetworkKeysTableName,
	BGPPeersTableName,
	BGPSettingsTableName,
	BGPImportPrefixesTableName,
	NATSettingsTableName,
	N3SettingsTableName,
	FlowAccountingSettingsTableName,
	RetentionPolicyTableName,
	OperatorTableName,
	JWTSecretTableName,
	RoutesTableName,
	ClusterMembersTableName,
	ClusterPKIRootsTableName,
	ClusterPKIIntermediatesTableName,
	ClusterIssuedCertsTableName,
	ClusterRevokedCertsTableName,
	ClusterJoinTokensTableName,
	ClusterPKIStateTableName,
	DailyUsageTableName,
	"schema_version",
}

var localOnlyTables = []string{
	RadioEventsTableName,
	FlowReportsTableName,
	FsmStateTableName,
}

func (db *Database) assertTableReplicationClassification(ctx context.Context) error {
	rows, err := db.conn().PlainDB().QueryContext(ctx,
		"SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'")
	if err != nil {
		return fmt.Errorf("list sqlite tables: %w", err)
	}

	defer func() { _ = rows.Close() }()

	class := make(map[string]struct{}, len(replicatedChangesetTables)+len(localOnlyTables))
	for _, t := range replicatedChangesetTables {
		class[t] = struct{}{}
	}

	for _, t := range localOnlyTables {
		class[t] = struct{}{}
	}

	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			return fmt.Errorf("scan sqlite table name: %w", err)
		}

		if _, ok := class[table]; ok {
			continue
		}

		return fmt.Errorf("table %q is not classified as replicated or local-only", table)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate sqlite table names: %w", err)
	}

	return nil
}

func (db *Database) applyChangeset(ctx context.Context, payload *bytesPayload) (any, error) {
	conn, err := db.conn().PlainDB().Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquire sqlite conn for apply: %w", err)
	}

	defer func() { _ = conn.Close() }()

	if err := conn.Raw(func(raw any) error {
		sqliteConn, ok := raw.(*sqlite3.SQLiteConn)
		if !ok {
			return fmt.Errorf("unexpected sqlite driver conn type %T", raw)
		}

		if _, err := sqliteConn.ExecContext(ctx, "PRAGMA foreign_keys = OFF", nil); err != nil {
			return fmt.Errorf("disable foreign keys before changeset apply: %w", err)
		}

		defer func() {
			_, _ = sqliteConn.ExecContext(context.Background(), "PRAGMA foreign_keys = ON", nil)
		}()

		if err := sqliteConn.ApplyChangeset(ctx, payload.Value); err != nil {
			return fmt.Errorf("apply sqlite changeset: %w", err)
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return nil, nil
}

func (db *Database) captureChangeset(ctx context.Context, applyFn func(context.Context) (any, error), operation string) ([]byte, any, error) {
	conn, err := db.conn().PlainDB().Conn(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("acquire sqlite conn for capture: %w", err)
	}

	defer func() { _ = conn.Close() }()

	var changeset []byte

	var result any

	err = conn.Raw(func(raw any) error {
		sqliteConn, ok := raw.(*sqlite3.SQLiteConn)
		if !ok {
			return fmt.Errorf("unexpected sqlite driver conn type %T", raw)
		}

		if _, err := sqliteConn.ExecContext(ctx, "BEGIN IMMEDIATE", nil); err != nil {
			return fmt.Errorf("begin changeset capture transaction: %w", err)
		}

		// Capture always rolls back: we replicate the changeset bytes
		// and let applyChangeset re-execute them on every node. Use
		// context.Background so a cancelled caller context doesn't
		// leave the connection mid-transaction.
		defer func() {
			_, _ = sqliteConn.ExecContext(context.Background(), "ROLLBACK", nil)
		}()

		changeset, err = sqliteConn.CaptureChangeset(ctx, func() error {
			dconn, ok := raw.(driver.Conn)
			if !ok {
				return fmt.Errorf("raw sqlite conn does not implement driver.Conn")
			}

			applyResult, applyErr := db.applyWithPinnedConn(ctx, dconn, applyFn)
			if applyErr != nil {
				return applyErr
			}

			result = applyResult

			return nil
		}, replicatedChangesetTables)
		if err != nil {
			return fmt.Errorf("capture sqlite changeset: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	logger.WithTrace(ctx, logger.DBLog).Debug("captured changeset",
		zap.String("operation", operation),
		zap.Int("bytes", len(changeset)))

	return changeset, result, nil
}

// pinnedRunnerCtxKey is the context key under which applyWithPinnedConn stores
// the pinned *sqlair.DB for apply functions. Applier methods resolve the
// runner via (*Database).runner so they target the pinned connection during
// capture without mutating the shared db.conn pointer (which would race with
// concurrent non-capture DB users and return "sql: database is closed" once
// the pinned handle is Close'd at capture end).
type pinnedRunnerCtxKey struct{}

// runner returns the sqlair DB to use for the current operation. Inside a
// changeset capture, applyFn receives a context carrying a pinned *sqlair.DB;
// outside of capture, callers get the shared db.conn().
func (db *Database) runner(ctx context.Context) *sqlair.DB {
	if r, ok := ctx.Value(pinnedRunnerCtxKey{}).(*sqlair.DB); ok {
		return r
	}

	return db.conn()
}

func (db *Database) applyWithPinnedConn(ctx context.Context, conn driver.Conn, applyFn func(context.Context) (any, error)) (any, error) {
	// The caller (proposeChangeset) holds db.proposeMu, so concurrent
	// captures are serialised at the propose level — no additional mutex
	// is needed here.
	pinned := sql.OpenDB(&pinnedConnector{conn: conn})
	pinned.SetMaxOpenConns(1)
	pinned.SetMaxIdleConns(1)

	defer func() { _ = pinned.Close() }()

	pinnedSQLAir := sqlair.NewDB(pinned)

	ctx = context.WithValue(ctx, pinnedRunnerCtxKey{}, pinnedSQLAir)

	return applyFn(ctx)
}

type pinnedConnector struct {
	conn driver.Conn
}

func (c *pinnedConnector) Connect(context.Context) (driver.Conn, error) {
	return &noCloseConn{Conn: c.conn}, nil
}

func (c *pinnedConnector) Driver() driver.Driver {
	return pinnedDriver{}
}

type pinnedDriver struct{}

func (p pinnedDriver) Open(string) (driver.Conn, error) {
	return nil, fmt.Errorf("pinned driver does not support Open")
}

type noCloseConn struct {
	driver.Conn
}

func (c *noCloseConn) Close() error {
	return nil
}
