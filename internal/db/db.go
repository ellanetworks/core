// Copyright 2024 Ella Networks

// Package db provides a simplistic ORM to communicate with an SQL database for storage
package db

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"

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
	filepath string

	// Subscriber statements
	listSubscribersStmt          *sqlair.Statement
	countSubscribersStmt         *sqlair.Statement
	getSubscriberStmt            *sqlair.Statement
	createSubscriberStmt         *sqlair.Statement
	updateSubscriberPolicyStmt   *sqlair.Statement
	updateSubscriberSqnNumStmt   *sqlair.Statement
	deleteSubscriberStmt         *sqlair.Statement
	checkSubscriberIPStmt        *sqlair.Statement
	allocateSubscriberIPStmt     *sqlair.Statement
	releaseSubscriberIPStmt      *sqlair.Statement
	releaseAllIPStmt             *sqlair.Statement
	countSubscribersByPolicyStmt *sqlair.Statement
	countSubscribersWithIPStmt   *sqlair.Statement

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
	listDataNetworksStmt   *sqlair.Statement
	getDataNetworkStmt     *sqlair.Statement
	getDataNetworkByIDStmt *sqlair.Statement
	createDataNetworkStmt  *sqlair.Statement
	editDataNetworkStmt    *sqlair.Statement
	deleteDataNetworkStmt  *sqlair.Statement
	countDataNetworksStmt  *sqlair.Statement

	// N3 Settings statements
	insertDefaultN3SettingsStmt *sqlair.Statement
	updateN3SettingsStmt        *sqlair.Statement
	getN3SettingsStmt           *sqlair.Statement

	// NAT Settings statements
	insertDefaultNATSettingsStmt *sqlair.Statement
	getNATSettingsStmt           *sqlair.Statement
	upsertNATSettingsStmt        *sqlair.Statement

	// Operator statements
	getOperatorStmt                 *sqlair.Statement
	initializeOperatorStmt          *sqlair.Statement
	updateOperatorSliceStmt         *sqlair.Statement
	updateOperatorTrackingStmt      *sqlair.Statement
	updateOperatorIDStmt            *sqlair.Statement
	updateOperatorCodeStmt          *sqlair.Statement
	updateHomeNetworkPrivateKeyStmt *sqlair.Statement

	// Policies statements
	listPoliciesStmt  *sqlair.Statement
	getPolicyStmt     *sqlair.Statement
	getPolicyByIDStmt *sqlair.Statement
	createPolicyStmt  *sqlair.Statement
	editPolicyStmt    *sqlair.Statement
	deletePolicyStmt  *sqlair.Statement
	countPoliciesStmt *sqlair.Statement

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
	insertAuditLogStmt     *sqlair.Statement
	listAuditLogsStmt      *sqlair.Statement
	deleteOldAuditLogsStmt *sqlair.Statement
	countAuditLogsStmt     *sqlair.Statement

	// Session statements
	createSessionStmt            *sqlair.Statement
	getSessionByTokenHashStmt    *sqlair.Statement
	deleteSessionByTokenHashStmt *sqlair.Statement
	deleteExpiredSessionsStmt    *sqlair.Statement
	countSessionsByUserStmt      *sqlair.Statement
	deleteOldestSessionsStmt     *sqlair.Statement

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
)

// Initial operator values
const (
	InitialMcc         = "001"
	InitialMnc         = "01"
	InitialOperatorSst = 1
)

var (
	InitialOperatorSd    = []byte{0x10, 0x20, 0x30}
	InitialSupportedTacs = []string{"000001"}
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
	InitialPolicyName            = "default"
	InitialPolicyBitrateUplink   = "200 Mbps"
	InitialPolicyBitrateDownlink = "200 Mbps"
	InitialPolicyVar5qi          = 9 // Default 5QI for non-GBR
	InitialPolicyArp             = 1 // Default ARP of 1
)

