// Copyright 2024 Ella Networks

package db

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/canonical/sqlair"
	"github.com/ellanetworks/core/internal/logger"
	"golang.org/x/crypto/curve25519"
)

const OperatorTableName = "operator"

const QueryCreateOperatorTable = `
	CREATE TABLE IF NOT EXISTS %s (
 		id INTEGER PRIMARY KEY AUTOINCREMENT,

		mcc TEXT NOT NULL,
		mnc TEXT NOT NULL,
		operatorCode TEXT NOT NULL,
		supportedTACs TEXT DEFAULT '[]',
		sst INTEGER NOT NULL,
		sd INTEGER NOT NULL,
		homeNetworkPrivateKey TEXT NOT NULL
)`

const (
	getOperatorStmt                         = "SELECT &Operator.* FROM %s WHERE id=1"
	updateOperatorCodeStmt                  = "UPDATE %s SET operatorCode=$Operator.operatorCode WHERE id=1"
	updateOperatorIdStmt                    = "UPDATE %s SET mcc=$Operator.mcc, mnc=$Operator.mnc WHERE id=1"
	updateOperatorSliceStmt                 = "UPDATE %s SET sst=$Operator.sst, sd=$Operator.sd WHERE id=1"
	updateOperatorTrackingStmt              = "UPDATE %s SET supportedTACs=$Operator.supportedTACs WHERE id=1"
	updateOperatorHomeNetworkPrivateKeyStmt = "UPDATE %s SET homeNetworkPrivateKey=$Operator.homeNetworkPrivateKey WHERE id=1"
	initializeOperatorStmt                  = "INSERT INTO %s (mcc, mnc, operatorCode, supportedTACs, sst, sd, homeNetworkPrivateKey) VALUES ($Operator.mcc, $Operator.mnc, $Operator.operatorCode, $Operator.supportedTACs, $Operator.sst, $Operator.sd, $Operator.homeNetworkPrivateKey)"
)

type Operator struct {
	ID                    int    `db:"id"`
	Mcc                   string `db:"mcc"`
	Mnc                   string `db:"mnc"`
	OperatorCode          string `db:"operatorCode"`
	SupportedTACs         string `db:"supportedTACs"` // JSON-encoded list of strings
	Sst                   int32  `db:"sst"`
	Sd                    int    `db:"sd"`
	HomeNetworkPrivateKey string `db:"homeNetworkPrivateKey"`
}

func (operator *Operator) GetSupportedTacs() []string {
	var supportedTACs []string
	err := json.Unmarshal([]byte(operator.SupportedTACs), &supportedTACs)
	if err != nil {
		logger.DBLog.Warnf("Failed to unmarshal supported TACs: %v", err)
		return nil
	}
	return supportedTACs
}

// deriveHomeNetworkPublicKey derives the public key from a given private key using Curve25519.
func deriveHomeNetworkPublicKey(privateKeyHex string) (string, error) {
	privateKey, err := hex.DecodeString(privateKeyHex)
	if err != nil {
		return "", fmt.Errorf("failed to decode private key hex: %w", err)
	}

	if len(privateKey) != 32 {
		return "", fmt.Errorf("invalid private key length: expected 32 bytes, got %d", len(privateKey))
	}

	// Compute the public key using Curve25519 base point multiplication
	publicKey, err := curve25519.X25519(privateKey, curve25519.Basepoint)
	if err != nil {
		return "", fmt.Errorf("failed to derive public key: %w", err)
	}

	return hex.EncodeToString(publicKey), nil
}

func (operator *Operator) GetHomeNetworkPublicKey() (string, error) {
	return deriveHomeNetworkPublicKey(operator.HomeNetworkPrivateKey)
}

func (operator *Operator) GetHexSd() string {
	return fmt.Sprintf("%X", operator.Sd)
}

func (operator *Operator) SetSupportedTacs(supportedTACs []string) {
	supportedTACsBytes, err := json.Marshal(supportedTACs)
	if err != nil {
		logger.DBLog.Warnf("Failed to marshal supported TACs: %v", err)
		return
	}
	operator.SupportedTACs = string(supportedTACsBytes)
}

