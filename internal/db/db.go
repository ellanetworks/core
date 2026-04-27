// Copyright 2024 Ella Networks

// Package db provides a simplistic ORM to communicate with an SQL database for storage
package db

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/canonical/sqlair"
	"github.com/ellanetworks/core/internal/dbwriter"
	"github.com/ellanetworks/core/internal/logger"
	ellaraft "github.com/ellanetworks/core/internal/raft"
	"github.com/google/uuid"
	autopilot "github.com/hashicorp/raft-autopilot"
	_ "github.com/mattn/go-sqlite3"
	"go.opentelemetry.io/otel"
	"go.uber.org/zap"
)

var tracer = otel.Tracer("ella-core/db")

// Database holds the single SQLite handle backing the application.
//
// dbPath is the SQLite file path; dataDir is its parent directory, used for
// sibling artifacts (raft/ subdirectory, backup/restore staging).
type Database struct {
	dbPath         string
	dataDir        string
	restoreMu      sync.Mutex
	proposeMu      sync.Mutex
	raftManager    *ellaraft.Manager
	proposeTimeout time.Duration

	// migrationCheckCh fan-ins re-trigger signals from the FSM applier
	// (after UpsertClusterMember or CmdMigrateShared commits) so the
	// leader re-runs CheckPendingMigrations without waiting for the
	// next leadership change. Debounced by the worker.
	migrationCheckCh     chan struct{}
	migrationCheckCancel context.CancelFunc

	// Subscriber statements
	listSubscribersStmt         *sqlair.Statement
	countSubscribersStmt        *sqlair.Statement
	getSubscriberStmt           *sqlair.Statement
	createSubscriberStmt        *sqlair.Statement
	updateSubscriberProfileStmt *sqlair.Statement
	updateSubscriberSqnNumStmt  *sqlair.Statement
	deleteSubscriberStmt        *sqlair.Statement

	// IP Lease statements
	createLeaseStmt              *sqlair.Statement
	getDynamicLeaseStmt          *sqlair.Statement
	getLeaseBySessionStmt        *sqlair.Statement
	updateLeaseSessionStmt       *sqlair.Statement
	updateLeaseNodeStmt          *sqlair.Statement
	deleteLeaseStmt              *sqlair.Statement
	deleteAllDynamicLeasesStmt   *sqlair.Statement
	deleteDynLeasesByNodeStmt    *sqlair.Statement
	listActiveLeasesStmt         *sqlair.Statement
	listActiveLeasesByNodeStmt   *sqlair.Statement
	listLeasesByPoolStmt         *sqlair.Statement
	listLeaseAddressesByPoolStmt *sqlair.Statement
	countLeasesByPoolStmt        *sqlair.Statement
	countActiveLeasesStmt        *sqlair.Statement
	countLeasesByIMSIStmt        *sqlair.Statement
	listLeasesByPoolPageStmt     *sqlair.Statement
	listAllLeasesStmt            *sqlair.Statement

	// API Token statements
	listAPITokensStmt     *sqlair.Statement
	countAPITokensStmt    *sqlair.Statement
	createAPITokenStmt    *sqlair.Statement
	getAPITokenByNameStmt *sqlair.Statement
	getAPITokenByIDStmt   *sqlair.Statement
	deleteAPITokenStmt    *sqlair.Statement

	// Radio Event statements
	insertRadioEventStmt     *sqlair.Statement
	listRadioEventsStmt      *sqlair.Statement
	countRadioEventsStmt     *sqlair.Statement
	deleteOldRadioEventsStmt *sqlair.Statement
	deleteAllRadioEventsStmt *sqlair.Statement
	getRadioEventByIDStmt    *sqlair.Statement

	// Daily Usage statements
	incrementDailyUsageStmt   *sqlair.Statement
	getUsagePerDayStmt        *sqlair.Statement
	getUsagePerSubscriberStmt *sqlair.Statement
	deleteAllDailyUsageStmt   *sqlair.Statement
	deleteOldDailyUsageStmt   *sqlair.Statement

	// Data Network statements
	listDataNetworksStmt    *sqlair.Statement
	listAllDataNetworksStmt *sqlair.Statement
	getDataNetworkStmt      *sqlair.Statement
	getDataNetworkByIDStmt  *sqlair.Statement
	createDataNetworkStmt   *sqlair.Statement
	editDataNetworkStmt     *sqlair.Statement
	deleteDataNetworkStmt   *sqlair.Statement
	countDataNetworksStmt   *sqlair.Statement

	// N3 Settings statements
	insertDefaultN3SettingsStmt *sqlair.Statement
	updateN3SettingsStmt        *sqlair.Statement
	getN3SettingsStmt           *sqlair.Statement

	// NAT Settings statements
	insertDefaultNATSettingsStmt *sqlair.Statement
	getNATSettingsStmt           *sqlair.Statement
	upsertNATSettingsStmt        *sqlair.Statement

	// BGP Settings statements
	insertDefaultBGPSettingsStmt *sqlair.Statement
	getBGPSettingsStmt           *sqlair.Statement
	upsertBGPSettingsStmt        *sqlair.Statement

	// BGP Peers statements
	listBGPPeersStmt    *sqlair.Statement
	listAllBGPPeersStmt *sqlair.Statement
	getBGPPeerStmt      *sqlair.Statement
	createBGPPeerStmt   *sqlair.Statement
	updateBGPPeerStmt   *sqlair.Statement
	deleteBGPPeerStmt   *sqlair.Statement
	countBGPPeersStmt   *sqlair.Statement

	// BGP Import Prefixes statements
	listImportPrefixesByPeerStmt   *sqlair.Statement
	createImportPrefixStmt         *sqlair.Statement
	deleteImportPrefixesByPeerStmt *sqlair.Statement

	// Flow Accounting Settings statements
	insertDefaultFlowAccountingSettingsStmt *sqlair.Statement
	getFlowAccountingSettingsStmt           *sqlair.Statement
	upsertFlowAccountingSettingsStmt        *sqlair.Statement

	// Operator statements
	getOperatorStmt                      *sqlair.Statement
	initializeOperatorStmt               *sqlair.Statement
	updateOperatorTrackingStmt           *sqlair.Statement
	updateOperatorIDStmt                 *sqlair.Statement
	updateOperatorCodeStmt               *sqlair.Statement
	updateOperatorSecurityAlgorithmsStmt *sqlair.Statement
	updateOperatorSPNStmt                *sqlair.Statement
	updateOperatorAMFIdentityStmt        *sqlair.Statement
	updateOperatorClusterIDStmt          *sqlair.Statement

	// Home Network Key statements
	listHomeNetworkKeysStmt                    *sqlair.Statement
	getHomeNetworkKeyStmt                      *sqlair.Statement
	getHomeNetworkKeyBySchemeAndIdentifierStmt *sqlair.Statement
	createHomeNetworkKeyStmt                   *sqlair.Statement
	deleteHomeNetworkKeyStmt                   *sqlair.Statement
	countHomeNetworkKeysStmt                   *sqlair.Statement

	// Policies statements
	listPoliciesStmt      *sqlair.Statement
	getPolicyStmt         *sqlair.Statement
	getPolicyByLookupStmt *sqlair.Statement

	getPolicyByProfileAndSliceStmt *sqlair.Statement
	listPoliciesByProfileStmt      *sqlair.Statement
	listPoliciesByProfileAllStmt   *sqlair.Statement
	createPolicyStmt               *sqlair.Statement
	editPolicyStmt                 *sqlair.Statement
	deletePolicyStmt               *sqlair.Statement
	countPoliciesStmt              *sqlair.Statement
	countPoliciesInProfileStmt     *sqlair.Statement
	countPoliciesInSliceStmt       *sqlair.Statement
	countPoliciesInDataNetworkStmt *sqlair.Statement

	// Network Slices statements
	listNetworkSlicesStmt      *sqlair.Statement
	listAllNetworkSlicesStmt   *sqlair.Statement
	getNetworkSliceStmt        *sqlair.Statement
	getNetworkSliceByIDStmt    *sqlair.Statement
	listNetworkSlicesByIDsStmt *sqlair.Statement
	createNetworkSliceStmt     *sqlair.Statement
	editNetworkSliceStmt       *sqlair.Statement
	deleteNetworkSliceStmt     *sqlair.Statement
	countNetworkSlicesStmt     *sqlair.Statement

	// Profiles statements
	listProfilesStmt              *sqlair.Statement
	getProfileStmt                *sqlair.Statement
	getProfileByIDStmt            *sqlair.Statement
	createProfileStmt             *sqlair.Statement
	editProfileStmt               *sqlair.Statement
	deleteProfileStmt             *sqlair.Statement
	countProfilesStmt             *sqlair.Statement
	countSubscribersByProfileStmt *sqlair.Statement

	// Network Rules statements
	getNetworkRuleStmt             *sqlair.Statement
	createNetworkRuleStmt          *sqlair.Statement
	updateNetworkRuleStmt          *sqlair.Statement
	deleteNetworkRuleStmt          *sqlair.Statement
	deleteNetworkRulesByPolicyStmt *sqlair.Statement
	countNetworkRulesStmt          *sqlair.Statement
	listRulesForPolicyStmt         *sqlair.Statement

	// Retention Policy statements
	selectRetentionPolicyStmt *sqlair.Statement
	upsertRetentionPolicyStmt *sqlair.Statement

	// Routes statements
	listRoutesStmt  *sqlair.Statement
	getRouteStmt    *sqlair.Statement
	createRouteStmt *sqlair.Statement
	deleteRouteStmt *sqlair.Statement
	countRoutesStmt *sqlair.Statement

	// Audit Log statements
	insertAuditLogStmt        *sqlair.Statement
	listAuditLogsFilteredStmt *sqlair.Statement
	deleteOldAuditLogsStmt    *sqlair.Statement
	countAuditLogsStmt        *sqlair.Statement

	// Flow Report statements
	insertFlowReportStmt                *sqlair.Statement
	listFlowReportsStmt                 *sqlair.Statement
	countFlowReportsStmt                *sqlair.Statement
	deleteOldFlowReportsStmt            *sqlair.Statement
	deleteAllFlowReportsStmt            *sqlair.Statement
	getFlowReportByIDStmt               *sqlair.Statement
	listFlowReportsByDayStmt            *sqlair.Statement
	listFlowReportsBySubscriberStmt     *sqlair.Statement
	flowReportProtocolCountsStmt        *sqlair.Statement
	flowReportTopDestinationsUplinkStmt *sqlair.Statement

	// Session statements
	createSessionStmt            *sqlair.Statement
	getSessionByTokenHashStmt    *sqlair.Statement
	deleteSessionByTokenHashStmt *sqlair.Statement
	deleteExpiredSessionsStmt    *sqlair.Statement
	countSessionsByUserStmt      *sqlair.Statement
	deleteOldestSessionsStmt     *sqlair.Statement
	deleteAllSessionsForUserStmt *sqlair.Statement
	deleteAllSessionsStmt        *sqlair.Statement

	// JWT Secret statements
	getJWTSecretStmt    *sqlair.Statement
	upsertJWTSecretStmt *sqlair.Statement

	// User statements
	listUsersStmt        *sqlair.Statement
	getUserStmt          *sqlair.Statement
	getUserByIDStmt      *sqlair.Statement
	createUserStmt       *sqlair.Statement
	editUserStmt         *sqlair.Statement
	editUserPasswordStmt *sqlair.Statement
	deleteUserStmt       *sqlair.Statement
	countUsersStmt       *sqlair.Statement

	// Cluster Members statements
	listClusterMembersStmt  *sqlair.Statement
	getClusterMemberStmt    *sqlair.Statement
	upsertClusterMemberStmt *sqlair.Statement
	deleteClusterMemberStmt *sqlair.Statement
	countClusterMembersStmt *sqlair.Statement
	setDrainStateStmt       *sqlair.Statement

	// Cluster PKI statements
	listPKIRootsStmt             *sqlair.Statement
	insertPKIRootStmt            *sqlair.Statement
	setPKIRootStatusStmt         *sqlair.Statement
	deletePKIRootStmt            *sqlair.Statement
	listPKIIntermediatesStmt     *sqlair.Statement
	insertPKIIntermediateStmt    *sqlair.Statement
	setPKIIntermediateStatusStmt *sqlair.Statement
	deletePKIIntermediateStmt    *sqlair.Statement
	insertIssuedCertStmt         *sqlair.Statement
	listIssuedCertsByNodeStmt    *sqlair.Statement
	listIssuedCertsActiveStmt    *sqlair.Statement
	deleteIssuedCertsExpiredStmt *sqlair.Statement
	insertRevokedCertStmt        *sqlair.Statement
	listRevokedCertsStmt         *sqlair.Statement
	deleteRevokedCertsPurgedStmt *sqlair.Statement
	insertJoinTokenStmt          *sqlair.Statement
	getJoinTokenStmt             *sqlair.Statement
	consumeJoinTokenStmt         *sqlair.Statement
	deleteJoinTokensStaleStmt    *sqlair.Statement
	initPKIStateStmt             *sqlair.Statement
	getPKIStateStmt              *sqlair.Statement
	allocateSerialStmt           *sqlair.Statement

	// connPtr holds the SQLite handle for the application database. Reopen
	// atomically swaps the pointer so concurrent readers (API handlers,
	// FSM apply goroutines) never see a torn value. Readers use conn() to
	// load the current pointer.
	connPtr atomic.Pointer[sqlair.DB]

	// appliedSchemaCache mirrors schema_version.version for the
	// op-gate / apply-gate hot path. Updated by refreshAppliedSchema.
	appliedSchemaCache atomic.Int64
}

