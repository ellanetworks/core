// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
)

var replicatedTables = []string{
	SubscribersTableName,
	PoliciesTableName,
	ProfilesTableName,
	DataNetworksTableName,
	SliceDataNetworksTableName,
	NetworkSlicesTableName,
	NetworkRulesTableName,
	FramedRoutesTableName,
	IPLeasesTableName,
	UsersTableName,
	APITokensTableName,
	SessionsTableName,
	HomeNetworkKeysTableName,
	RetentionPolicyTableName,
	OperatorTableName,
	JWTSecretTableName,
	ClusterMembersTableName,
	ClusterNodeCertsTableName,
	ClusterJoinTokensTableName,
	ClusterJoinHMACTableName,
	DailyUsageTableName,
	CellPositionsTableName,
	UERegistrationsTableName,
	SMSSettingsTableName,
	SMSCsTableName,
	DiameterPeersTableName,
	SMSCServiceCentresTableName,
	SMSWaitingTableName,
	SMSWaitingCentresTableName,
	PCSCFAddressesTableName,
	IMSSettingsTableName,
	IMSPeersTableName,
	IMSRegistrationsTableName,
	IMSRegistrationSetsTableName,
	IPSMGWRegistrationsTableName,
	IMSApplicationServersTableName,
	IMSPublicServiceIdentitiesTableName,
	IMSServiceProfilesTableName,
	IMSInitialFilterCriteriaTableName,
	IMSServiceProfileAssignmentsTableName,
	RoamingPartnersTableName,
	RoamingPartnerPLMNsTableName,
	VisitorsTableName,
	SEPPCATableName,
	SEPPIdentitiesTableName,
	UserIdentitiesTableName,
	OIDCSettingsTableName,
	OIDCGroupRolesTableName,
	OIDCUsedStatesTableName,
	RadioEventSettingsTableName,
	SubscriberCountTableName,
	"schema_version",
}

// localOnlyTables are NOT replicated through Raft. Each node owns its own
// copy: writes are applied directly to the local SQLite, never proposed
// through the Raft log; rows are preserved across full DB swaps via
// RestoreLocalTables.
//
// SEED CONTRACT for singleton local-only tables: every node must seed the
// singleton row at startup with documented defaults so readers always see
// a row. Seeds run from db.InitializeLocalSettings (called by NewDatabase
// on every node — leader, follower, standalone). The seed must be
// idempotent: an existing row, whether default or operator-set, is left
// untouched so a daemon restart never overwrites operator state.
//
// When adding a new singleton local-only table, register its Initialize*
// in db.InitializeLocalSettings AND extend the fresh-DB regression test
// in local_only_defaults_test.go.
var localOnlyTables = []string{
	AuditLogsTableName,
	RadioEventsTableName,
	FlowReportsTableName,
	RoutesTableName,
	BGPSettingsTableName,
	BGPPeersTableName,
	BGPImportPrefixesTableName,
	N3SettingsTableName,
	NATSettingsTableName,
	FlowAccountingSettingsTableName,
	LocalSwitchSettingsTableName,
	"positioning_sessions",
	DeferredLocationRequestsTableName,
	LocationReportsTableName,
}

var retiredReplicatedTables = []string{
	AuditLogsTableName,
	RoutesTableName,
	BGPSettingsTableName,
	BGPPeersTableName,
	BGPImportPrefixesTableName,
	N3SettingsTableName,
	NATSettingsTableName,
	FlowAccountingSettingsTableName,

	"cluster_pki_roots",
	"cluster_pki_intermediates",
	"cluster_issued_certs",
	"cluster_revoked_certs",
	"cluster_pki_state",
}

var replicatedTableSet = func() map[string]struct{} {
	set := make(map[string]struct{}, len(replicatedTables))
	for _, t := range replicatedTables {
		set[t] = struct{}{}
	}

	return set
}()

