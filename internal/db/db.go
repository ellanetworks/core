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

	policiesTable          string
	routesTable            string
	operatorTable          string
	dataNetworksTable      string
	usersTable             string
	auditLogsTable         string
	networkLogsTable       string
	retentionPoliciesTable string
	apiTokensTable         string
	sessionsTable          string
	natSettingsTable       string
	n3SettingsTable        string
	dailyUsageTable        string

	// Subscriber statements
	listSubscribersStmt          *sqlair.Statement
	countSubscribersStmt         *sqlair.Statement
	getSubscriberStmt            *sqlair.Statement
	createSubscriberStmt         *sqlair.Statement
	updateSubscriberStmt         *sqlair.Statement
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
func NewDatabase(databasePath string) (*Database, error) {
	sqlConnection, err := sql.Open("sqlite3", databasePath)
	if err != nil {
		return nil, err
	}

	// turn on WAL journaling
	if _, err := sqlConnection.Exec("PRAGMA journal_mode = WAL;"); err != nil {
		err := sqlConnection.Close()
		if err != nil {
			logger.DBLog.Error("Failed to close database connection after error", zap.Error(err))
		}
		return nil, fmt.Errorf("failed to enable WAL journaling: %w", err)
	}

	// Initialize tables
	if _, err := sqlConnection.Exec(fmt.Sprintf(QueryCreateSubscribersTable, SubscribersTableName)); err != nil {
		return nil, err
	}
	if _, err := sqlConnection.Exec(fmt.Sprintf(QueryCreatePoliciesTable, PoliciesTableName)); err != nil {
		return nil, err
	}
	if _, err := sqlConnection.Exec(fmt.Sprintf(QueryCreateRoutesTable, RoutesTableName)); err != nil {
		return nil, err
	}
	if _, err := sqlConnection.Exec(fmt.Sprintf(QueryCreateOperatorTable, OperatorTableName)); err != nil {
		return nil, err
	}
	if _, err := sqlConnection.Exec(fmt.Sprintf(QueryCreateDataNetworksTable, DataNetworksTableName)); err != nil {
		return nil, err
	}
	if _, err := sqlConnection.Exec(fmt.Sprintf(QueryCreateUsersTable, UsersTableName)); err != nil {
		return nil, err
	}
	if _, err := sqlConnection.Exec(createSessionsTableSQL); err != nil {
		return nil, err
	}
	if _, err := sqlConnection.Exec(fmt.Sprintf(QueryCreateAuditLogsTable, AuditLogsTableName)); err != nil {
		return nil, err
	}
	if _, err := sqlConnection.Exec(fmt.Sprintf(QueryCreateRadioEventsTable, RadioEventsTableName)); err != nil {
		return nil, err
	}
	if _, err := sqlConnection.Exec(QueryCreateRadioEventsIndex); err != nil {
		return nil, err
	}
	if _, err := sqlConnection.Exec(fmt.Sprintf(QueryCreateRetentionPolicyTable, RetentionPolicyTableName)); err != nil {
		return nil, err
	}
	if _, err := sqlConnection.Exec(fmt.Sprintf(QueryCreateAPITokensTable, APITokensTableName)); err != nil {
		return nil, err
	}
	if _, err := sqlConnection.Exec(fmt.Sprintf(QueryCreateNATSettingsTable, NATSettingsTableName)); err != nil {
		return nil, err
	}
	if _, err := sqlConnection.Exec(fmt.Sprintf(QueryCreateN3SettingsTable, N3SettingsTableName)); err != nil {
		return nil, err
	}
	if _, err := sqlConnection.Exec(fmt.Sprintf(QueryCreateDailyUsageTable, DailyUsageTableName)); err != nil {
		return nil, err
	}

	db := new(Database)
	db.conn = sqlair.NewDB(sqlConnection)
	db.filepath = databasePath
	db.policiesTable = PoliciesTableName
	db.routesTable = RoutesTableName
	db.operatorTable = OperatorTableName
	db.dataNetworksTable = DataNetworksTableName
	db.usersTable = UsersTableName
	db.auditLogsTable = AuditLogsTableName
	db.retentionPoliciesTable = RetentionPolicyTableName
	db.apiTokensTable = APITokensTableName
	db.sessionsTable = SessionsTableName
	db.natSettingsTable = NATSettingsTableName
	db.n3SettingsTable = N3SettingsTableName
	db.networkLogsTable = RadioEventsTableName
	db.dailyUsageTable = DailyUsageTableName

	err = db.PrepareStatements()
	if err != nil {
		return nil, fmt.Errorf("failed to prepare statements: %w", err)
	}

	err = db.Initialize()
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

	updateSubscriberStmt, err := sqlair.Prepare(fmt.Sprintf(editSubscriberStmt, SubscribersTableName), Subscriber{})
	if err != nil {
		return fmt.Errorf("failed to prepare update subscriber statement: %v", err)
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

	listAPITokensStmt, err := sqlair.Prepare(fmt.Sprintf(listAPITokensPagedStmt, db.apiTokensTable), ListArgs{}, APIToken{})
	if err != nil {
		return fmt.Errorf("failed to prepare list API tokens statement: %v", err)
	}

	countAPITokensStmt, err := sqlair.Prepare(fmt.Sprintf(countAPITokensStmt, db.apiTokensTable), APIToken{}, NumItems{})
	if err != nil {
		return fmt.Errorf("failed to prepare count API tokens statement: %v", err)
	}

	createAPITokenStmt, err := sqlair.Prepare(fmt.Sprintf(createAPITokenStmt, db.apiTokensTable), APIToken{})
	if err != nil {
		return fmt.Errorf("failed to prepare create API token statement: %v", err)
	}

	getAPITokenByNameStmt, err := sqlair.Prepare(fmt.Sprintf(getByNameStmt, db.apiTokensTable), APIToken{})
	if err != nil {
		return fmt.Errorf("failed to prepare get API token by name statement: %v", err)
	}

	deleteAPITokenStmt, err := sqlair.Prepare(fmt.Sprintf(deleteAPITokenStmt, db.apiTokensTable), APIToken{})
	if err != nil {
		return fmt.Errorf("failed to prepare delete API token statement: %v", err)
	}

	getAPITokenByIDStmt, err := sqlair.Prepare(fmt.Sprintf(getByTokenIDStmt, db.apiTokensTable), APIToken{})
	if err != nil {
		return fmt.Errorf("failed to prepare get API token by ID statement: %v", err)
	}

	insertRadioEventStmt, err := sqlair.Prepare(fmt.Sprintf(insertRadioEventStmt, db.networkLogsTable), dbwriter.RadioEvent{})
	if err != nil {
		return fmt.Errorf("failed to prepare insert radio event statement: %v", err)
	}

	listRadioEventsStmt, err := sqlair.Prepare(fmt.Sprintf(listRadioEventsPagedFilteredStmt, db.networkLogsTable), ListArgs{}, RadioEventFilters{}, dbwriter.RadioEvent{})
	if err != nil {
		return fmt.Errorf("failed to prepare list radio events statement: %v", err)
	}

	countRadioEventsStmt, err := sqlair.Prepare(fmt.Sprintf(countRadioEventsFilteredStmt, db.networkLogsTable), RadioEventFilters{}, NumItems{})
	if err != nil {
		return fmt.Errorf("failed to prepare count radio events statement: %v", err)
	}

	deleteOldRadioEventsStmt, err := sqlair.Prepare(fmt.Sprintf(deleteOldRadioEventsStmt, db.networkLogsTable), cutoffArgs{})
	if err != nil {
		return fmt.Errorf("failed to prepare delete old radio events statement: %v", err)
	}

	deleteAllRadioEventsStmt, err := sqlair.Prepare(fmt.Sprintf(deleteAllRadioEventsStmt, db.networkLogsTable))
	if err != nil {
		return fmt.Errorf("failed to prepare delete all radio events statement: %v", err)
	}

	getRadioEventByIDStmt, err := sqlair.Prepare(fmt.Sprintf(getRadioEventByIDStmt, db.networkLogsTable), dbwriter.RadioEvent{})
	if err != nil {
		return fmt.Errorf("failed to prepare get radio event by ID statement: %v", err)
	}

	db.listSubscribersStmt = listSubscribersStmt
	db.countSubscribersStmt = countSubscribersStmt
	db.getSubscriberStmt = getSubscriberStmt
	db.createSubscriberStmt = createSubscriberStmt
	db.updateSubscriberStmt = updateSubscriberStmt
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

	return nil
}

func (db *Database) Initialize() error {
	err := db.InitializeNATSettings(context.Background())
	if err != nil {
		return fmt.Errorf("failed to initialize NAT settings: %w", err)
	}

	err = db.InitializeN3Settings(context.Background())
	if err != nil {
		return fmt.Errorf("failed to initialize N3 settings: %w", err)
	}

	if !db.IsOperatorInitialized() {
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
		initialOperator.SetSupportedTacs(InitialSupportedTacs)
		if err := db.InitializeOperator(context.Background(), initialOperator); err != nil {
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
