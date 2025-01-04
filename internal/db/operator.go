// Copyright 2024 Ella Networks

package db

import (
	"context"
	"fmt"

	"github.com/canonical/sqlair"
	"github.com/ellanetworks/core/internal/logger"
)

const OperatorTableName = "operator"

const QueryCreateOperatorTable = `
	CREATE TABLE IF NOT EXISTS %s (
 		id INTEGER PRIMARY KEY AUTOINCREMENT,

		mcc TEXT NOT NULL,
		mnc TEXT NOT NULL,
		operatorCode TEXT NOT NULL
)`

const (
	DefaultMcc          = "001"
	DefaultMnc          = "01"
	DefaultOperatorCode = "0123456789ABCDEF0123456789ABCDEF"
)

const (
	getOperatorStmt        = "SELECT &Operator.* FROM %s WHERE id=1"
	updateOperatorIdStmt   = "UPDATE %s SET mcc=$Operator.mcc, mnc=$Operator.mnc WHERE id=1"
	updateOperatorCodeStmt = "UPDATE %s SET operatorCode=$Operator.operatorCode WHERE id=1"
	initializeOperatorStmt = "INSERT INTO %s (mcc, mnc, operatorCode) VALUES ($Operator.mcc, $Operator.mnc, $Operator.operatorCode)"
)

type Operator struct {
	ID           int    `db:"id"`
	Mcc          string `db:"mcc"`
	Mnc          string `db:"mnc"`
	OperatorCode string `db:"operatorCode"`
}

type OperatorId struct {
	Mcc string
	Mnc string
}

func (db *Database) InitializeOperator() error {
	stmt, err := sqlair.Prepare(fmt.Sprintf(initializeOperatorStmt, db.operatorTable), Operator{})
	if err != nil {
		return fmt.Errorf("failed to prepare initialize operator configuration statement: %v", err)
	}
	operator := Operator{
		Mcc:          DefaultMcc,
		Mnc:          DefaultMnc,
		OperatorCode: DefaultOperatorCode,
	}
	err = db.conn.Query(context.Background(), stmt, operator).Run()
	if err != nil {
		return fmt.Errorf("failed to initialize operator configuration: %v", err)
	}
	logger.DBLog.Infof("Initialized operator configuration")
	return nil
}

func (db *Database) GetOperatorId() (*OperatorId, error) {
	stmt, err := sqlair.Prepare(fmt.Sprintf(getOperatorStmt, db.operatorTable), Operator{})
	if err != nil {
		return nil, fmt.Errorf("failed to prepare get operator configuration statement: %v", err)
	}
	var operator Operator
	err = db.conn.Query(context.Background(), stmt).Get(&operator)
	if err != nil {
		return nil, fmt.Errorf("failed to get operator configuration: %v", err)
	}
	operatorID := &OperatorId{
		Mcc: operator.Mcc,
		Mnc: operator.Mnc,
	}
	return operatorID, nil
}

func (db *Database) UpdateOperatorId(operatorID *OperatorId) error {
	stmt, err := sqlair.Prepare(fmt.Sprintf(updateOperatorIdStmt, db.operatorTable), Operator{})
	if err != nil {
		return err
	}
	operator := Operator{
		Mcc: operatorID.Mcc,
		Mnc: operatorID.Mnc,
	}
	err = db.conn.Query(context.Background(), stmt, operator).Run()
	if err != nil {
		return fmt.Errorf("failed to update operator configuration: %v", err)
	}
	logger.DBLog.Infof("Updated operator configuration")
	return nil
}

func (db *Database) GetOperatorCode() (string, error) {
	stmt, err := sqlair.Prepare(fmt.Sprintf(getOperatorStmt, db.operatorTable), Operator{})
	if err != nil {
		return "", fmt.Errorf("failed to prepare get operator code statement: %v", err)
	}
	var operator Operator
	err = db.conn.Query(context.Background(), stmt).Get(&operator)
	if err != nil {
		return "", fmt.Errorf("failed to get operator code: %v", err)
	}
	return operator.OperatorCode, nil
}

func (db *Database) UpdateOperatorCode(operatorCode string) error {
	stmt, err := sqlair.Prepare(fmt.Sprintf(updateOperatorCodeStmt, db.operatorTable), Operator{})
	if err != nil {
		return err
	}
	operator := Operator{
		OperatorCode: operatorCode,
	}
	err = db.conn.Query(context.Background(), stmt, operator).Run()
	if err != nil {
		return fmt.Errorf("failed to update operator code: %v", err)
	}
	logger.DBLog.Infof("Updated operator code")
	return nil
}
