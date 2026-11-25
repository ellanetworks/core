// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	hraft "github.com/hashicorp/raft"
	"go.uber.org/zap"
)

// migration represents a single schema migration step.
type migration struct {
	version     int
	description string
	fn          func(ctx context.Context, tx *sql.Tx) error
}

// migrations is the append-only schema migration registry. Versions are
// sequential from 1 with no gaps; shipped entries are immutable.
var migrations = []migration{
	{1, "baseline schema", migrateV1},
	{2, "add NAS security columns, home network keys table, and SPN columns", migrateV2},
	{3, "add radio_name column to network_logs", migrateV3},
	{4, "add bgp_settings, bgp_peers, jwt_secret, ip_leases tables; drop ipAddress from subscribers", migrateV4},
	{5, "add network_rules and policy_network_rules tables", migrateV5},
	{6, "replace address TEXT with addressBin BLOB in ip_leases", migrateV6},
	{7, "data model redesign: profiles, policies, slices", migrateV7},
	{8, "add action to flow reports", migrateV8},
	{9, "HA schema additions (amfRegionID, cluster_members, ip_leases.nodeID, bgp_peers.nodeID)", migrateV9},
	{10, "drop bgp_peers.nodeID and cluster_members.maxSchemaVersion (both dead post-HA-redesign)", migrateV10},
	{11, "replicated AUTOINCREMENT PKs → TEXT (UUID); spec_uuid.md", migrateV11},
	{12, "replace chain-PKI cluster TLS with fingerprint pinning (cluster_node_certs)", migrateV12},
	{13, "add ipv6Pool column to data_networks and poolType column to ip_leases", migrateV13},
	{14, "add 4G config: profile allowed access (4G/5G), policy default binding; RAT-neutral NAS algorithm names", migrateV14},
	{15, "add positioning_sessions and cell_positions tables for LMF", migrateV15},
	{16, "add subscriber_framed_routes table", migrateV16},
	{17, "add local_switch_settings table", migrateV17},
	{18, "add description column to subscribers", migrateV18},
	{19, "store audit log, radio event and flow report timestamps as INTEGER epoch milliseconds (not seconds, unlike every other INTEGER time column)", migrateV19},
	{20, "retype node identity to TEXT; add cluster_members.amfPointer and displayName; drop raftAddress and suffrage (owned by the Raft configuration)", migrateV20},
	{21, "add ue_registrations table (HSS MME identity, UDM AMF registration)", migrateV21},
	{22, "add SMS: subscribers.msisdn column, sms_settings, SMSC peer and message waiting tables", migrateV22},
	{23, "add pcscf_addresses and ims_registrations tables: P-CSCF addresses given to phones and IMS S-CSCF assignments", migrateV23},
	{24, "replace positioning_sessions with deferred_location_requests and location_reports for periodic location", migrateV24},
	{25, "add IMS services: data_networks.ims, policies pre-emption capability and vulnerability, IMS application server, PSI, service profile, iFC and service profile assignment tables, IMS registrations per implicit registration set", migrateV25},
	{26, "index daily_usage by imsi and ue_registrations by version, so deleting a subscriber and allocating the next registration version do not scan every row; group SMSC peers into SMSCs with several nodes; add slice data networks, carrying NAT (per node) and local switching from the node-wide settings", migrateV26},
	{27, "add local-breakout roaming: roaming_partners and their roaming_partner_plmns, visitors (inbound roamers, apart from subscribers), roaming diameter peers, ue_registrations, ip_leases and flow_reports keyed by IMSI without a subscriber FK, daily_usage keyed by IMSI with its home PLMN (no subscriber FK, so no idx_daily_usage_imsi), sepp_ca and per-node sepp_identities; add ims_settings (configurable IMS home domain) and ims_peers (allowlist of inbound IMS Diameter peers)", migrateV27},
	{28, "add radio_ref column to network_logs: radio events keyed by the Global eNB / RAN Node ID, not the optional, mutable name", migrateV28},
	{29, "add single sign-on: user_identities (accounts at the identity provider), sessions.method and identity_id, oidc_settings, oidc_group_roles and oidc_used_states", migrateV29},
	{30, "add radio_event_settings: whether radio events are stored", migrateV30},
	{31, "replicate commands instead of changesets: log replicated rows that break a foreign key, since every node now applies writes with foreign keys on; add subscriber_count, the number of subscribers and visitors, kept by the writes that add or remove them; index ip_leases by pool and creation for the next free address search", migrateV31},
}

// baselineVersion is the highest migration that runs locally during
// cluster-mode startup; the leader proposes the rest through Raft.
// Anything touched before the cluster exists must sit at or below it: the
// pin table the mTLS listener verifies against, the member row written
// while forming, and the leader's Initialize() seed.
const baselineVersion = commandSchema

// SchemaVersion returns the highest migration version this binary understands.
// Used during cluster join to reject version-skewed nodes.
func SchemaVersion() int {
	return migrations[len(migrations)-1].version
}