// conn returns the current *sqlair.DB handle.
func (db *Database) conn() *sqlair.DB {
	return db.connPtr.Load()
}

const DBFilename = "ella.db"

// Initial Retention Policy values
const (
	DefaultLogRetentionDays             = 7
	DefaultSubscriberUsageRetentionDays = 365
	DefaultFlowReportsRetentionDays     = 7
)

// Initial operator values
const (
	InitialMcc = "001"
	InitialMnc = "01"
)

var InitialSupportedTacs = []string{"000001"}

// Initial Network Slice values
const (
	InitialSliceName = "default"
	InitialSliceSst  = 1
)

// Initial Profile values
const (
	InitialProfileName           = "default"
	InitialProfileUeAmbrUplink   = "200 Mbps"
	InitialProfileUeAmbrDownlink = "200 Mbps"
)

// Initial Data network values
const (
	InitialDataNetworkName   = "internet"
	InitialDataNetworkIPPool = "10.45.0.0/22"
	InitialDataNetworkDNS    = "8.8.8.8"
	InitialDataNetworkMTU    = 1400
)

// Initial Policy values
const (
	InitialPolicyName                = "default"
	InitialPolicySessionAmbrUplink   = "200 Mbps"
	InitialPolicySessionAmbrDownlink = "200 Mbps"
	InitialPolicyVar5qi              = 9 // Default 5QI for non-GBR
	InitialPolicyArp                 = 1 // Default ARP of 1
)

// openSQLiteConnection opens a SQLite database at the given path and configures
// connection limits, busy timeout, WAL journaling, synchronous mode, and foreign keys.
func openSQLiteConnection(ctx context.Context, databasePath string) (*sql.DB, error) {
	// _txlock=immediate makes every BEGIN use BEGIN IMMEDIATE, which
	// acquires a write lock up front. This is important for migrations
	// (prevents two processes from entering the same migration) and is
	// harmless for normal operations because SetMaxOpenConns(1) already
	// serialises all in-process access.
	dsn := databasePath + "?_txlock=immediate"

	sqlConnection, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	sqlConnection.SetMaxOpenConns(1)

	pragmas := []struct {
		sql  string
		desc string
	}{
		{"PRAGMA busy_timeout = 5000;", "set busy_timeout"},
		{"PRAGMA journal_mode = WAL;", "enable WAL journaling"},
		{"PRAGMA synchronous = NORMAL;", "set synchronous to NORMAL"},
		{"PRAGMA foreign_keys = ON;", "enable foreign key support"},
	}

	for _, p := range pragmas {
		if _, err := sqlConnection.ExecContext(ctx, p.sql); err != nil {
			_ = sqlConnection.Close()
			return nil, fmt.Errorf("failed to %s: %w", p.desc, err)
		}
	}

	return sqlConnection, nil
}

// Close closes the database connection and shuts down the raft manager.
// Both are always attempted and any errors are joined.
func (db *Database) Close() error {
	var raftErr, connErr error

	if db.migrationCheckCancel != nil {
		db.migrationCheckCancel()
	}

	if db.raftManager != nil {
		raftErr = db.raftManager.Shutdown()
	}

	if c := db.conn(); c != nil {
		connErr = c.PlainDB().Close()
	}

	return errors.Join(raftErr, connErr)
}

// signalMigrationCheck requests a non-blocking re-run of
// CheckPendingMigrations. Debounced by runMigrationCheckWorker. Safe to
// call from applier goroutines.
func (db *Database) signalMigrationCheck() {
	if db.migrationCheckCh == nil {
		return
	}

	select {
	case db.migrationCheckCh <- struct{}{}:
	default:
	}
}

// runMigrationCheckWorker consumes migrationCheckCh signals, waits 100ms
// to coalesce bursts, and calls CheckPendingMigrations. The check itself
// no-ops on followers; running it on every node is harmless because
// leader-state is checked inside.
func (db *Database) runMigrationCheckWorker(ctx context.Context) {
	const debounce = 100 * time.Millisecond

	for {
		select {
		case <-ctx.Done():
			return
		case <-db.migrationCheckCh:
		}

		timer := time.NewTimer(debounce)

	drain:
		for {
			select {
			case <-db.migrationCheckCh:
			case <-timer.C:
				break drain
			case <-ctx.Done():
				timer.Stop()
				return
			}
		}

		checkCtx, cancel := context.WithTimeout(ctx, 30*time.Second)

		if err := db.CheckPendingMigrations(checkCtx); err != nil {
			logger.WithTrace(checkCtx, logger.DBLog).Warn("pending migration check (re-trigger) failed",
				zap.Error(err))
		}

		cancel()
	}
}

// Dir returns the directory containing the database file.
func (db *Database) Dir() string {
	return db.dataDir
}

// Path returns the SQLite database file path.
func (db *Database) Path() string {
	return db.dbPath
}

// RaftAppliedIndex returns the index of the highest Raft log entry applied
// to the database. Exposed for tests and support bundles.
func (db *Database) RaftAppliedIndex() uint64 {
	if db.raftManager == nil {
		return 0
	}

	return db.raftManager.AppliedIndex()
}

// LeaderObserver returns the Raft leadership observer for registering
// callbacks that react to leadership transitions.
func (db *Database) LeaderObserver() *ellaraft.LeaderObserver {
	if db.raftManager == nil {
		return nil
	}

	return db.raftManager.LeaderObserver()
}

// IsLeader returns true if this node is the current Raft leader.
func (db *Database) IsLeader() bool {
	if db.raftManager == nil {
		return true
	}

	return db.raftManager.IsLeader()
}

// ApplyForwardedOperation is the leader-side entry point for a forwarded
// typed operation posted to /cluster/internal/propose. See
// operations.go for the full flow.

// ProposeTimeout exposes the Raft manager's configured propose timeout
// so handlers can apply the same bound to forwarded commands they
// commit on behalf of a follower.
func (db *Database) ProposeTimeout() time.Duration {
	if db.raftManager == nil {
		return 0
	}

	return db.raftManager.ProposeTimeout()
}

// NodeID returns this node's Raft node ID. Returns 0 when running standalone.
func (db *Database) NodeID() int {
	if db.raftManager == nil {
		return 0
	}

	return db.raftManager.NodeID()
}

// LeaderAddress returns the Raft transport address of the current leader.
func (db *Database) LeaderAddress() string {
	if db.raftManager == nil {
		return ""
	}

	return db.raftManager.LeaderAddress()
}