func (db *Database) InitializeOperator(initialOperator Operator) error {
	stmt, err := sqlair.Prepare(fmt.Sprintf(initializeOperatorStmt, db.operatorTable), Operator{})
	if err != nil {
		return fmt.Errorf("failed to prepare initialize operator configuration statement: %v", err)
	}
	err = db.conn.Query(context.Background(), stmt, initialOperator).Run()
	if err != nil {
		return fmt.Errorf("failed to initialize operator configuration: %v", err)
	}
	logger.DBLog.Infof("Initialized operator configuration")
	return nil
}

func (db *Database) GetOperator() (*Operator, error) {
	stmt, err := sqlair.Prepare(fmt.Sprintf(getOperatorStmt, db.operatorTable), Operator{})
	if err != nil {
		return nil, fmt.Errorf("failed to prepare get Operator statement: %v", err)
	}
	var operator Operator
	err = db.conn.Query(context.Background(), stmt).Get(&operator)
	if err != nil {
		return nil, fmt.Errorf("failed to get Operator: %v", err)
	}
	return &operator, nil
}

func (db *Database) UpdateOperatorSlice(sst int32, sd int) error {
	stmt, err := sqlair.Prepare(fmt.Sprintf(updateOperatorSliceStmt, db.operatorTable), Operator{})
	if err != nil {
		return err
	}
	operator := Operator{
		Sst: sst,
		Sd:  sd,
	}
	err = db.conn.Query(context.Background(), stmt, operator).Run()
	if err != nil {
		return fmt.Errorf("failed to update operator ID: %v", err)
	}
	logger.DBLog.Infof("Updated operator slice information")
	return nil
}

func (db *Database) UpdateOperatorTracking(supportedTACs []string) error {
	stmt, err := sqlair.Prepare(fmt.Sprintf(updateOperatorTrackingStmt, db.operatorTable), Operator{})
	if err != nil {
		return err
	}
	operator := Operator{}
	operator.SetSupportedTacs(supportedTACs)
	err = db.conn.Query(context.Background(), stmt, operator).Run()
	if err != nil {
		return fmt.Errorf("failed to update operator tracking area code: %v", err)
	}
	logger.DBLog.Infof("Updated operator tracking area code")
	return nil
}

func (db *Database) UpdateOperatorId(mcc, mnc string) error {
	stmt, err := sqlair.Prepare(fmt.Sprintf(updateOperatorIdStmt, db.operatorTable), Operator{})
	if err != nil {
		return err
	}
	operator := Operator{
		Mcc: mcc,
		Mnc: mnc,
	}
	err = db.conn.Query(context.Background(), stmt, operator).Run()
	if err != nil {
		return fmt.Errorf("failed to update operator ID: %v", err)
	}
	logger.DBLog.Infof("Updated operator ID")
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

func (db *Database) UpdateHomeNetworkPrivateKey(privateKey string) error {
	stmt, err := sqlair.Prepare(fmt.Sprintf(updateOperatorHomeNetworkPrivateKeyStmt, db.operatorTable), Operator{})
	if err != nil {
		return err
	}
	operator := Operator{
		HomeNetworkPrivateKey: privateKey,
	}
	err = db.conn.Query(context.Background(), stmt, operator).Run()
	if err != nil {
		return fmt.Errorf("failed to update operator home network private key: %v", err)
	}
	logger.DBLog.Infof("Updated operator home network private key")
	return nil
}

func (db *Database) GetHomeNetworkPrivateKey() (string, error) {
	stmt, err := sqlair.Prepare(fmt.Sprintf(getOperatorStmt, db.operatorTable), Operator{})
	if err != nil {
		return "", fmt.Errorf("failed to prepare get home network private key statement: %v", err)
	}
	var operator Operator
	err = db.conn.Query(context.Background(), stmt).Get(&operator)
	if err != nil {
		return "", fmt.Errorf("failed to get home network private key: %v", err)
	}
	return operator.HomeNetworkPrivateKey, nil
}
