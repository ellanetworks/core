// Copyright 2024 Ella Networks

// Package db provides a simplistic ORM to communicate with an SQL database for storage
package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/XSAM/otelsql"
	"github.com/canonical/sqlair"
	"github.com/ellanetworks/core/internal/logger"
	_ "github.com/mattn/go-sqlite3"
	"go.opentelemetry.io/otel/attribute"
)

// Database is the object used to communicate with the established repository.
type Database struct {
	filepath         string
	subscribersTable string
	profilesTable    string
	routesTable      string
	operatorTable    string
	usersTable       string
	conn             *sqlair.DB
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
func NewDatabase(databasePath string, initialOperator Operator) (*Database, error) {
	// Register an instrumented driver for sqlite3
	driverName, err := otelsql.Register(
		"sqlite3",
		otelsql.WithAttributes(
			attribute.String("db.system", "sqlite"),
			attribute.String("db.name", databasePath),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to register otel sqlite3 driver: %w", err)
	}

	// Open a connection using the instrumented driver
	sqlConnection, err := sql.Open(driverName, databasePath)
	if err != nil {
		return nil, err
	}

	// Initialize tables
	if _, err := sqlConnection.Exec(fmt.Sprintf(QueryCreateSubscribersTable, SubscribersTableName)); err != nil {
		return nil, err
	}
	if _, err := sqlConnection.Exec(fmt.Sprintf(QueryCreateProfilesTable, ProfilesTableName)); err != nil {
		return nil, err
	}
	if _, err := sqlConnection.Exec(fmt.Sprintf(QueryCreateRoutesTable, RoutesTableName)); err != nil {
		return nil, err
	}
	if _, err := sqlConnection.Exec(fmt.Sprintf(QueryCreateOperatorTable, OperatorTableName)); err != nil {
		return nil, err
	}
	if _, err := sqlConnection.Exec(fmt.Sprintf(QueryCreateUsersTable, UsersTableName)); err != nil {
		return nil, err
	}

	db := new(Database)
	db.conn = sqlair.NewDB(sqlConnection)
	db.filepath = databasePath
	db.subscribersTable = SubscribersTableName
	db.profilesTable = ProfilesTableName
	db.routesTable = RoutesTableName
	db.operatorTable = OperatorTableName
	db.usersTable = UsersTableName

	if err := db.InitializeOperator(initialOperator); err != nil {
		return nil, fmt.Errorf("failed to initialize network configuration: %v", err)
	}

	logger.DBLog.Info("Database Initialized")
	return db, nil
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