// LeaderAddressAndID returns the leader's Raft transport address together
// with its integer node-id. Either value is zero when there is no leader
// or when the leader's ServerID cannot be parsed as an integer.
func (db *Database) LeaderAddressAndID() (string, int) {
	if db.raftManager == nil {
		return "", 0
	}

	return db.raftManager.LeaderAddressAndID()
}

// RaftState returns the current Raft state as a string (Leader, Follower, etc.).
func (db *Database) RaftState() string {
	if db.raftManager == nil {
		return "Leader"
	}

	return db.raftManager.State().String()
}

// AutopilotState returns the current autopilot state snapshot, or nil
// when this node is not the leader (autopilot runs leader-only) or when
// autopilot has not yet produced a first tick. Never returns non-nil on
// a follower or in single-server mode.
func (db *Database) AutopilotState() *autopilot.State {
	if db.raftManager == nil {
		return nil
	}

	return db.raftManager.AutopilotState()
}

// ClusterEnabled returns whether clustering is active.
func (db *Database) ClusterEnabled() bool {
	if db.raftManager == nil {
		return false
	}

	return db.raftManager.ClusterEnabled()
}

// LeadershipTransfer triggers a leadership transfer to another voter. The raft
// library picks the most up-to-date follower (highest replicated nextIndex)
// excluding self.
func (db *Database) LeadershipTransfer() error {
	if db.raftManager == nil {
		return fmt.Errorf("clustering not enabled")
	}

	return db.raftManager.LeadershipTransfer()
}

// AddVoter adds a node to the Raft cluster. Only callable on the leader.
func (db *Database) AddVoter(nodeID int, raftAddress string) error {
	if db.raftManager == nil {
		return fmt.Errorf("clustering not enabled")
	}

	return db.raftManager.AddVoter(nodeID, raftAddress)
}

// AddNonvoter adds a node to the Raft cluster as a non-voting member.
func (db *Database) AddNonvoter(nodeID int, raftAddress string) error {
	if db.raftManager == nil {
		return fmt.Errorf("clustering not enabled")
	}

	return db.raftManager.AddNonvoter(nodeID, raftAddress)
}

// CurrentSchemaVersion reads the schema_version singleton.
// Returns 0 on error.
func (db *Database) CurrentSchemaVersion(ctx context.Context) (int, error) {
	var v int

	if err := db.conn().PlainDB().QueryRowContext(ctx,
		"SELECT version FROM schema_version WHERE id = 1").Scan(&v); err != nil {
		return 0, fmt.Errorf("read schema_version: %w", err)
	}

	return v, nil
}

// RequireSchema returns ErrMigrationPending when applied < minVersion.
// Prefer declaring RequireSchema(N) at op registration time; this
// helper is for non-op-driven handlers that need the same check.
func (db *Database) RequireSchema(ctx context.Context, minVersion int) error {
	current, err := db.CurrentSchemaVersion(ctx)
	if err != nil {
		return err
	}

	if current < minVersion {
		return ErrMigrationPending
	}

	return nil
}

// cachedAppliedSchema is an atomic-load read of schema_version.version
// for the op-gate hot path. Returns 0 before the first refresh.
func (db *Database) cachedAppliedSchema() int {
	return int(db.appliedSchemaCache.Load())
}

// refreshAppliedSchema repopulates the cache from schema_version.
// Called after initial migrations, after CmdMigrateShared apply, and
// after Reopen.
func (db *Database) refreshAppliedSchema(ctx context.Context) error {
	v, err := db.CurrentSchemaVersion(ctx)
	if err != nil {
		return err
	}

	db.appliedSchemaCache.Store(int64(v))

	return nil
}

// checkOpSchema is the call-time gate. Returns ErrMigrationPending
// when applied < minSchema; minSchema <= 1 short-circuits.
func (db *Database) checkOpSchema(minSchema int) error {
	if minSchema <= 1 {
		return nil
	}

	applied := db.cachedAppliedSchema()
	if applied == 0 {
		v, err := db.CurrentSchemaVersion(context.Background())
		if err != nil {
			return fmt.Errorf("op gate: read schema version: %w", err)
		}

		db.appliedSchemaCache.Store(int64(v))

		applied = v
	}

	if applied < minSchema {
		return ErrMigrationPending
	}

	return nil
}

// assertAppliedSchema is the apply-time counterpart. Returns a plain
// error so the FSM panic handler halts the node (matching the contract
// for any other apply failure).
func (db *Database) assertAppliedSchema(ctx context.Context, required int, label string) error {
	if required <= 1 {
		return nil
	}

	applied := db.cachedAppliedSchema()
	if applied == 0 {
		v, err := db.CurrentSchemaVersion(ctx)
		if err != nil {
			return fmt.Errorf("apply gate: read schema version: %w", err)
		}

		db.appliedSchemaCache.Store(int64(v))

		applied = v
	}

	if required > applied {
		return fmt.Errorf("apply gate: %s requires schema %d, local applied %d",
			label, required, applied)
	}

	return nil
}

// CheckPendingMigrations proposes CmdMigrateShared entries for each
// migration beyond the current applied version, bounded by what every
// voter can apply. Called on leadership transitions and when voter
// capabilities change. Only the leader proposes.
//
// The proposal gate upholds the rolling-upgrade invariant:
//
//	dbSchema ≤ binarySchema(v) for every voter v.
//
// If any voter has not yet self-announced its maxSchemaVersion
// (column default 0), the gate defers entirely: "unknown" is treated
// as "cannot apply." Learners are ignored — they can crash on an
// unsupported migration without breaking quorum.
func (db *Database) CheckPendingMigrations(ctx context.Context) error {
	if db.raftManager == nil || !db.raftManager.ClusterEnabled() {
		return nil
	}

	if !db.raftManager.IsLeader() {
		return nil
	}

	current, err := db.CurrentSchemaVersion(ctx)
	if err != nil {
		return err
	}

	binaryMax := SchemaVersion()
	if current >= binaryMax {
		return nil
	}

	floor, laggard, err := db.minVoterSchemaSupport(ctx)
	if err != nil {
		return err
	}

	target := binaryMax
	if floor < target {
		target = floor
	}

	if current >= target {
		logger.WithTrace(ctx, logger.DBLog).Info("Migration deferred: waiting on voter upgrades",
			zap.Int("current", current),
			zap.Int("binaryMax", binaryMax),
			zap.Int("voterFloor", floor),
			zap.Int("laggardNodeID", laggard),
		)

		return nil
	}

	for v := current + 1; v <= target; v++ {
		logger.WithTrace(ctx, logger.DBLog).Info("Proposing migration over Raft",
			zap.Int("targetVersion", v))

		if _, err := opMigrateShared.Invoke(db, migrateSharedPayload{TargetVersion: v}); err != nil {
			return fmt.Errorf("propose migration %d: %w", v, err)
		}
	}

	return nil
}

// PendingMigrationStatus is a read-only snapshot of cluster
// migration readiness; surfaced on /api/v1/status.
type PendingMigrationStatus struct {
	Pending       bool
	CurrentSchema int
	TargetSchema  int // bounded by min(binaryMax, voter floor); equals current when blocked
	LaggardNodeID int // voter holding target == current; zero when unblocked
}

// PendingMigrationInfo computes the snapshot. Read-only, safe on any
// node — cluster_members is replicated so followers see the same
// voter-capability data the leader uses.
func (db *Database) PendingMigrationInfo(ctx context.Context) (PendingMigrationStatus, error) {
	current, err := db.CurrentSchemaVersion(ctx)
	if err != nil {
		return PendingMigrationStatus{}, fmt.Errorf("read schema version: %w", err)
	}

	binaryMax := SchemaVersion()
	if current >= binaryMax {
		return PendingMigrationStatus{
			Pending:       false,
			CurrentSchema: current,
			TargetSchema:  current,
		}, nil
	}

	// Standalone (no raft): no voters to gate on.
	if db.raftManager == nil {
		return PendingMigrationStatus{
			Pending:       true,
			CurrentSchema: current,
			TargetSchema:  binaryMax,
		}, nil
	}

	floor, laggard, err := db.minVoterSchemaSupport(ctx)
	if err != nil {
		return PendingMigrationStatus{}, err
	}

	target := binaryMax
	if floor < target {
		target = floor
	}

	status := PendingMigrationStatus{
		Pending:       true,
		CurrentSchema: current,
		TargetSchema:  target,
	}

	if target == current {
		status.LaggardNodeID = laggard
	}

	return status, nil
}

// minVoterSchemaSupport returns the minimum maxSchemaVersion across
// voter rows in cluster_members and the nodeID of the laggard. Any
// voter whose maxSchemaVersion is 0 (not yet self-announced) returns
// a floor of 0 — the gate blocks until every voter reports support.
// When there are no voter rows yet (e.g. fresh bootstrap before any
// self-announce) the floor is the leader's own binary max, so the
// leader is not blocked from applying its own migrations.
func (db *Database) minVoterSchemaSupport(ctx context.Context) (int, int, error) {
	members, err := db.ListClusterMembers(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("list cluster members: %w", err)
	}

	floor := -1
	laggard := 0

	for _, m := range members {
		if m.Suffrage != "voter" {
			continue
		}

		if floor < 0 || m.MaxSchemaVersion < floor {
			floor = m.MaxSchemaVersion
			laggard = m.NodeID
		}
	}

	if floor < 0 {
		return SchemaVersion(), db.raftManager.NodeID(), nil
	}

	return floor, laggard, nil
}

// clusterCoordinator implements ellaraft.LeaderCallback to propose deferred
// migrations on leadership transitions.
type clusterCoordinator struct {
	db *Database
}

func newClusterCoordinator(db *Database) *clusterCoordinator {
	return &clusterCoordinator{db: db}
}

func (c *clusterCoordinator) OnBecameLeader() {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := c.db.CheckPendingMigrations(ctx); err != nil {
			logger.WithTrace(ctx, logger.DBLog).Warn("check pending migrations on leader change failed",
				zap.Error(err))
		}
	}()
}