// openSQLiteConnection opens a SQLite database at the given path and configures
// connection limits, busy timeout, WAL journaling, synchronous mode, and foreign keys.
func openSQLiteConnection(ctx context.Context, databasePath string) (*sql.DB, error) {
	sqlConnection, err := sql.Open("sqlite3", databasePath)
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

	// Initialize tables
	if _, err := sqlConnection.ExecContext(ctx, fmt.Sprintf(QueryCreateSubscribersTable, SubscribersTableName)); err != nil {
		return nil, err
	}

	if _, err := sqlConnection.ExecContext(ctx, fmt.Sprintf(QueryCreatePoliciesTable, PoliciesTableName)); err != nil {
		return nil, err
	}

	if _, err := sqlConnection.ExecContext(ctx, fmt.Sprintf(QueryCreateRoutesTable, RoutesTableName)); err != nil {
		return nil, err
	}

	if _, err := sqlConnection.ExecContext(ctx, fmt.Sprintf(QueryCreateOperatorTable, OperatorTableName)); err != nil {
		return nil, err
	}

	if _, err := sqlConnection.ExecContext(ctx, fmt.Sprintf(QueryCreateDataNetworksTable, DataNetworksTableName)); err != nil {
		return nil, err
	}

	if _, err := sqlConnection.ExecContext(ctx, fmt.Sprintf(QueryCreateUsersTable, UsersTableName)); err != nil {
		return nil, err
	}

	if _, err := sqlConnection.ExecContext(ctx, createSessionsTableSQL); err != nil {
		return nil, err
	}

	if _, err := sqlConnection.ExecContext(ctx, fmt.Sprintf(QueryCreateAuditLogsTable, AuditLogsTableName)); err != nil {
		return nil, err
	}

	if _, err := sqlConnection.ExecContext(ctx, fmt.Sprintf(QueryCreateRadioEventsTable, RadioEventsTableName)); err != nil {
		return nil, err
	}

	if _, err := sqlConnection.ExecContext(ctx, QueryCreateRadioEventsIndex); err != nil {
		return nil, err
	}

	if _, err := sqlConnection.ExecContext(ctx, fmt.Sprintf(QueryCreateRetentionPolicyTable, RetentionPolicyTableName)); err != nil {
		return nil, err
	}

	if _, err := sqlConnection.ExecContext(ctx, fmt.Sprintf(QueryCreateAPITokensTable, APITokensTableName)); err != nil {
		return nil, err
	}

	if _, err := sqlConnection.ExecContext(ctx, fmt.Sprintf(QueryCreateNATSettingsTable, NATSettingsTableName)); err != nil {
		return nil, err
	}

	if _, err := sqlConnection.ExecContext(ctx, fmt.Sprintf(QueryCreateN3SettingsTable, N3SettingsTableName)); err != nil {
		return nil, err
	}

	if _, err := sqlConnection.ExecContext(ctx, fmt.Sprintf(QueryCreateDailyUsageTable, DailyUsageTableName)); err != nil {
		return nil, err
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
		{&db.updateSubscriberPolicyStmt, fmt.Sprintf(editSubscriberPolicyStmt, SubscribersTableName), []any{Subscriber{}}},
		{&db.updateSubscriberSqnNumStmt, fmt.Sprintf(editSubscriberSeqNumStmt, SubscribersTableName), []any{Subscriber{}}},
		{&db.deleteSubscriberStmt, fmt.Sprintf(deleteSubscriberStmt, SubscribersTableName), []any{Subscriber{}}},
		{&db.checkSubscriberIPStmt, fmt.Sprintf(checkIPStmt, SubscribersTableName), []any{Subscriber{}}},
		{&db.allocateSubscriberIPStmt, fmt.Sprintf(allocateIPStmt, SubscribersTableName), []any{Subscriber{}}},
		{&db.releaseSubscriberIPStmt, fmt.Sprintf(releaseIPStmt, SubscribersTableName), []any{Subscriber{}}},
		{&db.releaseAllIPStmt, fmt.Sprintf(releaseAllIPStmt, SubscribersTableName), nil},
		{&db.countSubscribersByPolicyStmt, fmt.Sprintf(countSubscribersInPolicyStmt, SubscribersTableName), []any{NumItems{}, Subscriber{}}},
		{&db.countSubscribersWithIPStmt, fmt.Sprintf(countSubscribersWithIPStmt, SubscribersTableName), []any{NumItems{}}},

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

		// Operator
		{&db.getOperatorStmt, fmt.Sprintf(getOperatorStmt, OperatorTableName), []any{Operator{}}},
		{&db.initializeOperatorStmt, fmt.Sprintf(initializeOperatorStmt, OperatorTableName), []any{Operator{}}},
		{&db.updateOperatorSliceStmt, fmt.Sprintf(updateOperatorSliceStmt, OperatorTableName), []any{Operator{}}},
		{&db.updateOperatorTrackingStmt, fmt.Sprintf(updateOperatorTrackingStmt, OperatorTableName), []any{Operator{}}},
		{&db.updateOperatorIDStmt, fmt.Sprintf(updateOperatorIDStmt, OperatorTableName), []any{Operator{}}},
		{&db.updateOperatorCodeStmt, fmt.Sprintf(updateOperatorCodeStmt, OperatorTableName), []any{Operator{}}},
		{&db.updateHomeNetworkPrivateKeyStmt, fmt.Sprintf(updateOperatorHomeNetworkPrivateKeyStmt, OperatorTableName), []any{Operator{}}},

		// Policies
		{&db.listPoliciesStmt, fmt.Sprintf(listPoliciesPagedStmt, PoliciesTableName), []any{ListArgs{}, Policy{}, NumItems{}}},
		{&db.getPolicyStmt, fmt.Sprintf(getPolicyStmt, PoliciesTableName), []any{Policy{}}},
		{&db.getPolicyByIDStmt, fmt.Sprintf(getPolicyByIDStmt, PoliciesTableName), []any{Policy{}}},
		{&db.createPolicyStmt, fmt.Sprintf(createPolicyStmt, PoliciesTableName), []any{Policy{}}},
		{&db.editPolicyStmt, fmt.Sprintf(editPolicyStmt, PoliciesTableName), []any{Policy{}}},
		{&db.deletePolicyStmt, fmt.Sprintf(deletePolicyStmt, PoliciesTableName), []any{Policy{}}},
		{&db.countPoliciesStmt, fmt.Sprintf(countPoliciesStmt, PoliciesTableName), []any{NumItems{}}},

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
		{&db.listAuditLogsStmt, fmt.Sprintf(listAuditLogsPageStmt, AuditLogsTableName), []any{ListArgs{}, dbwriter.AuditLog{}, NumItems{}}},
		{&db.deleteOldAuditLogsStmt, fmt.Sprintf(deleteOldAuditLogsStmt, AuditLogsTableName), []any{cutoffArgs{}}},
		{&db.countAuditLogsStmt, fmt.Sprintf(countAuditLogsStmt, AuditLogsTableName), []any{NumItems{}}},

		// Sessions
		{&db.createSessionStmt, fmt.Sprintf(createSessionStmt, SessionsTableName), []any{Session{}}},
		{&db.getSessionByTokenHashStmt, fmt.Sprintf(getSessionByTokenHashStmt, SessionsTableName), []any{Session{}}},
		{&db.deleteSessionByTokenHashStmt, fmt.Sprintf(deleteSessionByTokenHashStmt, SessionsTableName), []any{Session{}}},
		{&db.deleteExpiredSessionsStmt, fmt.Sprintf(deleteExpiredSessionsStmt, SessionsTableName), nil},
		{&db.countSessionsByUserStmt, fmt.Sprintf(countSessionsByUserStmt, SessionsTableName), []any{UserIDArgs{}, NumItems{}}},
		{&db.deleteOldestSessionsStmt, fmt.Sprintf(deleteOldestSessionsStmt, SessionsTableName, SessionsTableName), []any{DeleteOldestArgs{}}},

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

	err = db.InitializeN3Settings(ctx)
	if err != nil {
		return fmt.Errorf("failed to initialize N3 settings: %w", err)
	}

	if !db.IsOperatorInitialized(ctx) {
		initialOp, err := generateOperatorCode()
		if err != nil {
			return fmt.Errorf("couldn't generate operator code: %w", err)
		}

		initialHNPrivateKey, err := generateHomeNetworkPrivateKey()
		if err != nil {
			return fmt.Errorf("couldn't generate HN private key: %w", err)
		}

		initialOperator := &Operator{
			Mcc:                   InitialMcc,
			Mnc:                   InitialMnc,
			OperatorCode:          initialOp,
			Sst:                   InitialOperatorSst,
			Sd:                    InitialOperatorSd,
			HomeNetworkPrivateKey: initialHNPrivateKey,
		}

		err = initialOperator.SetSupportedTacs(InitialSupportedTacs)
		if err != nil {
			return fmt.Errorf("failed to set supported TACs: %w", err)
		}

		err = db.InitializeOperator(context.Background(), initialOperator)
		if err != nil {
			return fmt.Errorf("failed to initialize network configuration: %v", err)
		}
	}

	if !db.IsRetentionPolicyInitialized(context.Background(), CategoryAuditLogs) {
		initialPolicy := &RetentionPolicy{
			Category: CategoryAuditLogs,
			Days:     DefaultLogRetentionDays,
		}

		if err := db.SetRetentionPolicy(context.Background(), initialPolicy); err != nil {
			return fmt.Errorf("failed to initialize log retention policy: %v", err)
		}

		logger.WithTrace(ctx, logger.DBLog).Info("Initialized audit log retention policy", zap.Int("days", DefaultLogRetentionDays))
	}

	if !db.IsRetentionPolicyInitialized(context.Background(), CategoryRadioLogs) {
		initialPolicy := &RetentionPolicy{
			Category: CategoryRadioLogs,
			Days:     DefaultLogRetentionDays,
		}

		if err := db.SetRetentionPolicy(context.Background(), initialPolicy); err != nil {
			return fmt.Errorf("failed to initialize radio event retention policy: %v", err)
		}

		logger.WithTrace(ctx, logger.DBLog).Info("Initialized radio event retention policy", zap.Int("days", DefaultLogRetentionDays))
	}

	if !db.IsRetentionPolicyInitialized(context.Background(), CategorySubscriberUsage) {
		initialPolicy := &RetentionPolicy{
			Category: CategorySubscriberUsage,
			Days:     DefaultSubscriberUsageRetentionDays,
		}

		if err := db.SetRetentionPolicy(context.Background(), initialPolicy); err != nil {
			return fmt.Errorf("failed to initialize subscriber usage retention policy: %v", err)
		}

		logger.WithTrace(ctx, logger.DBLog).Info("Initialized subscriber usage retention policy", zap.Int("days", DefaultSubscriberUsageRetentionDays))
	}

	numDataNetworks, err := db.CountDataNetworks(context.Background())
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
		if err := db.CreateDataNetwork(context.Background(), initialDataNetwork); err != nil {
			return fmt.Errorf("failed to create default data network: %v", err)
		}

		dataNetwork, err := db.GetDataNetwork(context.Background(), InitialDataNetworkName)
		if err != nil {
			return fmt.Errorf("failed to get default data network: %v", err)
		}

		initialPolicy := &Policy{
			Name:            InitialPolicyName,
			BitrateUplink:   InitialPolicyBitrateUplink,
			BitrateDownlink: InitialPolicyBitrateDownlink,
			Var5qi:          InitialPolicyVar5qi,
			Arp:             InitialPolicyArp,
			DataNetworkID:   dataNetwork.ID,
		}

		if err := db.CreatePolicy(context.Background(), initialPolicy); err != nil {
			return fmt.Errorf("failed to create default policy: %v", err)
		}
	}

	return nil
}

func (db *Database) BeginTransaction() (*Transaction, error) {
	tx, err := db.conn.Begin(context.Background(), nil)
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
