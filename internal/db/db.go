// Copyright 2024 Ella Networks

// Package db provides a simplistic ORM to communicate with an SQL database for storage
package db

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"sync"

	"github.com/canonical/sqlair"
	"github.com/ellanetworks/core/internal/dbwriter"
	"github.com/ellanetworks/core/internal/logger"
	_ "github.com/mattn/go-sqlite3"
	"go.opentelemetry.io/otel"
	"go.uber.org/zap"
)

var tracer = otel.Tracer("ella-core/db")

// Database is the object used to communicate with the established repository.
type Database struct {
	filepath  string
	restoreMu sync.Mutex

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
	deleteLeaseStmt              *sqlair.Statement
	deleteAllDynamicLeasesStmt   *sqlair.Statement
	listActiveLeasesStmt         *sqlair.Statement
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
	listNetworkSlicesStmt    *sqlair.Statement
	listAllNetworkSlicesStmt *sqlair.Statement
	getNetworkSliceStmt      *sqlair.Statement
	getNetworkSliceByIDStmt  *sqlair.Statement
	createNetworkSliceStmt   *sqlair.Statement
	editNetworkSliceStmt     *sqlair.Statement
	deleteNetworkSliceStmt   *sqlair.Statement
	countNetworkSlicesStmt   *sqlair.Statement

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

	conn *sqlair.DB
}

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

// Close closes the connection to the repository cleanly.
func (db *Database) Close() error {
	if db.conn == nil {
		return nil
	}

	return db.conn.PlainDB().Close()
}

// NewDatabase connects to a given table in a given database,
// stores the connection information and returns an object containing the information.
// The database path must be a valid file path or ":memory:".
// The table will be created if it doesn't exist in the format expected by the package.
func NewDatabase(ctx context.Context, databasePath string) (*Database, error) {
	sqlConnection, err := openSQLiteConnection(ctx, databasePath)
	if err != nil {
		return nil, err
	}

	if err := RunMigrations(ctx, sqlConnection); err != nil {
		_ = sqlConnection.Close()
		return nil, fmt.Errorf("schema migration failed: %w", err)
	}

	db := new(Database)
	db.conn = sqlair.NewDB(sqlConnection)
	db.filepath = databasePath

	err = db.PrepareStatements()
	if err != nil {
		return nil, fmt.Errorf("failed to prepare statements: %w", err)
	}

	RegisterMetrics(db)

	err = db.Initialize(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	logger.WithTrace(ctx, logger.DBLog).Debug("Database Initialized")

	return db, nil
}

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
		{&db.deleteLeaseStmt, fmt.Sprintf(deleteLeaseStmt, IPLeasesTableName), []any{IPLease{}}},
		{&db.deleteAllDynamicLeasesStmt, fmt.Sprintf(deleteAllDynamicLeasesStmt, IPLeasesTableName), nil},
		{&db.listActiveLeasesStmt, fmt.Sprintf(listActiveLeasesStmt, IPLeasesTableName), []any{IPLease{}}},
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

func (db *Database) Initialize(ctx context.Context) error {
	err := db.InitializeNATSettings(ctx)
	if err != nil {
		return fmt.Errorf("failed to initialize NAT settings: %w", err)
	}

	err = db.InitializeFlowAccountingSettings(ctx)
	if err != nil {
		return fmt.Errorf("failed to initialize flow accounting settings: %w", err)
	}

	err = db.InitializeBGPSettings(ctx)
	if err != nil {
		return fmt.Errorf("failed to initialize BGP settings: %w", err)
	}

	err = db.InitializeJWTSecret(ctx)
	if err != nil {
		return fmt.Errorf("failed to initialize JWT secret: %w", err)
	}

	err = db.InitializeN3Settings(ctx)
	if err != nil {
		return fmt.Errorf("failed to initialize N3 settings: %w", err)
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

func (db *Database) BeginTransaction(ctx context.Context) (*Transaction, error) {
	tx, err := db.conn.Begin(ctx, nil)
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
