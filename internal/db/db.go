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
	sqlConnection, err := sql.Open("sqlite3", databasePath)
	if err != nil {
		return nil, err
	}

	// turn on WAL journaling
	if _, err := sqlConnection.ExecContext(ctx, "PRAGMA journal_mode = WAL;"); err != nil {
		err := sqlConnection.Close()
		if err != nil {
			logger.DBLog.Error("Failed to close database connection after error", zap.Error(err))
		}

		return nil, fmt.Errorf("failed to enable WAL journaling: %w", err)
	}

	// turn synchronous to NORMAL for performance
	if _, err := sqlConnection.ExecContext(ctx, "PRAGMA synchronous = NORMAL;"); err != nil {
		err := sqlConnection.Close()
		if err != nil {
			logger.DBLog.Error("Failed to close database connection after error", zap.Error(err))
		}

		return nil, fmt.Errorf("failed to set synchronous to NORMAL: %w", err)
	}

	// turn on foreign key support
	if _, err := sqlConnection.ExecContext(ctx, "PRAGMA foreign_keys = ON;"); err != nil {
		err := sqlConnection.Close()
		if err != nil {
			logger.DBLog.Error("Failed to close database connection after error", zap.Error(err))
		}

		return nil, fmt.Errorf("failed to enable foreign key support: %w", err)
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

	err = db.Initialize(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	logger.DBLog.Debug("Database Initialized")

	return db, nil
}

func (db *Database) PrepareStatements() error {
	listSubscribersStmt, err := sqlair.Prepare(fmt.Sprintf(listSubscribersPagedStmt, SubscribersTableName), ListArgs{}, Subscriber{})
	if err != nil {
		return fmt.Errorf("failed to prepare list subscribers statement: %v", err)
	}

	countSubscribersStmt, err := sqlair.Prepare(fmt.Sprintf(countSubscribersStmt, SubscribersTableName), NumItems{})
	if err != nil {
		return fmt.Errorf("failed to prepare count subscribers statement: %v", err)
	}

	getSubscriberStmt, err := sqlair.Prepare(fmt.Sprintf(getSubscriberStmt, SubscribersTableName), Subscriber{})
	if err != nil {
		return fmt.Errorf("failed to prepare get subscriber statement: %v", err)
	}

	createSubscriberStmt, err := sqlair.Prepare(fmt.Sprintf(createSubscriberStmt, SubscribersTableName), Subscriber{})
	if err != nil {
		return fmt.Errorf("failed to prepare create subscriber statement: %v", err)
	}

	updateSubscriberPolicyStmt, err := sqlair.Prepare(fmt.Sprintf(editSubscriberPolicyStmt, SubscribersTableName), Subscriber{})
	if err != nil {
		return fmt.Errorf("failed to prepare update subscriber policy statement: %v", err)
	}

	updateSubscriberSqnNumStmt, err := sqlair.Prepare(fmt.Sprintf(editSubscriberSeqNumStmt, SubscribersTableName), Subscriber{})
	if err != nil {
		return fmt.Errorf("failed to prepare update subscriber SQN statement: %v", err)
	}

	deleteSubscriberStmt, err := sqlair.Prepare(fmt.Sprintf(deleteSubscriberStmt, SubscribersTableName), Subscriber{})
	if err != nil {
		return fmt.Errorf("failed to prepare delete subscriber statement: %v", err)
	}

	checkSubscriberIPStmt, err := sqlair.Prepare(fmt.Sprintf(checkIPStmt, SubscribersTableName), Subscriber{})
	if err != nil {
		return fmt.Errorf("failed to prepare IP check statement: %v", err)
	}

	allocateSubscriberIPStmt, err := sqlair.Prepare(fmt.Sprintf(allocateIPStmt, SubscribersTableName), Subscriber{})
	if err != nil {
		return fmt.Errorf("failed to prepare IP allocation statement: %v", err)
	}

	releaseSubscriberIPStmt, err := sqlair.Prepare(fmt.Sprintf(releaseIPStmt, SubscribersTableName), Subscriber{})
	if err != nil {
		return fmt.Errorf("failed to prepare IP release statement: %v", err)
	}

	countSubscribersByPolicyStmt, err := sqlair.Prepare(fmt.Sprintf(countSubscribersInPolicyStmt, SubscribersTableName), NumItems{}, Subscriber{})
	if err != nil {
		return fmt.Errorf("failed to prepare count subscribers by policy statement: %v", err)
	}

	countSubscribersWithIPStmt, err := sqlair.Prepare(fmt.Sprintf(countSubscribersWithIPStmt, SubscribersTableName), NumItems{})
	if err != nil {
		return fmt.Errorf("failed to prepare count subscribers with IP statement: %v", err)
	}

	listAPITokensStmt, err := sqlair.Prepare(fmt.Sprintf(listAPITokensPagedStmt, APITokensTableName), ListArgs{}, APIToken{})
	if err != nil {
		return fmt.Errorf("failed to prepare list API tokens statement: %v", err)
	}

	countAPITokensStmt, err := sqlair.Prepare(fmt.Sprintf(countAPITokensStmt, APITokensTableName), APIToken{}, NumItems{})
	if err != nil {
		return fmt.Errorf("failed to prepare count API tokens statement: %v", err)
	}

	createAPITokenStmt, err := sqlair.Prepare(fmt.Sprintf(createAPITokenStmt, APITokensTableName), APIToken{})
	if err != nil {
		return fmt.Errorf("failed to prepare create API token statement: %v", err)
	}

	getAPITokenByNameStmt, err := sqlair.Prepare(fmt.Sprintf(getByNameStmt, APITokensTableName), APIToken{})
	if err != nil {
		return fmt.Errorf("failed to prepare get API token by name statement: %v", err)
	}

	deleteAPITokenStmt, err := sqlair.Prepare(fmt.Sprintf(deleteAPITokenStmt, APITokensTableName), APIToken{})
	if err != nil {
		return fmt.Errorf("failed to prepare delete API token statement: %v", err)
	}

	getAPITokenByIDStmt, err := sqlair.Prepare(fmt.Sprintf(getByTokenIDStmt, APITokensTableName), APIToken{})
	if err != nil {
		return fmt.Errorf("failed to prepare get API token by ID statement: %v", err)
	}

	insertRadioEventStmt, err := sqlair.Prepare(fmt.Sprintf(insertRadioEventStmt, RadioEventsTableName), dbwriter.RadioEvent{})
	if err != nil {
		return fmt.Errorf("failed to prepare insert radio event statement: %v", err)
	}

	listRadioEventsStmt, err := sqlair.Prepare(fmt.Sprintf(listRadioEventsPagedFilteredStmt, RadioEventsTableName), ListArgs{}, RadioEventFilters{}, dbwriter.RadioEvent{})
	if err != nil {
		return fmt.Errorf("failed to prepare list radio events statement: %v", err)
	}

	countRadioEventsStmt, err := sqlair.Prepare(fmt.Sprintf(countRadioEventsFilteredStmt, RadioEventsTableName), RadioEventFilters{}, NumItems{})
	if err != nil {
		return fmt.Errorf("failed to prepare count radio events statement: %v", err)
	}

	deleteOldRadioEventsStmt, err := sqlair.Prepare(fmt.Sprintf(deleteOldRadioEventsStmt, RadioEventsTableName), cutoffArgs{})
	if err != nil {
		return fmt.Errorf("failed to prepare delete old radio events statement: %v", err)
	}

	deleteAllRadioEventsStmt, err := sqlair.Prepare(fmt.Sprintf(deleteAllRadioEventsStmt, RadioEventsTableName))
	if err != nil {
		return fmt.Errorf("failed to prepare delete all radio events statement: %v", err)
	}

	getRadioEventByIDStmt, err := sqlair.Prepare(fmt.Sprintf(getRadioEventByIDStmt, RadioEventsTableName), dbwriter.RadioEvent{})
	if err != nil {
		return fmt.Errorf("failed to prepare get radio event by ID statement: %v", err)
	}

	incrementDailyUsageStmt, err := sqlair.Prepare(fmt.Sprintf(incrementDailyUsageStmt, DailyUsageTableName), DailyUsage{})
	if err != nil {
		return fmt.Errorf("failed to prepare increment daily usage statement: %v", err)
	}

	getUsagePerDayStmt, err := sqlair.Prepare(fmt.Sprintf(getUsagePerDayStmt, DailyUsageTableName), UsageFilters{}, UsagePerDay{})
	if err != nil {
		return fmt.Errorf("couldn't prepare statement: %w", err)
	}

	getUsagePerSubscriberStmt, err := sqlair.Prepare(fmt.Sprintf(getUsagePerSubscriberStmt, DailyUsageTableName), UsageFilters{}, UsagePerSub{})
	if err != nil {
		return fmt.Errorf("couldn't prepare statement: %w", err)
	}

	deleteAllDailyUsageStmt, err := sqlair.Prepare(fmt.Sprintf(deleteAllDailyUsageStmt, DailyUsageTableName))
	if err != nil {
		return fmt.Errorf("failed to prepare delete all daily usage statement: %v", err)
	}

	deleteOldDailyUsageStmt, err := sqlair.Prepare(fmt.Sprintf(deleteOldDailyUsageStmt, DailyUsageTableName), cutoffDaysArgs{})
	if err != nil {
		return fmt.Errorf("failed to prepare delete old daily usage statement: %v", err)
	}

	listDataNetworksStmt, err := sqlair.Prepare(fmt.Sprintf(listDataNetworksPagedStmt, DataNetworksTableName), ListArgs{}, DataNetwork{})
	if err != nil {
		return fmt.Errorf("failed to prepare list data networks statement: %v", err)
	}

	getDataNetworkStmt, err := sqlair.Prepare(fmt.Sprintf(getDataNetworkStmt, DataNetworksTableName), DataNetwork{})
	if err != nil {
		return fmt.Errorf("failed to prepare get data network statement: %v", err)
	}

	getDataNetworkByIDStmt, err := sqlair.Prepare(fmt.Sprintf(getDataNetworkByIDStmt, DataNetworksTableName), DataNetwork{})
	if err != nil {
		return fmt.Errorf("failed to prepare get data network by ID statement: %v", err)
	}

	createDataNetworkStmt, err := sqlair.Prepare(fmt.Sprintf(createDataNetworkStmt, DataNetworksTableName), DataNetwork{})
	if err != nil {
		return fmt.Errorf("failed to prepare create data network statement: %v", err)
	}

	editDataNetworkStmt, err := sqlair.Prepare(fmt.Sprintf(editDataNetworkStmt, DataNetworksTableName), DataNetwork{})
	if err != nil {
		return fmt.Errorf("failed to prepare update data network statement: %v", err)
	}

	deleteDataNetworkStmt, err := sqlair.Prepare(fmt.Sprintf(deleteDataNetworkStmt, DataNetworksTableName), DataNetwork{})
	if err != nil {
		return fmt.Errorf("failed to prepare delete data network statement: %v", err)
	}

	countDataNetworksStmt, err := sqlair.Prepare(fmt.Sprintf(countDataNetworksStmt, DataNetworksTableName), NumItems{})
	if err != nil {
		return fmt.Errorf("failed to prepare count data networks statement: %v", err)
	}

	insertDefaultN3SettingsStmt, err := sqlair.Prepare(fmt.Sprintf(insertDefaultN3SettingsStmt, N3SettingsTableName), N3Settings{})
	if err != nil {
		return fmt.Errorf("failed to prepare insert default N3 settings statement: %w", err)
	}

	updateN3SettingsStmt, err := sqlair.Prepare(fmt.Sprintf(upsertN3SettingsStmt, N3SettingsTableName), N3Settings{})
	if err != nil {
		return fmt.Errorf("failed to prepare upsert N3 settings statement: %w", err)
	}

	getN3SettingsStmt, err := sqlair.Prepare(fmt.Sprintf(getN3SettingsStmt, N3SettingsTableName), N3Settings{})
	if err != nil {
		return fmt.Errorf("failed to prepare get N3 settings statement: %w", err)
	}

	insertDefaultNATSettingsStmt, err := sqlair.Prepare(fmt.Sprintf(insertDefaultNATSettingsStmt, NATSettingsTableName), NATSettings{})
	if err != nil {
		return fmt.Errorf("failed to prepare insert default NAT settings statement: %w", err)
	}

	getNATSettingsStmt, err := sqlair.Prepare(fmt.Sprintf(getNATSettingsStmt, NATSettingsTableName), NATSettings{})
	if err != nil {
		return fmt.Errorf("failed to prepare get NAT settings statement: %w", err)
	}

	upsertNATSettingsStmt, err := sqlair.Prepare(fmt.Sprintf(upsertNATSettingsStmt, NATSettingsTableName), NATSettings{})
	if err != nil {
		return fmt.Errorf("failed to prepare upsert NAT settings statement: %w", err)
	}

	getOperatorSettingsStmt, err := sqlair.Prepare(fmt.Sprintf(getOperatorStmt, OperatorTableName), Operator{})
	if err != nil {
		return fmt.Errorf("failed to prepare get operator statement: %v", err)
	}

	initializeOperatorStmt, err := sqlair.Prepare(fmt.Sprintf(initializeOperatorStmt, OperatorTableName), Operator{})
	if err != nil {
		return fmt.Errorf("failed to prepare initialize operator configuration statement: %w", err)
	}

	updateOperatorSliceStmt, err := sqlair.Prepare(fmt.Sprintf(updateOperatorSliceStmt, OperatorTableName), Operator{})
	if err != nil {
		return fmt.Errorf("failed to prepare update operator slice statement: %w", err)
	}

	updateOperatorTrackingStmt, err := sqlair.Prepare(fmt.Sprintf(updateOperatorTrackingStmt, OperatorTableName), Operator{})
	if err != nil {
		return fmt.Errorf("failed to prepare update operator tracking statement: %w", err)
	}

	updateOperatorIDStmt, err := sqlair.Prepare(fmt.Sprintf(updateOperatorIDStmt, OperatorTableName), Operator{})
	if err != nil {
		return fmt.Errorf("failed to prepare update operator ID statement: %w", err)
	}

	updateOperatorCodeStmt, err := sqlair.Prepare(fmt.Sprintf(updateOperatorCodeStmt, OperatorTableName), Operator{})
	if err != nil {
		return fmt.Errorf("failed to prepare update operator code statement: %w", err)
	}

	updateHomeNetworkPrivateKeyStmt, err := sqlair.Prepare(fmt.Sprintf(updateOperatorHomeNetworkPrivateKeyStmt, OperatorTableName), Operator{})
	if err != nil {
		return fmt.Errorf("failed to prepare update operator HN private key statement: %w", err)
	}

	listPoliciesStmt, err := sqlair.Prepare(fmt.Sprintf(listPoliciesPagedStmt, PoliciesTableName), ListArgs{}, Policy{})
	if err != nil {
		return fmt.Errorf("failed to prepare list policies statement: %v", err)
	}

	getPolicyStmt, err := sqlair.Prepare(fmt.Sprintf(getPolicyStmt, PoliciesTableName), Policy{})
	if err != nil {
		return fmt.Errorf("failed to prepare get policy statement: %v", err)
	}

	getPolicyByIDStmt, err := sqlair.Prepare(fmt.Sprintf(getPolicyByIDStmt, PoliciesTableName), Policy{})
	if err != nil {
		return fmt.Errorf("failed to prepare get policy by ID statement: %v", err)
	}

	createPolicyStmt, err := sqlair.Prepare(fmt.Sprintf(createPolicyStmt, PoliciesTableName), Policy{})
	if err != nil {
		return fmt.Errorf("failed to prepare create policy statement: %v", err)
	}

	editPolicyStmt, err := sqlair.Prepare(fmt.Sprintf(editPolicyStmt, PoliciesTableName), Policy{})
	if err != nil {
		return fmt.Errorf("failed to prepare update policy statement: %v", err)
	}

	deletePolicyStmt, err := sqlair.Prepare(fmt.Sprintf(deletePolicyStmt, PoliciesTableName), Policy{})
	if err != nil {
		return fmt.Errorf("failed to prepare delete policy statement: %v", err)
	}

	countPoliciesStmt, err := sqlair.Prepare(fmt.Sprintf(countPoliciesStmt, PoliciesTableName), NumItems{})
	if err != nil {
		return fmt.Errorf("failed to prepare count policies statement: %v", err)
	}

	selectRetentionPolicyStmt, err := sqlair.Prepare(fmt.Sprintf(selectRetentionPolicyStmt, RetentionPolicyTableName), RetentionPolicy{})
	if err != nil {
		return fmt.Errorf("failed to prepare select retention policy statement: %v", err)
	}

	upsertRetentionPolicyStmt, err := sqlair.Prepare(fmt.Sprintf(upsertRetentionPolicyStmt, RetentionPolicyTableName), RetentionPolicy{})
	if err != nil {
		return fmt.Errorf("failed to prepare upsert retention policy statement: %v", err)
	}

	listRoutesStmt, err := sqlair.Prepare(fmt.Sprintf(listRoutesPageStmt, RoutesTableName), ListArgs{}, Route{})
	if err != nil {
		return fmt.Errorf("failed to prepare list routes statement: %v", err)
	}

	getRouteStmt, err := sqlair.Prepare(fmt.Sprintf(getRouteStmt, RoutesTableName), Route{})
	if err != nil {
		return fmt.Errorf("failed to prepare get route statement: %v", err)
	}

	createRouteStmt, err := sqlair.Prepare(fmt.Sprintf(createRouteStmt, RoutesTableName), Route{})
	if err != nil {
		return fmt.Errorf("failed to prepare create route statement: %v", err)
	}

	deleteRouteStmt, err := sqlair.Prepare(fmt.Sprintf(deleteRouteStmt, RoutesTableName), Route{})
	if err != nil {
		return fmt.Errorf("failed to prepare delete route statement: %v", err)
	}

	countRoutesStmt, err := sqlair.Prepare(fmt.Sprintf(countRoutesStmt, RoutesTableName), NumItems{})
	if err != nil {
		return fmt.Errorf("failed to prepare count routes statement: %v", err)
	}

	insertAuditLogStmt, err := sqlair.Prepare(fmt.Sprintf(insertAuditLogStmt, AuditLogsTableName), dbwriter.AuditLog{})
	if err != nil {
		return fmt.Errorf("failed to prepare insert audit log statement: %v", err)
	}

	listAuditLogsStmt, err := sqlair.Prepare(fmt.Sprintf(listAuditLogsPageStmt, AuditLogsTableName), ListArgs{}, dbwriter.AuditLog{})
	if err != nil {
		return fmt.Errorf("failed to prepare list audit logs statement: %v", err)
	}

	deleteOldAuditLogsStmt, err := sqlair.Prepare(fmt.Sprintf(deleteOldAuditLogsStmt, AuditLogsTableName), cutoffArgs{})
	if err != nil {
		return fmt.Errorf("failed to prepare delete old audit logs statement: %v", err)
	}

	countAuditLogsStmt, err := sqlair.Prepare(fmt.Sprintf(countAuditLogsStmt, AuditLogsTableName), NumItems{})
	if err != nil {
		return fmt.Errorf("failed to prepare count audit logs statement: %v", err)
	}

	createSessionStmt, err := sqlair.Prepare(fmt.Sprintf(createSessionStmt, SessionsTableName), Session{})
	if err != nil {
		return fmt.Errorf("failed to prepare create session statement: %v", err)
	}

	getSessionByTokenHashStmt, err := sqlair.Prepare(fmt.Sprintf(getSessionByTokenHashStmt, SessionsTableName), Session{})
	if err != nil {
		return fmt.Errorf("failed to prepare get session by token hash statement: %v", err)
	}

	deleteSessionByTokenHashStmt, err := sqlair.Prepare(fmt.Sprintf(deleteSessionByTokenHashStmt, SessionsTableName), Session{})
	if err != nil {
		return fmt.Errorf("failed to prepare delete session by token hash statement: %v", err)
	}

	deleteExpiredSessionsStmt, err := sqlair.Prepare(fmt.Sprintf(deleteExpiredSessionsStmt, SessionsTableName))
	if err != nil {
		return fmt.Errorf("failed to prepare delete expired sessions statement: %v", err)
	}

	countSessionsByUserStmt, err := sqlair.Prepare(fmt.Sprintf(countSessionsByUserStmt, SessionsTableName), UserIDArgs{}, NumItems{})
	if err != nil {
		return fmt.Errorf("failed to prepare count sessions by user statement: %v", err)
	}

	deleteOldestSessionsStmt, err := sqlair.Prepare(fmt.Sprintf(deleteOldestSessionsStmt, SessionsTableName, SessionsTableName), DeleteOldestArgs{})
	if err != nil {
		return fmt.Errorf("failed to prepare delete oldest sessions statement: %v", err)
	}

	listUsersStmt, err := sqlair.Prepare(fmt.Sprintf(listUsersPageStmt, UsersTableName), ListArgs{}, User{})
	if err != nil {
		return fmt.Errorf("failed to prepare list users statement: %v", err)
	}

	getUserStmt, err := sqlair.Prepare(fmt.Sprintf(getUserStmt, UsersTableName), User{})
	if err != nil {
		return fmt.Errorf("failed to prepare get user statement: %v", err)
	}

	getUserByIDStmt, err := sqlair.Prepare(fmt.Sprintf(getUserByIDStmt, UsersTableName), User{})
	if err != nil {
		return fmt.Errorf("failed to prepare get user by ID statement: %v", err)
	}

	createUserStmt, err := sqlair.Prepare(fmt.Sprintf(createUserStmt, UsersTableName), User{})
	if err != nil {
		return fmt.Errorf("failed to prepare create user statement: %v", err)
	}

	editUserStmt, err := sqlair.Prepare(fmt.Sprintf(editUserStmt, UsersTableName), User{})
	if err != nil {
		return fmt.Errorf("failed to prepare edit user statement: %v", err)
	}

	editUserPasswordStmt, err := sqlair.Prepare(fmt.Sprintf(editUserPasswordStmt, UsersTableName), User{})
	if err != nil {
		return fmt.Errorf("failed to prepare edit user password statement: %v", err)
	}

	deleteUserStmt, err := sqlair.Prepare(fmt.Sprintf(deleteUserStmt, UsersTableName), User{})
	if err != nil {
		return fmt.Errorf("failed to prepare delete user statement: %v", err)
	}

	countUsersStmt, err := sqlair.Prepare(fmt.Sprintf(countUsersStmt, UsersTableName), NumItems{})
	if err != nil {
		return fmt.Errorf("failed to prepare count users statement: %v", err)
	}

	db.listSubscribersStmt = listSubscribersStmt
	db.countSubscribersStmt = countSubscribersStmt
	db.getSubscriberStmt = getSubscriberStmt
	db.createSubscriberStmt = createSubscriberStmt
	db.updateSubscriberPolicyStmt = updateSubscriberPolicyStmt
	db.updateSubscriberSqnNumStmt = updateSubscriberSqnNumStmt
	db.deleteSubscriberStmt = deleteSubscriberStmt
	db.checkSubscriberIPStmt = checkSubscriberIPStmt
	db.allocateSubscriberIPStmt = allocateSubscriberIPStmt
	db.releaseSubscriberIPStmt = releaseSubscriberIPStmt
	db.countSubscribersByPolicyStmt = countSubscribersByPolicyStmt
	db.countSubscribersWithIPStmt = countSubscribersWithIPStmt

	db.listAPITokensStmt = listAPITokensStmt
	db.countAPITokensStmt = countAPITokensStmt
	db.createAPITokenStmt = createAPITokenStmt
	db.getAPITokenByNameStmt = getAPITokenByNameStmt
	db.deleteAPITokenStmt = deleteAPITokenStmt
	db.getAPITokenByIDStmt = getAPITokenByIDStmt

	db.insertRadioEventStmt = insertRadioEventStmt
	db.listRadioEventsStmt = listRadioEventsStmt
	db.countRadioEventsStmt = countRadioEventsStmt
	db.deleteOldRadioEventsStmt = deleteOldRadioEventsStmt
	db.deleteAllRadioEventsStmt = deleteAllRadioEventsStmt
	db.getRadioEventByIDStmt = getRadioEventByIDStmt

	db.incrementDailyUsageStmt = incrementDailyUsageStmt
	db.getUsagePerDayStmt = getUsagePerDayStmt
	db.getUsagePerSubscriberStmt = getUsagePerSubscriberStmt
	db.deleteAllDailyUsageStmt = deleteAllDailyUsageStmt
	db.deleteOldDailyUsageStmt = deleteOldDailyUsageStmt

	db.listDataNetworksStmt = listDataNetworksStmt
	db.getDataNetworkStmt = getDataNetworkStmt
	db.getDataNetworkByIDStmt = getDataNetworkByIDStmt
	db.createDataNetworkStmt = createDataNetworkStmt
	db.editDataNetworkStmt = editDataNetworkStmt
	db.deleteDataNetworkStmt = deleteDataNetworkStmt
	db.countDataNetworksStmt = countDataNetworksStmt

	db.insertDefaultN3SettingsStmt = insertDefaultN3SettingsStmt
	db.updateN3SettingsStmt = updateN3SettingsStmt
	db.getN3SettingsStmt = getN3SettingsStmt

	db.insertDefaultNATSettingsStmt = insertDefaultNATSettingsStmt
	db.getNATSettingsStmt = getNATSettingsStmt
	db.upsertNATSettingsStmt = upsertNATSettingsStmt

	db.getOperatorStmt = getOperatorSettingsStmt
	db.initializeOperatorStmt = initializeOperatorStmt
	db.updateOperatorSliceStmt = updateOperatorSliceStmt
	db.updateOperatorTrackingStmt = updateOperatorTrackingStmt
	db.updateOperatorIDStmt = updateOperatorIDStmt
	db.updateOperatorCodeStmt = updateOperatorCodeStmt
	db.updateHomeNetworkPrivateKeyStmt = updateHomeNetworkPrivateKeyStmt

	db.listPoliciesStmt = listPoliciesStmt
	db.getPolicyStmt = getPolicyStmt
	db.getPolicyByIDStmt = getPolicyByIDStmt
	db.createPolicyStmt = createPolicyStmt
	db.editPolicyStmt = editPolicyStmt
	db.deletePolicyStmt = deletePolicyStmt
	db.countPoliciesStmt = countPoliciesStmt

	db.selectRetentionPolicyStmt = selectRetentionPolicyStmt
	db.upsertRetentionPolicyStmt = upsertRetentionPolicyStmt

	db.listRoutesStmt = listRoutesStmt
	db.getRouteStmt = getRouteStmt
	db.createRouteStmt = createRouteStmt
	db.deleteRouteStmt = deleteRouteStmt
	db.countRoutesStmt = countRoutesStmt

	db.insertAuditLogStmt = insertAuditLogStmt
	db.listAuditLogsStmt = listAuditLogsStmt
	db.deleteOldAuditLogsStmt = deleteOldAuditLogsStmt
	db.countAuditLogsStmt = countAuditLogsStmt

	db.createSessionStmt = createSessionStmt
	db.getSessionByTokenHashStmt = getSessionByTokenHashStmt
	db.deleteSessionByTokenHashStmt = deleteSessionByTokenHashStmt
	db.deleteExpiredSessionsStmt = deleteExpiredSessionsStmt
	db.countSessionsByUserStmt = countSessionsByUserStmt
	db.deleteOldestSessionsStmt = deleteOldestSessionsStmt

	db.listUsersStmt = listUsersStmt
	db.getUserStmt = getUserStmt
	db.getUserByIDStmt = getUserByIDStmt
	db.createUserStmt = createUserStmt
	db.editUserStmt = editUserStmt
	db.editUserPasswordStmt = editUserPasswordStmt
	db.deleteUserStmt = deleteUserStmt
	db.countUsersStmt = countUsersStmt

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

		logger.DBLog.Info("Initialized audit log retention policy", zap.Int("days", DefaultLogRetentionDays))
	}

	if !db.IsRetentionPolicyInitialized(context.Background(), CategoryRadioLogs) {
		initialPolicy := &RetentionPolicy{
			Category: CategoryRadioLogs,
			Days:     DefaultLogRetentionDays,
		}

		if err := db.SetRetentionPolicy(context.Background(), initialPolicy); err != nil {
			return fmt.Errorf("failed to initialize radio event retention policy: %v", err)
		}

		logger.DBLog.Info("Initialized radio event retention policy", zap.Int("days", DefaultLogRetentionDays))
	}

	if !db.IsRetentionPolicyInitialized(context.Background(), CategorySubscriberUsage) {
		initialPolicy := &RetentionPolicy{
			Category: CategorySubscriberUsage,
			Days:     DefaultSubscriberUsageRetentionDays,
		}

		if err := db.SetRetentionPolicy(context.Background(), initialPolicy); err != nil {
			return fmt.Errorf("failed to initialize subscriber usage retention policy: %v", err)
		}

		logger.DBLog.Info("Initialized subscriber usage retention policy", zap.Int("days", DefaultSubscriberUsageRetentionDays))
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