func (c *clusterCoordinator) OnLostLeadership() {}

// RemoveServer removes a node from the Raft cluster. Only callable on the leader.
func (db *Database) RemoveServer(nodeID int) error {
	if db.raftManager == nil {
		return fmt.Errorf("clustering not enabled")
	}

	return db.raftManager.RemoveServer(nodeID)
}

// RunDiscovery performs cluster formation for HA mode. Must be called after
// the HTTP server starts so peers can reach this node's API.
// After discovery, the leader generates a cluster ID if none exists yet.
func (db *Database) RunDiscovery(ctx context.Context) error {
	if db.raftManager == nil {
		return nil
	}

	if err := db.raftManager.RunDiscovery(ctx); err != nil {
		return err
	}

	return nil
}

// PostInitClusterSetup generates the cluster ID (if absent) and upserts this
// node's cluster_members row. Must be called on the leader after Initialize()
// has seeded the operator row.
func (db *Database) PostInitClusterSetup(ctx context.Context, binaryVersion string) error {
	if db.raftManager == nil || !db.raftManager.IsLeader() {
		return nil
	}

	op, err := db.GetOperator(ctx)
	if err != nil {
		return fmt.Errorf("failed to read operator for cluster ID check: %w", err)
	}

	if op.ClusterID == "" {
		clusterID := uuid.New().String()
		if err := db.UpdateOperatorClusterID(ctx, clusterID); err != nil {
			return fmt.Errorf("failed to set cluster ID: %w", err)
		}

		logger.WithTrace(ctx, logger.DBLog).Info("Generated cluster ID", zap.String("cluster_id", clusterID))
	}

	if err := db.selfUpsertClusterMember(ctx, binaryVersion); err != nil {
		logger.WithTrace(ctx, logger.DBLog).Warn("self-upsert cluster member failed", zap.Error(err))
	}

	return nil
}

// selfUpsertClusterMember proposes this leader's own cluster_members row with
// the running binary version. Idempotent via ON CONFLICT(nodeID). Called after
// discovery on the leader. Followers self-announce via SelfAnnounce.
//
// Addresses are pulled from the raft manager (authoritative for raftAddress
// post-bind and for the configured apiAddress) rather than from any existing
// DB row, so first-time self-registration writes a fully populated row.
// Existing Suffrage is preserved to avoid clobbering a non-voter entry set
// during a rolling upgrade rejoin.
func (db *Database) selfUpsertClusterMember(ctx context.Context, binaryVersion string) error {
	if db.raftManager == nil {
		return nil
	}

	suffrage := "voter"

	if existing, err := db.GetClusterMember(ctx, db.raftManager.NodeID()); err == nil && existing != nil && existing.Suffrage != "" {
		suffrage = existing.Suffrage
	}

	member := &ClusterMember{
		NodeID:           db.raftManager.NodeID(),
		RaftAddress:      db.raftManager.RaftAddress(),
		APIAddress:       db.raftManager.APIAddress(),
		BinaryVersion:    binaryVersion,
		Suffrage:         suffrage,
		MaxSchemaVersion: SchemaVersion(),
	}

	return db.UpsertClusterMember(ctx, member)
}

// SelfAnnounce refreshes this node's cluster_members row so voter
// capability (binaryVersion + maxSchemaVersion) stays current. On the
// leader it proposes the upsert directly; on a follower it POSTs to the
// current leader's cluster port. Called on every startup after Raft is
// up so rolling-restart upgrades rendezvous on the migration gate.
func (db *Database) SelfAnnounce(ctx context.Context, binaryVersion string) error {
	if db.raftManager == nil || !db.raftManager.ClusterEnabled() {
		return nil
	}

	if db.raftManager.IsLeader() {
		return db.selfUpsertClusterMember(ctx, binaryVersion)
	}

	suffrage := "voter"

	if existing, err := db.GetClusterMember(ctx, db.raftManager.NodeID()); err == nil && existing != nil && existing.Suffrage != "" {
		suffrage = existing.Suffrage
	}

	payload := struct {
		NodeID           int    `json:"nodeId"`
		RaftAddress      string `json:"raftAddress"`
		APIAddress       string `json:"apiAddress"`
		BinaryVersion    string `json:"binaryVersion"`
		MaxSchemaVersion int    `json:"maxSchemaVersion"`
		Suffrage         string `json:"suffrage,omitempty"`
	}{
		NodeID:           db.raftManager.NodeID(),
		RaftAddress:      db.raftManager.RaftAddress(),
		APIAddress:       db.raftManager.APIAddress(),
		BinaryVersion:    binaryVersion,
		MaxSchemaVersion: SchemaVersion(),
		Suffrage:         suffrage,
	}

	return db.raftManager.SelfAnnounce(ctx, payload)
}

// NewDatabase opens (or creates) the SQLite database file at dbPath. The
// parent directory is used for sibling artifacts (raft/, backup/restore
// staging). A non-existent path is treated as a fresh install.
func NewDatabase(ctx context.Context, dbPath string, raftCfg ellaraft.ClusterConfig, raftOpts ...ellaraft.ManagerOption) (*Database, error) {
	dataDir := filepath.Dir(dbPath)

	sqlConn, err := openSQLiteConnection(ctx, dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if raftCfg.Enabled {
		if err := runMigrations(ctx, sqlConn, baselineVersion); err != nil {
			_ = sqlConn.Close()
			return nil, fmt.Errorf("schema migration failed: %w", err)
		}
	} else {
		if err := runMigrations(ctx, sqlConn, 0); err != nil {
			_ = sqlConn.Close()
			return nil, fmt.Errorf("schema migration failed: %w", err)
		}
	}

	if err := ensureFsmStateTable(ctx, sqlConn); err != nil {
		_ = sqlConn.Close()
		return nil, fmt.Errorf("ensure fsm_state table: %w", err)
	}

	db := new(Database)
	db.connPtr.Store(sqlair.NewDB(sqlConn))
	db.dbPath = dbPath
	db.dataDir = dataDir

	if err := db.assertTableReplicationClassification(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed table replication classification check: %w", err)
	}

	if err := db.refreshAppliedSchema(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("seed applied-schema cache: %w", err)
	}

	if err := db.PrepareStatements(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to prepare statements: %w", err)
	}

	raftMgr, err := ellaraft.NewManager(ctx, raftCfg, db, dataDir, raftOpts...)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to start raft manager: %w", err)
	}

	db.raftManager = raftMgr
	db.proposeTimeout = raftMgr.ProposeTimeout()

	db.migrationCheckCh = make(chan struct{}, 1)
	workerCtx, workerCancel := context.WithCancel(context.Background())
	db.migrationCheckCancel = workerCancel

	go db.runMigrationCheckWorker(workerCtx)

	if observer := raftMgr.LeaderObserver(); observer != nil {
		observer.Register(newClusterCoordinator(db))
	}

	RegisterMetrics(db)

	// In HA mode, defer Initialize() until RunDiscovery has formed or
	// joined the cluster and a leader exists — otherwise every propose()
	// here would fail with ErrNotLeader on a fresh follower. Callers must
	// invoke Initialize explicitly after RunDiscovery.
	if !raftCfg.Enabled {
		if err := db.Initialize(ctx); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("failed to initialize database: %w", err)
		}
	}

	logger.WithTrace(ctx, logger.DBLog).Debug("Database Initialized")

	return db, nil
}