// runMigrations brings the database up to the given maxVersion. Pass 0 for
// the latest available version. In cluster mode, callers pass baselineVersion
// so only the baseline runs locally; post-baseline migrations are proposed
// through Raft by the leader.
func runMigrations(ctx context.Context, sqlConn *sql.DB, maxVersion int) error {
	for i, m := range migrations {
		if m.version != i+1 {
			return fmt.Errorf("migration registry error: expected version %d at index %d, got %d", i+1, i, m.version)
		}

		if m.fn == nil {
			return fmt.Errorf("migration registry error: migration %d has nil function", m.version)
		}
	}

	if _, err := sqlConn.ExecContext(ctx,
		`CREATE TABLE IF NOT EXISTS schema_version (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			version INTEGER NOT NULL
		)`); err != nil {
		return fmt.Errorf("failed to create schema_version table: %w", err)
	}

	if _, err := sqlConn.ExecContext(ctx,
		"INSERT OR IGNORE INTO schema_version (id, version) VALUES (1, 0)"); err != nil {
		return fmt.Errorf("failed to seed schema_version: %w", err)
	}

	var current int

	if err := sqlConn.QueryRowContext(ctx,
		"SELECT version FROM schema_version WHERE id = 1").Scan(&current); err != nil {
		return fmt.Errorf("failed to read schema version: %w", err)
	}

	if current > SchemaVersion() {
		return fmt.Errorf("%w: database schema is v%d, this binary supports up to v%d; downgrades are not supported, restore a backup taken on this version",
			ErrSchemaNewerThanBinary, current, SchemaVersion())
	}

	from := current

	for _, m := range migrations {
		if m.version <= current {
			continue
		}

		if maxVersion > 0 && m.version > maxVersion {
			break
		}

		if err := applyLocalMigration(ctx, sqlConn, m); err != nil {
			return err
		}

		current = m.version
	}

	if current > from {
		logger.DBLog.Info("Database schema migrated",
			zap.Int("from_version", from),
			zap.Int("to_version", current),
		)
	}

	return nil
}

// applyLocalMigration runs a single migration inside a transaction with FK
// enforcement disabled. PRAGMA foreign_keys is a no-op inside a transaction,
// so it is disabled on the connection before the migration tx begins — this
// prevents DROP TABLE from cascade-deleting child rows during table rebuilds.
// FK is re-enabled unconditionally via defer.
func applyLocalMigration(ctx context.Context, sqlConn *sql.DB, m migration) error {
	logger.DBLog.Debug("Applying migration",
		zap.Int("version", m.version),
		zap.String("description", m.description),
	)

	if _, err := sqlConn.ExecContext(ctx, "PRAGMA foreign_keys = OFF"); err != nil {
		return fmt.Errorf("failed to disable foreign keys for migration %d: %w", m.version, err)
	}

	defer func() {
		_, _ = sqlConn.ExecContext(context.Background(), "PRAGMA foreign_keys = ON")
	}()

	tx, err := sqlConn.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction for migration %d: %w", m.version, err)
	}

	if err := m.fn(ctx, tx); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("migration %d (%s) failed: %w", m.version, m.description, err)
	}

	if _, err := tx.ExecContext(ctx,
		"UPDATE schema_version SET version = ? WHERE id = 1", m.version); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("failed to update schema_version to %d: %w", m.version, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit migration %d: %w", m.version, err)
	}

	logger.DBLog.Debug("Migration applied successfully",
		zap.Int("version", m.version),
		zap.String("description", m.description),
	)

	return nil
}

var ErrUpgradeFromBeforeCommands = errors.New("this cluster member last ran a version before Ella Core v1.23; upgrade every cluster member to v1.23 before upgrading to this version")

func checkClusterUpgradePath(ctx context.Context, conn *sql.DB) error {
	schema, err := readIfTableExists(ctx, conn, "schema_version", "SELECT version FROM schema_version WHERE id = 1")
	if err != nil || schema == 0 || schema >= commandSchema {
		return err
	}

	lastApplied, err := readIfTableExists(ctx, conn, FsmStateTableName, "SELECT lastApplied FROM "+FsmStateTableName+" WHERE id = 1")
	if err != nil || lastApplied == 0 {
		return err
	}

	servers, err := storedRaftServers(ctx, conn)
	if err != nil || servers <= 1 {
		return err
	}

	return fmt.Errorf("%w (database schema %d, want %d or later)", ErrUpgradeFromBeforeCommands, schema, commandSchema)
}

func storedRaftServers(ctx context.Context, conn *sql.DB) (n int, err error) {
	var hasColumn bool
	if err := conn.QueryRowContext(ctx, "SELECT EXISTS (SELECT 1 FROM pragma_table_info('"+FsmStateTableName+"') WHERE name = 'configuration')").Scan(&hasColumn); err != nil {
		return 0, fmt.Errorf("look up %s.configuration: %w", FsmStateTableName, err)
	}

	if !hasColumn {
		return 0, nil
	}

	var blob []byte
	if err := conn.QueryRowContext(ctx, "SELECT configuration FROM "+FsmStateTableName+" WHERE id = 1").Scan(&blob); err != nil {
		return 0, fmt.Errorf("read %s.configuration: %w", FsmStateTableName, err)
	}

	if len(blob) == 0 {
		return 0, nil
	}

	defer func() {
		if r := recover(); r != nil {
			n, err = 0, fmt.Errorf("decode %s.configuration: %v", FsmStateTableName, r)
		}
	}()

	return len(hraft.DecodeConfiguration(blob).Servers), nil
}

func readIfTableExists(ctx context.Context, conn *sql.DB, table, query string) (int64, error) {
	var exists bool
	if err := conn.QueryRowContext(ctx, "SELECT EXISTS (SELECT 1 FROM sqlite_master WHERE type = 'table' AND name = ?)", table).Scan(&exists); err != nil {
		return 0, fmt.Errorf("look up table %s: %w", table, err)
	}

	if !exists {
		return 0, nil
	}

	var v int64
	if err := conn.QueryRowContext(ctx, query).Scan(&v); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, nil
		}

		return 0, fmt.Errorf("read %s: %w", table, err)
	}

	return v, nil
}
