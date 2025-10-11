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
	"github.com/ellanetworks/core/internal/logger"
	_ "github.com/mattn/go-sqlite3"
	"go.opentelemetry.io/otel"
	"go.uber.org/zap"
)

var tracer = otel.Tracer("ella-core/db")

// Database is the object used to communicate with the established repository.
type Database struct {
	filepath               string
	subscribersTable       string
	policiesTable          string
	routesTable            string
	operatorTable          string
	dataNetworksTable      string
	usersTable             string
	auditLogsTable         string
	subscriberLogsTable    string
	radioLogsTable         string
	retentionPoliciesTable string
	apiTokensTable         string
	sessionsTable          string
	natSettingsTable       string
	conn                   *sqlair.DB
}

// Initial Log Retention Policy values
const DefaultLogRetentionDays = 7

// Initial operator values
const (
	InitialMcc         = "001"
	InitialMnc         = "01"
	InitialOperatorSst = 1
)

var InitialSupportedTacs = []string{"001"}

// Initial Data network values
const (
	InitialDataNetworkName   = "internet"
	InitialDataNetworkIPPool = "10.45.0.0/16"
	InitialDataNetworkDNS    = "8.8.8.8"
	InitialDataNetworkMTU    = 1400
)

// Initial Policy values
const (
	InitialPolicyName            = "default"
	InitialPolicyBitrateUplink   = "200 Mbps"
	InitialPolicyBitrateDownlink = "200 Mbps"
	InitialPolicyVar5qi          = 9  // Default 5QI for non-GBR
	InitialPolicyPriorityLevel   = 90 // Default priority level 5QI of 9
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
	if _, err := sqlConnection.Exec(fmt.Sprintf(QueryCreateSubscriberLogsTable, SubscriberLogsTableName)); err != nil {
		return nil, err
	}
	if _, err := sqlConnection.Exec(QueryCreateSubscriberLogsIndex); err != nil {
		return nil, err
	}
	if _, err := sqlConnection.Exec(fmt.Sprintf(QueryCreateRadioLogsTable, RadioLogsTableName)); err != nil {
		return nil, err
	}
	if _, err := sqlConnection.Exec(QueryCreateRadioLogsIndex); err != nil {
		return nil, err
	}
	if _, err := sqlConnection.Exec(fmt.Sprintf(QueryCreateLogRetentionPolicyTable, LogRetentionPolicyTableName)); err != nil {
		return nil, err
	}
	if _, err := sqlConnection.Exec(fmt.Sprintf(QueryCreateAPITokensTable, APITokensTableName)); err != nil {
		return nil, err
	}
	if _, err := sqlConnection.Exec(fmt.Sprintf(QueryCreateNATSettingsTable, NATSettingsTableName)); err != nil {
		return nil, err
	}

	db := new(Database)
	db.conn = sqlair.NewDB(sqlConnection)
	db.filepath = databasePath
	db.subscribersTable = SubscribersTableName
	db.policiesTable = PoliciesTableName
	db.routesTable = RoutesTableName
	db.operatorTable = OperatorTableName
	db.dataNetworksTable = DataNetworksTableName
	db.usersTable = UsersTableName
	db.auditLogsTable = AuditLogsTableName
	db.subscriberLogsTable = SubscriberLogsTableName
	db.radioLogsTable = RadioLogsTableName
	db.retentionPoliciesTable = LogRetentionPolicyTableName
	db.apiTokensTable = APITokensTableName
	db.sessionsTable = SessionsTableName
	db.natSettingsTable = NATSettingsTableName

	err = db.Initialize()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	logger.DBLog.Info("Database Initialized")

	return db, nil
}

func (db *Database) Initialize() error {
	err := db.InitializeNATSettings(context.Background())
	if err != nil {
		return fmt.Errorf("failed to initialize NAT settings: %w", err)
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
			Sd:                    nil,
			HomeNetworkPrivateKey: initialHNPrivateKey,
		}
		initialOperator.SetSupportedTacs(InitialSupportedTacs)
		if err := db.InitializeOperator(context.Background(), initialOperator); err != nil {
			return fmt.Errorf("failed to initialize network configuration: %v", err)
		}
	}

	if !db.IsLogRetentionPolicyInitialized(context.Background(), CategoryAuditLogs) {
		initialPolicy := &LogRetentionPolicy{
			Category: CategoryAuditLogs,
			Days:     DefaultLogRetentionDays,
		}

		if err := db.SetLogRetentionPolicy(context.Background(), initialPolicy); err != nil {
			return fmt.Errorf("failed to initialize log retention policy: %v", err)
		}

		logger.DBLog.Info("Initialized audit log retention policy", zap.Int("days", DefaultLogRetentionDays))
	}

	if !db.IsLogRetentionPolicyInitialized(context.Background(), CategorySubscriberLogs) {
		initialPolicy := &LogRetentionPolicy{
			Category: CategorySubscriberLogs,
			Days:     DefaultLogRetentionDays,
		}

		if err := db.SetLogRetentionPolicy(context.Background(), initialPolicy); err != nil {
			return fmt.Errorf("failed to initialize subscriber log retention policy: %v", err)
		}

		logger.DBLog.Info("Initialized subscriber log retention policy", zap.Int("days", DefaultLogRetentionDays))
	}

	if !db.IsLogRetentionPolicyInitialized(context.Background(), CategoryRadioLogs) {
		initialPolicy := &LogRetentionPolicy{
			Category: CategoryRadioLogs,
			Days:     DefaultLogRetentionDays,
		}

		if err := db.SetLogRetentionPolicy(context.Background(), initialPolicy); err != nil {
			return fmt.Errorf("failed to initialize radio log retention policy: %v", err)
		}

		logger.DBLog.Info("Initialized radio log retention policy", zap.Int("days", DefaultLogRetentionDays))
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
			PriorityLevel:   InitialPolicyPriorityLevel,
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