// NewDatabaseWithoutRaft opens a Database backed by SQLite with no Raft
// manager attached. Writes are applied directly to the local database
// instead of going through propose/capture/replicate, so there is no
// leader election or bolt store startup cost. Intended for unit tests
// whose subject is not replication; production code must use NewDatabase.
func NewDatabaseWithoutRaft(ctx context.Context, dbPath string) (*Database, error) {
	dataDir := filepath.Dir(dbPath)

	sqlConn, err := openSQLiteConnection(ctx, dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := runMigrations(ctx, sqlConn, 0); err != nil {
		_ = sqlConn.Close()
		return nil, fmt.Errorf("schema migration failed: %w", err)
	}

	if err := ensureFsmStateTable(ctx, sqlConn); err != nil {
		_ = sqlConn.Close()
		return nil, fmt.Errorf("ensure fsm_state table: %w", err)
	}

	db := new(Database)
	db.connPtr.Store(sqlair.NewDB(sqlConn))
	db.dbPath = dbPath
	db.dataDir = dataDir

	if err := db.assertTableReplicationClassification(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed table replication classification check: %w", err)
	}

	if err := db.refreshAppliedSchema(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("seed applied-schema cache: %w", err)
	}

	if err := db.PrepareStatements(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to prepare statements: %w", err)
	}

	RegisterMetrics(db)

	if err := db.Initialize(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	return db, nil
}

// PrepareStatements compiles every registered sqlair.Statement.
//
// Forward-compat: when a post-baseline migration (v10+) adds a column
// referenced by a new statement, gate that statement on
// db.cachedAppliedSchema() — the matching op's RequireSchema prevents
// callers from reaching the unprepared pointer. See
// spec_rolling_upgrade.md §3.2 for the re-prep follow-up.
func (db *Database) PrepareStatements() error {
	type stmtDef struct {
		dest  **sqlair.Statement
		query string
		types []any
	}

	stmts := []stmtDef{
		// Subscribers
		{&db.listSubscribersStmt, fmt.Sprintf(listSubscribersPagedStmt, SubscribersTableName), []any{ListArgs{}, Subscriber{}, NumItems{}}},
		{&db.countSubscribersStmt, fmt.Sprintf(countSubscribersStmt, SubscribersTableName), []any{NumItems{}}},
		{&db.getSubscriberStmt, fmt.Sprintf(getSubscriberStmt, SubscribersTableName), []any{Subscriber{}}},
		{&db.createSubscriberStmt, fmt.Sprintf(createSubscriberStmt, SubscribersTableName), []any{Subscriber{}}},
		{&db.updateSubscriberProfileStmt, fmt.Sprintf(editSubscriberProfileStmt, SubscribersTableName), []any{Subscriber{}}},
		{&db.updateSubscriberSqnNumStmt, fmt.Sprintf(editSubscriberSeqNumStmt, SubscribersTableName), []any{Subscriber{}}},
		{&db.deleteSubscriberStmt, fmt.Sprintf(deleteSubscriberStmt, SubscribersTableName), []any{Subscriber{}}},

		// IP Leases
		{&db.createLeaseStmt, fmt.Sprintf(createLeaseStmt, IPLeasesTableName), []any{IPLease{}}},
		{&db.getDynamicLeaseStmt, fmt.Sprintf(getDynamicLeaseStmt, IPLeasesTableName), []any{IPLease{}}},
		{&db.getLeaseBySessionStmt, fmt.Sprintf(getLeaseBySessionStmt, IPLeasesTableName), []any{IPLease{}}},
		{&db.updateLeaseSessionStmt, fmt.Sprintf(updateLeaseSessionStmt, IPLeasesTableName), []any{IPLease{}}},
		{&db.updateLeaseNodeStmt, fmt.Sprintf(updateLeaseNodeStmt, IPLeasesTableName), []any{IPLease{}}},
		{&db.deleteLeaseStmt, fmt.Sprintf(deleteLeaseStmt, IPLeasesTableName), []any{IPLease{}}},
		{&db.deleteAllDynamicLeasesStmt, fmt.Sprintf(deleteAllDynamicLeasesStmt, IPLeasesTableName), nil},
		{&db.deleteDynLeasesByNodeStmt, fmt.Sprintf(deleteDynLeasesByNodeStmt, IPLeasesTableName), []any{IPLease{}}},
		{&db.listActiveLeasesStmt, fmt.Sprintf(listActiveLeasesStmt, IPLeasesTableName), []any{IPLease{}}},
		{&db.listActiveLeasesByNodeStmt, fmt.Sprintf(listActiveLeasesByNodeStmt, IPLeasesTableName), []any{IPLease{}}},
		{&db.listLeasesByPoolStmt, fmt.Sprintf(listLeasesByPoolStmt, IPLeasesTableName), []any{IPLease{}}},
		{&db.listLeaseAddressesByPoolStmt, fmt.Sprintf(listLeaseAddressesByPoolStmt, IPLeasesTableName), []any{IPLease{}}},
		{&db.countLeasesByPoolStmt, fmt.Sprintf(countLeasesByPoolStmt, IPLeasesTableName), []any{NumItems{}, IPLease{}}},
		{&db.countActiveLeasesStmt, fmt.Sprintf(countActiveLeasesStmt, IPLeasesTableName), []any{NumItems{}}},
		{&db.countLeasesByIMSIStmt, fmt.Sprintf(countLeasesByIMSIStmt, IPLeasesTableName), []any{NumItems{}, IPLease{}}},
		{&db.listLeasesByPoolPageStmt, fmt.Sprintf(listLeasesByPoolPageStmt, IPLeasesTableName), []any{ListArgs{}, IPLease{}, NumItems{}}},
		{&db.listAllLeasesStmt, fmt.Sprintf(listAllLeasesStmt, IPLeasesTableName), []any{IPLease{}}},

		// API Tokens
		{&db.listAPITokensStmt, fmt.Sprintf(listAPITokensPagedStmt, APITokensTableName), []any{ListArgs{}, APIToken{}, NumItems{}}},
		{&db.countAPITokensStmt, fmt.Sprintf(countAPITokensStmt, APITokensTableName), []any{APIToken{}, NumItems{}}},
		{&db.createAPITokenStmt, fmt.Sprintf(createAPITokenStmt, APITokensTableName), []any{APIToken{}}},
		{&db.getAPITokenByNameStmt, fmt.Sprintf(getByNameStmt, APITokensTableName), []any{APIToken{}}},
		{&db.deleteAPITokenStmt, fmt.Sprintf(deleteAPITokenStmt, APITokensTableName), []any{APIToken{}}},
		{&db.getAPITokenByIDStmt, fmt.Sprintf(getByTokenIDStmt, APITokensTableName), []any{APIToken{}}},

		// Radio Events
		{&db.insertRadioEventStmt, fmt.Sprintf(insertRadioEventStmt, RadioEventsTableName), []any{dbwriter.RadioEvent{}}},
		{&db.listRadioEventsStmt, fmt.Sprintf(listRadioEventsPagedFilteredStmt, RadioEventsTableName), []any{ListArgs{}, RadioEventFilters{}, dbwriter.RadioEvent{}, NumItems{}}},
		{&db.countRadioEventsStmt, fmt.Sprintf(countRadioEventsFilteredStmt, RadioEventsTableName), []any{RadioEventFilters{}, NumItems{}}},
		{&db.deleteOldRadioEventsStmt, fmt.Sprintf(deleteOldRadioEventsStmt, RadioEventsTableName), []any{cutoffArgs{}}},
		{&db.deleteAllRadioEventsStmt, fmt.Sprintf(deleteAllRadioEventsStmt, RadioEventsTableName), nil},
		{&db.getRadioEventByIDStmt, fmt.Sprintf(getRadioEventByIDStmt, RadioEventsTableName), []any{dbwriter.RadioEvent{}}},

		// Daily Usage
		{&db.incrementDailyUsageStmt, fmt.Sprintf(incrementDailyUsageStmt, DailyUsageTableName), []any{DailyUsage{}}},
		{&db.getUsagePerDayStmt, fmt.Sprintf(getUsagePerDayStmt, DailyUsageTableName), []any{UsageFilters{}, UsagePerDay{}}},
		{&db.getUsagePerSubscriberStmt, fmt.Sprintf(getUsagePerSubscriberStmt, DailyUsageTableName), []any{UsageFilters{}, UsagePerSub{}}},
		{&db.deleteAllDailyUsageStmt, fmt.Sprintf(deleteAllDailyUsageStmt, DailyUsageTableName), nil},
		{&db.deleteOldDailyUsageStmt, fmt.Sprintf(deleteOldDailyUsageStmt, DailyUsageTableName), []any{cutoffDaysArgs{}}},

		// Data Networks
		{&db.listDataNetworksStmt, fmt.Sprintf(listDataNetworksPagedStmt, DataNetworksTableName), []any{ListArgs{}, DataNetwork{}, NumItems{}}},
		{&db.listAllDataNetworksStmt, fmt.Sprintf(listAllDataNetworksStmt, DataNetworksTableName), []any{DataNetwork{}}},
		{&db.getDataNetworkStmt, fmt.Sprintf(getDataNetworkStmt, DataNetworksTableName), []any{DataNetwork{}}},
		{&db.getDataNetworkByIDStmt, fmt.Sprintf(getDataNetworkByIDStmt, DataNetworksTableName), []any{DataNetwork{}}},
		{&db.createDataNetworkStmt, fmt.Sprintf(createDataNetworkStmt, DataNetworksTableName), []any{DataNetwork{}}},
		{&db.editDataNetworkStmt, fmt.Sprintf(editDataNetworkStmt, DataNetworksTableName), []any{DataNetwork{}}},
		{&db.deleteDataNetworkStmt, fmt.Sprintf(deleteDataNetworkStmt, DataNetworksTableName), []any{DataNetwork{}}},
		{&db.countDataNetworksStmt, fmt.Sprintf(countDataNetworksStmt, DataNetworksTableName), []any{NumItems{}}},

		// N3 Settings
		{&db.insertDefaultN3SettingsStmt, fmt.Sprintf(insertDefaultN3SettingsStmt, N3SettingsTableName), []any{N3Settings{}}},
		{&db.updateN3SettingsStmt, fmt.Sprintf(upsertN3SettingsStmt, N3SettingsTableName), []any{N3Settings{}}},
		{&db.getN3SettingsStmt, fmt.Sprintf(getN3SettingsStmt, N3SettingsTableName), []any{N3Settings{}}},

		// NAT Settings
		{&db.insertDefaultNATSettingsStmt, fmt.Sprintf(insertDefaultNATSettingsStmt, NATSettingsTableName), []any{NATSettings{}}},
		{&db.getNATSettingsStmt, fmt.Sprintf(getNATSettingsStmt, NATSettingsTableName), []any{NATSettings{}}},
		{&db.upsertNATSettingsStmt, fmt.Sprintf(upsertNATSettingsStmt, NATSettingsTableName), []any{NATSettings{}}},

		// BGP Settings
		{&db.insertDefaultBGPSettingsStmt, fmt.Sprintf(insertDefaultBGPSettingsStmt, BGPSettingsTableName), []any{BGPSettings{}}},
		{&db.getBGPSettingsStmt, fmt.Sprintf(getBGPSettingsStmt, BGPSettingsTableName), []any{BGPSettings{}}},
		{&db.upsertBGPSettingsStmt, fmt.Sprintf(upsertBGPSettingsStmt, BGPSettingsTableName), []any{BGPSettings{}}},

		// BGP Peers
		{&db.listBGPPeersStmt, fmt.Sprintf(listBGPPeersPagedStmt, BGPPeersTableName), []any{ListArgs{}, BGPPeer{}, NumItems{}}},
		{&db.listAllBGPPeersStmt, fmt.Sprintf(listAllBGPPeersStmt, BGPPeersTableName), []any{BGPPeer{}}},
		{&db.getBGPPeerStmt, fmt.Sprintf(getBGPPeerStmt, BGPPeersTableName), []any{BGPPeer{}}},
		{&db.createBGPPeerStmt, fmt.Sprintf(createBGPPeerStmt, BGPPeersTableName), []any{BGPPeer{}}},
		{&db.updateBGPPeerStmt, fmt.Sprintf(updateBGPPeerStmt, BGPPeersTableName), []any{BGPPeer{}}},
		{&db.deleteBGPPeerStmt, fmt.Sprintf(deleteBGPPeerStmt, BGPPeersTableName), []any{BGPPeer{}}},
		{&db.countBGPPeersStmt, fmt.Sprintf(countBGPPeersStmt, BGPPeersTableName), []any{NumItems{}}},

		// BGP Import Prefixes
		{&db.listImportPrefixesByPeerStmt, fmt.Sprintf(listImportPrefixesByPeerStmt, BGPImportPrefixesTableName), []any{BGPImportPrefix{}}},
		{&db.createImportPrefixStmt, fmt.Sprintf(createImportPrefixStmt, BGPImportPrefixesTableName), []any{BGPImportPrefix{}}},
		{&db.deleteImportPrefixesByPeerStmt, fmt.Sprintf(deleteImportPrefixesByPeerStmt, BGPImportPrefixesTableName), []any{BGPImportPrefix{}}},

		// Flow Accounting Settings
		{&db.insertDefaultFlowAccountingSettingsStmt, fmt.Sprintf(insertDefaultFlowAccountingSettingsStmt, FlowAccountingSettingsTableName), []any{FlowAccountingSettings{}}},
		{&db.getFlowAccountingSettingsStmt, fmt.Sprintf(getFlowAccountingSettingsStmt, FlowAccountingSettingsTableName), []any{FlowAccountingSettings{}}},
		{&db.upsertFlowAccountingSettingsStmt, fmt.Sprintf(upsertFlowAccountingSettingsStmt, FlowAccountingSettingsTableName), []any{FlowAccountingSettings{}}},

		// Operator
		{&db.getOperatorStmt, fmt.Sprintf(getOperatorStmt, OperatorTableName), []any{Operator{}}},
		{&db.initializeOperatorStmt, fmt.Sprintf(initializeOperatorStmt, OperatorTableName), []any{Operator{}}},
		{&db.updateOperatorTrackingStmt, fmt.Sprintf(updateOperatorTrackingStmt, OperatorTableName), []any{Operator{}}},
		{&db.updateOperatorIDStmt, fmt.Sprintf(updateOperatorIDStmt, OperatorTableName), []any{Operator{}}},
		{&db.updateOperatorCodeStmt, fmt.Sprintf(updateOperatorCodeStmt, OperatorTableName), []any{Operator{}}},
		{&db.updateOperatorSecurityAlgorithmsStmt, fmt.Sprintf(updateOperatorSecurityAlgorithmsStmtConst, OperatorTableName), []any{Operator{}}},
		{&db.updateOperatorSPNStmt, fmt.Sprintf(updateOperatorSPNStmtConst, OperatorTableName), []any{Operator{}}},
		{&db.updateOperatorAMFIdentityStmt, fmt.Sprintf(updateOperatorAMFIdentityStmtConst, OperatorTableName), []any{Operator{}}},
		{&db.updateOperatorClusterIDStmt, fmt.Sprintf(updateOperatorClusterIDStmtConst, OperatorTableName), []any{Operator{}}},

		// Home Network Keys
		{&db.listHomeNetworkKeysStmt, fmt.Sprintf(listHomeNetworkKeysStmtStr, HomeNetworkKeysTableName), []any{HomeNetworkKey{}}},
		{&db.getHomeNetworkKeyStmt, fmt.Sprintf(getHomeNetworkKeyStmtStr, HomeNetworkKeysTableName), []any{HomeNetworkKey{}}},
		{&db.getHomeNetworkKeyBySchemeAndIdentifierStmt, fmt.Sprintf(getHomeNetworkKeyBySchemeAndIdentifierStmtStr, HomeNetworkKeysTableName), []any{HomeNetworkKey{}}},
		{&db.createHomeNetworkKeyStmt, fmt.Sprintf(createHomeNetworkKeyStmtStr, HomeNetworkKeysTableName), []any{HomeNetworkKey{}}},
		{&db.deleteHomeNetworkKeyStmt, fmt.Sprintf(deleteHomeNetworkKeyStmtStr, HomeNetworkKeysTableName), []any{HomeNetworkKey{}}},
		{&db.countHomeNetworkKeysStmt, fmt.Sprintf(countHomeNetworkKeysStmtStr, HomeNetworkKeysTableName), []any{NumItems{}}},

		// Policies
		{&db.listPoliciesStmt, fmt.Sprintf(listPoliciesPagedStmt, PoliciesTableName), []any{ListArgs{}, Policy{}, NumItems{}}},
		{&db.getPolicyStmt, fmt.Sprintf(getPolicyStmt, PoliciesTableName), []any{Policy{}}},
		{&db.getPolicyByLookupStmt, fmt.Sprintf(getPolicyByLookupStmt, PoliciesTableName), []any{Policy{}}},
		{&db.getPolicyByProfileAndSliceStmt, fmt.Sprintf(getPolicyByProfileAndSliceStmt, PoliciesTableName), []any{Policy{}}},
		{&db.listPoliciesByProfileStmt, fmt.Sprintf(listPoliciesByProfilePagedStmt, PoliciesTableName), []any{ListArgs{}, Policy{}, NumItems{}}},
		{&db.listPoliciesByProfileAllStmt, fmt.Sprintf(listPoliciesByProfileAllStmt, PoliciesTableName), []any{Policy{}}},
		{&db.createPolicyStmt, fmt.Sprintf(createPolicyStmt, PoliciesTableName), []any{Policy{}}},
		{&db.editPolicyStmt, fmt.Sprintf(editPolicyStmt, PoliciesTableName), []any{Policy{}}},
		{&db.deletePolicyStmt, fmt.Sprintf(deletePolicyStmt, PoliciesTableName), []any{Policy{}}},
		{&db.countPoliciesStmt, fmt.Sprintf(countPoliciesStmt, PoliciesTableName), []any{NumItems{}}},
		{&db.countPoliciesInProfileStmt, fmt.Sprintf(countPoliciesInProfileStmt, PoliciesTableName), []any{NumItems{}, Policy{}}},
		{&db.countPoliciesInSliceStmt, fmt.Sprintf(countPoliciesInSliceStmt, PoliciesTableName), []any{NumItems{}, Policy{}}},
		{&db.countPoliciesInDataNetworkStmt, fmt.Sprintf(countPoliciesInDataNetworkStmt, PoliciesTableName), []any{NumItems{}, Policy{}}},

		// Network Slices
		{&db.listNetworkSlicesStmt, fmt.Sprintf(listNetworkSlicesPagedStmt, NetworkSlicesTableName), []any{ListArgs{}, NetworkSlice{}, NumItems{}}},
		{&db.listAllNetworkSlicesStmt, fmt.Sprintf(listAllNetworkSlicesStmt, NetworkSlicesTableName), []any{NetworkSlice{}}},
		{&db.getNetworkSliceStmt, fmt.Sprintf(getNetworkSliceStmt, NetworkSlicesTableName), []any{NetworkSlice{}}},
		{&db.getNetworkSliceByIDStmt, fmt.Sprintf(getNetworkSliceByIDStmt, NetworkSlicesTableName), []any{NetworkSlice{}}},
		{&db.listNetworkSlicesByIDsStmt, fmt.Sprintf(listNetworkSlicesByIDsStmt, NetworkSlicesTableName), []any{NetworkSlice{}, SliceIDs{}}},
		{&db.createNetworkSliceStmt, fmt.Sprintf(createNetworkSliceStmt, NetworkSlicesTableName), []any{NetworkSlice{}}},
		{&db.editNetworkSliceStmt, fmt.Sprintf(editNetworkSliceStmt, NetworkSlicesTableName), []any{NetworkSlice{}}},
		{&db.deleteNetworkSliceStmt, fmt.Sprintf(deleteNetworkSliceStmt, NetworkSlicesTableName), []any{NetworkSlice{}}},
		{&db.countNetworkSlicesStmt, fmt.Sprintf(countNetworkSlicesStmt, NetworkSlicesTableName), []any{NumItems{}}},

		// Profiles
		{&db.listProfilesStmt, fmt.Sprintf(listProfilesPagedStmt, ProfilesTableName), []any{ListArgs{}, Profile{}, NumItems{}}},
		{&db.getProfileStmt, fmt.Sprintf(getProfileStmt, ProfilesTableName), []any{Profile{}}},
		{&db.getProfileByIDStmt, fmt.Sprintf(getProfileByIDStmt, ProfilesTableName), []any{Profile{}}},
		{&db.createProfileStmt, fmt.Sprintf(createProfileStmt, ProfilesTableName), []any{Profile{}}},
		{&db.editProfileStmt, fmt.Sprintf(editProfileStmt, ProfilesTableName), []any{Profile{}}},
		{&db.deleteProfileStmt, fmt.Sprintf(deleteProfileStmt, ProfilesTableName), []any{Profile{}}},
		{&db.countProfilesStmt, fmt.Sprintf(countProfilesStmt, ProfilesTableName), []any{NumItems{}}},
		{&db.countSubscribersByProfileStmt, fmt.Sprintf(countSubscribersInProfileStmt, SubscribersTableName), []any{NumItems{}, Subscriber{}}},

		// Network Rules
		{&db.getNetworkRuleStmt, fmt.Sprintf(getNetworkRuleStmt, NetworkRulesTableName), []any{NetworkRule{}}},
		{&db.createNetworkRuleStmt, fmt.Sprintf(createNetworkRuleStmt, NetworkRulesTableName), []any{NetworkRule{}}},
		{&db.updateNetworkRuleStmt, fmt.Sprintf(updateNetworkRuleStmt, NetworkRulesTableName), []any{NetworkRule{}}},
		{&db.deleteNetworkRuleStmt, fmt.Sprintf(deleteNetworkRuleStmt, NetworkRulesTableName), []any{NetworkRule{}}},
		{&db.deleteNetworkRulesByPolicyStmt, fmt.Sprintf(deleteNetworkRulesByPolicyStmt, NetworkRulesTableName), []any{NetworkRule{}}},
		{&db.countNetworkRulesStmt, fmt.Sprintf(countNetworkRulesStmt, NetworkRulesTableName), []any{NumItems{}}},
		{&db.listRulesForPolicyStmt, fmt.Sprintf(listRulesForPolicyStmt, NetworkRulesTableName), []any{NetworkRule{}}},

		// Retention Policy
		{&db.selectRetentionPolicyStmt, fmt.Sprintf(selectRetentionPolicyStmt, RetentionPolicyTableName), []any{RetentionPolicy{}}},
		{&db.upsertRetentionPolicyStmt, fmt.Sprintf(upsertRetentionPolicyStmt, RetentionPolicyTableName), []any{RetentionPolicy{}}},

		// Routes
		{&db.listRoutesStmt, fmt.Sprintf(listRoutesPageStmt, RoutesTableName), []any{ListArgs{}, Route{}, NumItems{}}},
		{&db.getRouteStmt, fmt.Sprintf(getRouteStmt, RoutesTableName), []any{Route{}}},
		{&db.createRouteStmt, fmt.Sprintf(createRouteStmt, RoutesTableName), []any{Route{}}},
		{&db.deleteRouteStmt, fmt.Sprintf(deleteRouteStmt, RoutesTableName), []any{Route{}}},
		{&db.countRoutesStmt, fmt.Sprintf(countRoutesStmt, RoutesTableName), []any{NumItems{}}},

		// Audit Logs
		{&db.insertAuditLogStmt, fmt.Sprintf(insertAuditLogStmt, AuditLogsTableName), []any{dbwriter.AuditLog{}}},
		{&db.listAuditLogsFilteredStmt, fmt.Sprintf(listAuditLogsFilteredPageStmt, AuditLogsTableName), []any{ListArgs{}, AuditLogFilters{}, dbwriter.AuditLog{}, NumItems{}}},
		{&db.deleteOldAuditLogsStmt, fmt.Sprintf(deleteOldAuditLogsStmt, AuditLogsTableName), []any{cutoffArgs{}}},
		{&db.countAuditLogsStmt, fmt.Sprintf(countAuditLogsStmt, AuditLogsTableName), []any{NumItems{}}},

		// Flow Reports
		{&db.insertFlowReportStmt, fmt.Sprintf(insertFlowReportStmt, FlowReportsTableName), []any{dbwriter.FlowReport{}}},
		{&db.listFlowReportsStmt, fmt.Sprintf(listFlowReportsPagedFilteredStmt, FlowReportsTableName), []any{ListArgs{}, FlowReportFilters{}, dbwriter.FlowReport{}, NumItems{}}},
		{&db.countFlowReportsStmt, fmt.Sprintf(countFlowReportsFilteredStmt, FlowReportsTableName), []any{FlowReportFilters{}, NumItems{}}},
		{&db.deleteOldFlowReportsStmt, fmt.Sprintf(deleteOldFlowReportsStmt, FlowReportsTableName), []any{cutoffArgs{}}},
		{&db.deleteAllFlowReportsStmt, fmt.Sprintf(deleteAllFlowReportsStmt, FlowReportsTableName), nil},
		{&db.getFlowReportByIDStmt, fmt.Sprintf(getFlowReportByIDStmt, FlowReportsTableName), []any{dbwriter.FlowReport{}}},
		{&db.listFlowReportsByDayStmt, fmt.Sprintf(listFlowReportsFilteredByDayStmt, FlowReportsTableName), []any{FlowReportFilters{}, dbwriter.FlowReport{}}},
		{&db.listFlowReportsBySubscriberStmt, fmt.Sprintf(listFlowReportsFilteredBySubscriberStmt, FlowReportsTableName), []any{FlowReportFilters{}, dbwriter.FlowReport{}}},
		{&db.flowReportProtocolCountsStmt, fmt.Sprintf(flowReportProtocolCountsStmt, FlowReportsTableName), []any{FlowReportFilters{}, FlowReportProtocolCount{}}},
		{&db.flowReportTopDestinationsUplinkStmt, fmt.Sprintf(flowReportTopDestinationsUplinkStmt, FlowReportsTableName), []any{FlowReportFilters{}, FlowReportIPCount{}}},

		// Sessions
		{&db.createSessionStmt, fmt.Sprintf(createSessionStmt, SessionsTableName), []any{Session{}}},
		{&db.getSessionByTokenHashStmt, fmt.Sprintf(getSessionByTokenHashStmt, SessionsTableName), []any{Session{}}},
		{&db.deleteSessionByTokenHashStmt, fmt.Sprintf(deleteSessionByTokenHashStmt, SessionsTableName), []any{Session{}}},
		{&db.deleteExpiredSessionsStmt, fmt.Sprintf(deleteExpiredSessionsStmt, SessionsTableName), []any{SessionCutoff{}}},
		{&db.countSessionsByUserStmt, fmt.Sprintf(countSessionsByUserStmt, SessionsTableName), []any{UserIDArgs{}, NumItems{}}},
		{&db.deleteOldestSessionsStmt, fmt.Sprintf(deleteOldestSessionsStmt, SessionsTableName, SessionsTableName), []any{DeleteOldestArgs{}}},
		{&db.deleteAllSessionsForUserStmt, fmt.Sprintf(deleteAllSessionsForUserStmt, SessionsTableName), []any{UserIDArgs{}}},
		{&db.deleteAllSessionsStmt, fmt.Sprintf(deleteAllSessionsStmt, SessionsTableName), nil},

		// JWT Secret
		{&db.getJWTSecretStmt, fmt.Sprintf(getJWTSecretStmt, JWTSecretTableName), []any{JWTSecret{}}},
		{&db.upsertJWTSecretStmt, fmt.Sprintf(upsertJWTSecretStmt, JWTSecretTableName), []any{JWTSecret{}}},

		// Users
		{&db.listUsersStmt, fmt.Sprintf(listUsersPageStmt, UsersTableName), []any{ListArgs{}, User{}, NumItems{}}},
		{&db.getUserStmt, fmt.Sprintf(getUserStmt, UsersTableName), []any{User{}}},
		{&db.getUserByIDStmt, fmt.Sprintf(getUserByIDStmt, UsersTableName), []any{User{}}},
		{&db.createUserStmt, fmt.Sprintf(createUserStmt, UsersTableName), []any{User{}}},
		{&db.editUserStmt, fmt.Sprintf(editUserStmt, UsersTableName), []any{User{}}},
		{&db.editUserPasswordStmt, fmt.Sprintf(editUserPasswordStmt, UsersTableName), []any{User{}}},
		{&db.deleteUserStmt, fmt.Sprintf(deleteUserStmt, UsersTableName), []any{User{}}},
		{&db.countUsersStmt, fmt.Sprintf(countUsersStmt, UsersTableName), []any{NumItems{}}},

		// Cluster Members
		{&db.listClusterMembersStmt, fmt.Sprintf(listClusterMembersStmtStr, ClusterMembersTableName), []any{ClusterMember{}}},
		{&db.getClusterMemberStmt, fmt.Sprintf(getClusterMemberStmtStr, ClusterMembersTableName), []any{ClusterMember{}}},
		{&db.upsertClusterMemberStmt, fmt.Sprintf(upsertClusterMemberStmtStr, ClusterMembersTableName), []any{ClusterMember{}}},
		{&db.deleteClusterMemberStmt, fmt.Sprintf(deleteClusterMemberStmtStr, ClusterMembersTableName), []any{ClusterMember{}}},
		{&db.countClusterMembersStmt, fmt.Sprintf(countClusterMembersStmtStr, ClusterMembersTableName), []any{NumItems{}}},
		{&db.setDrainStateStmt, fmt.Sprintf(setDrainStateStmtStr, ClusterMembersTableName), []any{ClusterMember{}}},

		// Cluster PKI
		{&db.listPKIRootsStmt, fmt.Sprintf(listPKIRootsStmtStr, ClusterPKIRootsTableName), []any{ClusterPKIRoot{}}},
		{&db.insertPKIRootStmt, fmt.Sprintf(insertPKIRootStmtStr, ClusterPKIRootsTableName), []any{ClusterPKIRoot{}}},
		{&db.setPKIRootStatusStmt, fmt.Sprintf(setPKIRootStatusStmtStr, ClusterPKIRootsTableName), []any{ClusterPKIRoot{}}},
		{&db.deletePKIRootStmt, fmt.Sprintf(deletePKIRootStmtStr, ClusterPKIRootsTableName), []any{ClusterPKIRoot{}}},
		{&db.listPKIIntermediatesStmt, fmt.Sprintf(listPKIIntermediatesStmtStr, ClusterPKIIntermediatesTableName), []any{ClusterPKIIntermediate{}}},
		{&db.insertPKIIntermediateStmt, fmt.Sprintf(insertPKIIntermediateStmtStr, ClusterPKIIntermediatesTableName), []any{ClusterPKIIntermediate{}}},
		{&db.setPKIIntermediateStatusStmt, fmt.Sprintf(setPKIIntermediateStatusStmtStr, ClusterPKIIntermediatesTableName), []any{ClusterPKIIntermediate{}}},
		{&db.deletePKIIntermediateStmt, fmt.Sprintf(deletePKIIntermediateStmtStr, ClusterPKIIntermediatesTableName), []any{ClusterPKIIntermediate{}}},
		{&db.insertIssuedCertStmt, fmt.Sprintf(insertIssuedCertStmtStr, ClusterIssuedCertsTableName), []any{ClusterIssuedCert{}}},
		{&db.listIssuedCertsByNodeStmt, fmt.Sprintf(listIssuedCertsByNodeStmtStr, ClusterIssuedCertsTableName), []any{ClusterIssuedCert{}}},
		{&db.listIssuedCertsActiveStmt, fmt.Sprintf(listIssuedCertsActiveStmtStr, ClusterIssuedCertsTableName), []any{ClusterIssuedCert{}}},
		{&db.deleteIssuedCertsExpiredStmt, fmt.Sprintf(deleteIssuedCertsExpiredStmtStr, ClusterIssuedCertsTableName), []any{ClusterIssuedCert{}}},
		{&db.insertRevokedCertStmt, fmt.Sprintf(insertRevokedCertStmtStr, ClusterRevokedCertsTableName), []any{ClusterRevokedCert{}}},
		{&db.listRevokedCertsStmt, fmt.Sprintf(listRevokedCertsStmtStr, ClusterRevokedCertsTableName), []any{ClusterRevokedCert{}}},
		{&db.deleteRevokedCertsPurgedStmt, fmt.Sprintf(deleteRevokedCertsPurgedStmtStr, ClusterRevokedCertsTableName), []any{ClusterRevokedCert{}}},
		{&db.insertJoinTokenStmt, fmt.Sprintf(insertJoinTokenStmtStr, ClusterJoinTokensTableName), []any{ClusterJoinToken{}}},
		{&db.getJoinTokenStmt, fmt.Sprintf(getJoinTokenStmtStr, ClusterJoinTokensTableName), []any{ClusterJoinToken{}}},
		{&db.consumeJoinTokenStmt, fmt.Sprintf(consumeJoinTokenStmtStr, ClusterJoinTokensTableName), []any{ClusterJoinToken{}}},
		{&db.deleteJoinTokensStaleStmt, fmt.Sprintf(deleteJoinTokensStaleStmtStr, ClusterJoinTokensTableName), []any{ClusterJoinToken{}}},
		{&db.initPKIStateStmt, fmt.Sprintf(initPKIStateStmtStr, ClusterPKIStateTableName), []any{ClusterPKIState{}}},
		{&db.getPKIStateStmt, fmt.Sprintf(getPKIStateStmtStr, ClusterPKIStateTableName), []any{ClusterPKIState{}}},
		{&db.allocateSerialStmt, fmt.Sprintf(allocateSerialStmtStr, ClusterPKIStateTableName), []any{ClusterPKIState{}}},
	}

	for _, s := range stmts {
		stmt, err := sqlair.Prepare(s.query, s.types...)
		if err != nil {
			return fmt.Errorf("failed to prepare statement: %w", err)
		}

		*s.dest = stmt
	}

	return nil
}

// WaitForInitialization polls until the leader's Initialize data has
// replicated to this follower (operator row exists), or ctx is cancelled.
func (db *Database) WaitForInitialization(ctx context.Context, timeout time.Duration) error {
	deadline := time.After(timeout)

	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		if db.IsOperatorInitialized(ctx) {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline:
			return fmt.Errorf("timed out waiting for leader initialization to replicate")
		case <-ticker.C:
		}
	}
}

func (db *Database) Initialize(ctx context.Context) error {
	initSteps := []struct {
		name string
		fn   func() error
	}{
		{"NAT settings", func() error { return db.InitializeNATSettings(ctx) }},
		{"flow accounting settings", func() error { return db.InitializeFlowAccountingSettings(ctx) }},
		{"BGP settings", func() error { return db.InitializeBGPSettings(ctx) }},
		{"JWT secret", func() error { return db.InitializeJWTSecret(ctx) }},
		{"N3 settings", func() error { return db.InitializeN3Settings(ctx) }},
	}

	for _, step := range initSteps {
		if err := step.fn(); err != nil {
			return fmt.Errorf("failed to initialize %s: %w", step.name, err)
		}
	}

	if !db.IsOperatorInitialized(ctx) {
		initialOp, err := generateOperatorCode()
		if err != nil {
			return fmt.Errorf("couldn't generate operator code: %w", err)
		}

		initialOperator := &Operator{
			Mcc:          InitialMcc,
			Mnc:          InitialMnc,
			OperatorCode: initialOp,
		}

		err = initialOperator.SetSupportedTacs(InitialSupportedTacs)
		if err != nil {
			return fmt.Errorf("failed to set supported TACs: %w", err)
		}

		err = db.InitializeOperator(ctx, initialOperator)
		if err != nil {
			return fmt.Errorf("failed to initialize network configuration: %v", err)
		}
	}

	numKeys, err := db.CountHomeNetworkKeys(ctx)
	if err != nil {
		return fmt.Errorf("failed to count home network keys: %w", err)
	}

	if numKeys == 0 {
		initialHNPrivateKey, err := generateHomeNetworkPrivateKey()
		if err != nil {
			return fmt.Errorf("failed to generate default home network key: %w", err)
		}

		defaultKey := &HomeNetworkKey{
			KeyIdentifier: 0,
			Scheme:        "A",
			PrivateKey:    initialHNPrivateKey,
		}

		if err := db.CreateHomeNetworkKey(ctx, defaultKey); err != nil {
			return fmt.Errorf("failed to create default home network key: %w", err)
		}
	}

	if !db.IsRetentionPolicyInitialized(ctx, CategoryAuditLogs) {
		initialPolicy := &RetentionPolicy{
			Category: CategoryAuditLogs,
			Days:     DefaultLogRetentionDays,
		}

		if err := db.SetRetentionPolicy(ctx, initialPolicy); err != nil {
			return fmt.Errorf("failed to initialize log retention policy: %v", err)
		}

		logger.WithTrace(ctx, logger.DBLog).Info("Initialized audit log retention policy", zap.Int("days", DefaultLogRetentionDays))
	}

	if !db.IsRetentionPolicyInitialized(ctx, CategoryRadioLogs) {
		initialPolicy := &RetentionPolicy{
			Category: CategoryRadioLogs,
			Days:     DefaultLogRetentionDays,
		}

		if err := db.SetRetentionPolicy(ctx, initialPolicy); err != nil {
			return fmt.Errorf("failed to initialize radio event retention policy: %v", err)
		}

		logger.WithTrace(ctx, logger.DBLog).Info("Initialized radio event retention policy", zap.Int("days", DefaultLogRetentionDays))
	}

	if !db.IsRetentionPolicyInitialized(ctx, CategorySubscriberUsage) {
		initialPolicy := &RetentionPolicy{
			Category: CategorySubscriberUsage,
			Days:     DefaultSubscriberUsageRetentionDays,
		}

		if err := db.SetRetentionPolicy(ctx, initialPolicy); err != nil {
			return fmt.Errorf("failed to initialize subscriber usage retention policy: %v", err)
		}

		logger.WithTrace(ctx, logger.DBLog).Info("Initialized subscriber usage retention policy", zap.Int("days", DefaultSubscriberUsageRetentionDays))
	}

	if !db.IsRetentionPolicyInitialized(ctx, CategoryFlowReports) {
		initialPolicy := &RetentionPolicy{
			Category: CategoryFlowReports,
			Days:     DefaultFlowReportsRetentionDays,
		}

		if err := db.SetRetentionPolicy(ctx, initialPolicy); err != nil {
			return fmt.Errorf("failed to initialize flow reports retention policy: %v", err)
		}

		logger.WithTrace(ctx, logger.DBLog).Info("Initialized flow reports retention policy", zap.Int("days", DefaultFlowReportsRetentionDays))
	}

	numDataNetworks, err := db.CountDataNetworks(ctx)
	if err != nil {
		return fmt.Errorf("failed to get number of data networks: %v", err)
	}

	if numDataNetworks == 0 {
		initialDataNetwork := &DataNetwork{
			Name:   InitialDataNetworkName,
			IPPool: InitialDataNetworkIPPool,
			DNS:    InitialDataNetworkDNS,
			MTU:    InitialDataNetworkMTU,
		}
		if err := db.CreateDataNetwork(ctx, initialDataNetwork); err != nil {
			return fmt.Errorf("failed to create default data network: %v", err)
		}

		dataNetwork, err := db.GetDataNetwork(ctx, InitialDataNetworkName)
		if err != nil {
			return fmt.Errorf("failed to get default data network: %v", err)
		}

		initialSlice := &NetworkSlice{
			Name: InitialSliceName,
			Sst:  InitialSliceSst,
		}
		if err := db.CreateNetworkSlice(ctx, initialSlice); err != nil {
			return fmt.Errorf("failed to create default network slice: %v", err)
		}

		slice, err := db.GetNetworkSlice(ctx, InitialSliceName)
		if err != nil {
			return fmt.Errorf("failed to get default network slice: %v", err)
		}

		initialProfile := &Profile{
			Name:           InitialProfileName,
			UeAmbrUplink:   InitialProfileUeAmbrUplink,
			UeAmbrDownlink: InitialProfileUeAmbrDownlink,
		}
		if err := db.CreateProfile(ctx, initialProfile); err != nil {
			return fmt.Errorf("failed to create default profile: %v", err)
		}

		profile, err := db.GetProfile(ctx, InitialProfileName)
		if err != nil {
			return fmt.Errorf("failed to get default profile: %v", err)
		}

		initialPolicy := &Policy{
			Name:                InitialPolicyName,
			ProfileID:           profile.ID,
			SliceID:             slice.ID,
			DataNetworkID:       dataNetwork.ID,
			Var5qi:              InitialPolicyVar5qi,
			Arp:                 InitialPolicyArp,
			SessionAmbrUplink:   InitialPolicySessionAmbrUplink,
			SessionAmbrDownlink: InitialPolicySessionAmbrDownlink,
		}

		if err := db.CreatePolicy(ctx, initialPolicy); err != nil {
			return fmt.Errorf("failed to create default policy: %v", err)
		}
	}

	return nil
}

// BeginTransaction starts a transaction against the database.
func (db *Database) BeginTransaction(ctx context.Context) (*Transaction, error) {
	tx, err := db.conn().Begin(ctx, nil)
	if err != nil {
		return nil, err
	}

	return &Transaction{tx: tx, db: db}, nil
}

// Transaction wraps a SQLair transaction.
type Transaction struct {
	tx *sqlair.TX
	db *Database
}

func (t *Transaction) Commit() error {
	return t.tx.Commit()
}

func (t *Transaction) Rollback() error {
	return t.tx.Rollback()
}

func generateOperatorCode() (string, error) {
	var op [16]byte

	_, err := rand.Read(op[:])

	return hex.EncodeToString(op[:]), err
}

func generateHomeNetworkPrivateKey() (string, error) {
	var pk [32]byte
	if _, err := rand.Read(pk[:]); err != nil {
		return "", err
	}

	pk[0] &= 248
	pk[31] &= 127
	pk[31] |= 64

	return hex.EncodeToString(pk[:]), nil
}