func isReplicatedTable(table string) bool {
	_, ok := replicatedTableSet[table]

	return ok
}

// fsmInternalTables are managed directly by the FSM layer, not through
// replication or local-only backup/restore. The fsm_state table
// tracks the Raft index of the last applied log entry. Its value must
// always match the database content:
//
//   - In a snapshot, fsm_state.lastApplied reflects the last log entry
//     applied before the snapshot was taken.
//   - During FSM.Restore, fsm_state must NOT be overwritten from the
//     pre-restore database (as localOnlyTables are). Preserving a stale
//     lastApplied causes the FSM to skip replaying log entries that
//     follow the snapshot, silently losing committed mutations.
//
// The table is created/seeded by ensureFsmStateTable on every startup and
// after every Reopen.
var fsmInternalTables = []string{
	FsmStateTableName,
}

func (db *Database) assertTableReplicationClassification(ctx context.Context) error {
	rows, err := db.writer().QueryContext(ctx,
		"SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'")
	if err != nil {
		return fmt.Errorf("list sqlite tables: %w", err)
	}

	defer func() { _ = rows.Close() }()

	class := make(map[string]struct{}, len(replicatedTables)+len(localOnlyTables)+len(fsmInternalTables))
	for _, t := range replicatedTables {
		class[t] = struct{}{}
	}

	for _, t := range localOnlyTables {
		class[t] = struct{}{}
	}

	for _, t := range fsmInternalTables {
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

		return fmt.Errorf("table %q is not classified as replicated, local-only, or fsm-internal", table)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate sqlite table names: %w", err)
	}

	return nil
}

// pinnedRunnerCtxKey is the context key under which a command apply stores a
// querier over its pinned connection. Applier methods resolve the runner via
// (*Database).runner so they target the pinned connection without mutating
// the shared db.conn pointer (which would race with concurrent DB users and
// return "sql: database is closed" once the pinned handle is closed).
type pinnedRunnerCtxKey struct{}

func (db *Database) writer() *sql.DB {
	return db.conn()
}

func (db *Database) reader() *sql.DB {
	return db.readConnection
}

// runner returns the querier to use for the current operation. Inside a
// command apply, the context carries the pinned querier; outside of an
// apply, callers get the shared writer pool.
func (db *Database) runner(ctx context.Context) querier {
	if r, ok := ctx.Value(pinnedRunnerCtxKey{}).(pinnedQuerier); ok {
		return r
	}

	return db.writer()
}

func (db *Database) readQuerier(ctx context.Context) querier {
	if r, ok := ctx.Value(pinnedRunnerCtxKey{}).(pinnedQuerier); ok {
		return r
	}

	return db.reader()
}

type pinnedQuerier struct {
	db *sql.DB
}

func (q pinnedQuerier) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return q.db.ExecContext(ctx, query, args...)
}

func (q pinnedQuerier) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return q.db.QueryContext(ctx, query, args...)
}

func (q pinnedQuerier) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return q.db.QueryRowContext(ctx, query, args...)
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

var (
	_ driver.ExecerContext      = (*noCloseConn)(nil)
	_ driver.QueryerContext     = (*noCloseConn)(nil)
	_ driver.ConnPrepareContext = (*noCloseConn)(nil)
)

func (c *noCloseConn) Close() error {
	return nil
}

func (c *noCloseConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	e, ok := c.Conn.(driver.ExecerContext)
	if !ok {
		return nil, driver.ErrSkip
	}

	return e.ExecContext(ctx, query, args)
}

func (c *noCloseConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	q, ok := c.Conn.(driver.QueryerContext)
	if !ok {
		return nil, driver.ErrSkip
	}

	return q.QueryContext(ctx, query, args)
}

func (c *noCloseConn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	if p, ok := c.Conn.(driver.ConnPrepareContext); ok {
		return p.PrepareContext(ctx, query)
	}

	return c.Prepare(query)
}
