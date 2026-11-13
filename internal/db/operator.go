// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"crypto/ecdh"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

const OperatorTableName = "operator"

const (
	getOperatorSQL                      = "SELECT id, mcc, mnc, operatorCode, COALESCE(supportedTACs, '[]'), ciphering, integrity, spnFullName, spnShortName, amfRegionID, amfSetID, clusterID FROM " + OperatorTableName + " WHERE id=1"
	updateOperatorCodeSQL               = "UPDATE " + OperatorTableName + " SET operatorCode=? WHERE id=1"
	updateOperatorIDSQL                 = "UPDATE " + OperatorTableName + " SET mcc=?, mnc=? WHERE id=1"
	updateOperatorTrackingSQL           = "UPDATE " + OperatorTableName + " SET supportedTACs=? WHERE id=1"
	updateOperatorSecurityAlgorithmsSQL = "UPDATE " + OperatorTableName + " SET ciphering=?, integrity=? WHERE id=1"
	updateOperatorSPNSQL                = "UPDATE " + OperatorTableName + " SET spnFullName=?, spnShortName=? WHERE id=1"
	updateOperatorAMFIdentitySQL        = "UPDATE " + OperatorTableName + " SET amfRegionID=?, amfSetID=? WHERE id=1"
	updateOperatorClusterIDSQL          = "UPDATE " + OperatorTableName + " SET clusterID=? WHERE id=1"
	initializeOperatorSQL               = "INSERT INTO " + OperatorTableName + " (mcc, mnc, operatorCode, supportedTACs, ciphering, integrity, clusterID) VALUES (?, ?, ?, ?, ?, ?, ?)"
)

type Operator struct {
	ID            int
	Mcc           string
	Mnc           string
	OperatorCode  string
	SupportedTACs string // JSON-encoded list of TAC strings
	Ciphering     string // JSON list of RAT-neutral algorithm names, e.g. '["AES","SNOW3G"]'
	Integrity     string // JSON list of RAT-neutral algorithm names, e.g. '["AES","SNOW3G"]'
	SpnFullName   string
	SpnShortName  string
	AmfRegionID   int
	AmfSetID      int
	ClusterID     string
}

const guamiRegionIDNodeType = 0x80

const (
	maxAmfRegionID = 0x7f
	maxAmfSetID    = 1023
)

func (operator *Operator) GUAMIRegionID() uint8 {
	return uint8(operator.AmfRegionID) | guamiRegionIDNodeType
}

func (operator *Operator) GetSupportedTacs() ([]string, error) {
	if operator.SupportedTACs == "" {
		return nil, nil
	}

	var supportedTACs []string

	err := json.Unmarshal([]byte(operator.SupportedTACs), &supportedTACs)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal supported TACs: %w", err)
	}

	return supportedTACs, nil
}

func (operator *Operator) GetCiphering() ([]string, error) {
	if operator.Ciphering == "" {
		return nil, nil
	}

	var order []string

	err := json.Unmarshal([]byte(operator.Ciphering), &order)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal ciphering order: %w", err)
	}

	return order, nil
}

func (operator *Operator) SetCiphering(order []string) error {
	b, err := json.Marshal(order)
	if err != nil {
		return fmt.Errorf("failed to marshal ciphering order: %w", err)
	}

	operator.Ciphering = string(b)

	return nil
}

func (operator *Operator) GetIntegrity() ([]string, error) {
	if operator.Integrity == "" {
		return nil, nil
	}

	var order []string

	err := json.Unmarshal([]byte(operator.Integrity), &order)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal integrity order: %w", err)
	}

	return order, nil
}

func (operator *Operator) SetIntegrity(order []string) error {
	b, err := json.Marshal(order)
	if err != nil {
		return fmt.Errorf("failed to marshal integrity order: %w", err)
	}

	operator.Integrity = string(b)

	return nil
}

// deriveHomeNetworkPublicKey derives the public key from a given private key using X25519 (Profile A).
func deriveHomeNetworkPublicKey(privateKeyHex string) (string, error) {
	privBytes, err := hex.DecodeString(privateKeyHex)
	if err != nil {
		return "", fmt.Errorf("failed to decode private key hex: %w", err)
	}

	priv, err := ecdh.X25519().NewPrivateKey(privBytes)
	if err != nil {
		return "", fmt.Errorf("failed to create X25519 private key: %w", err)
	}

	return hex.EncodeToString(priv.PublicKey().Bytes()), nil
}

func (operator *Operator) SetSupportedTacs(supportedTACs []string) error {
	supportedTACsBytes, err := json.Marshal(supportedTACs)
	if err != nil {
		return fmt.Errorf("failed to marshal supported TACs: %w", err)
	}

	operator.SupportedTACs = string(supportedTACsBytes)

	return nil
}

func getOperator(ctx context.Context, q querier) (Operator, error) {
	var op Operator

	err := q.QueryRowContext(ctx, getOperatorSQL).Scan(&op.ID, &op.Mcc, &op.Mnc, &op.OperatorCode, &op.SupportedTACs, &op.Ciphering, &op.Integrity, &op.SpnFullName, &op.SpnShortName, &op.AmfRegionID, &op.AmfSetID, &op.ClusterID)

	return op, err
}

func (db *Database) IsOperatorInitialized(ctx context.Context) bool {
	ctx, finish := startDBQuery(ctx, OperatorTableName, "SELECT")

	op, err := getOperator(ctx, db.reader())
	finish(err)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false
		}

		logger.From(ctx, logger.DBLog).Error("Failed to get operator", zap.Error(err))

		return false
	}

	return op.ID > 0
}

func (db *Database) InitializeOperator(ctx context.Context, initialOperator *Operator) (err error) {
	ctx, finish := startDBQuery(ctx, OperatorTableName, "INSERT")
	defer func() { finish(err) }()

	_, err = opInitializeOperator.Invoke(ctx, db, initialOperator)
	if err != nil {
		return err
	}

	return nil
}

// GetOperator retrieves the operator row.
func (db *Database) GetOperator(ctx context.Context) (_ *Operator, err error) {
	ctx, finish := startDBQuery(ctx, OperatorTableName, "SELECT")
	defer func() { finish(err) }()

	op, err := getOperator(ctx, db.reader())
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return &op, nil
}

// UpdateOperatorTracking updates supported TACs.
func (db *Database) UpdateOperatorTracking(ctx context.Context, supportedTACs []string) (err error) {
	ctx, finish := startDBQuery(ctx, OperatorTableName, "UPDATE")
	defer func() { finish(err) }()

	op := &Operator{}

	err = op.SetSupportedTacs(supportedTACs)
	if err != nil {
		return fmt.Errorf("failed to set supported TACs: %w", err)
	}

	_, err = opUpdateOperatorTracking.Invoke(ctx, db, op)
	if err != nil {
		return err
	}

	return nil
}

// UpdateOperatorID updates MCC/MNC.
func (db *Database) UpdateOperatorID(ctx context.Context, mcc, mnc string) error {
	_, err := db.ChangeOperatorID(ctx, mcc, mnc)
	return err
}

func (db *Database) ChangeOperatorID(ctx context.Context, mcc, mnc string) (_ bool, err error) {
	ctx, finish := startDBQuery(ctx, OperatorTableName, "UPDATE")
	defer func() { finish(err) }()

	op := &Operator{Mcc: mcc, Mnc: mnc}

	seppIdentityDeleted, err := opUpdateOperatorID.Invoke(ctx, db, op)
	if err != nil {
		return false, err
	}

	return seppIdentityDeleted, nil
}

// GetOperatorCode fetches only the operatorCode field.
func (db *Database) GetOperatorCode(ctx context.Context) (_ string, err error) {
	ctx, finish := startDBQuery(ctx, OperatorTableName, "SELECT")
	defer func() { finish(err) }()

	op, err := getOperator(ctx, db.reader())
	if err != nil {
		return "", fmt.Errorf("query failed: %w", err)
	}

	return op.OperatorCode, nil
}

// UpdateOperatorCode sets a new operatorCode.
func (db *Database) UpdateOperatorCode(ctx context.Context, operatorCode string) (err error) {
	ctx, finish := startDBQuery(ctx, OperatorTableName, "UPDATE")
	defer func() { finish(err) }()

	op := &Operator{OperatorCode: operatorCode}

	_, err = opUpdateOperatorCode.Invoke(ctx, db, op)
	if err != nil {
		return err
	}

	return nil
}

// UpdateOperatorSecurityAlgorithms updates the NAS security algorithm preference order.
func (db *Database) UpdateOperatorSecurityAlgorithms(ctx context.Context, cipheringOrder, integrityOrder []string) (err error) {
	ctx, finish := startDBQuery(ctx, OperatorTableName, "UPDATE")
	defer func() { finish(err) }()

	op := &Operator{}

	err = op.SetCiphering(cipheringOrder)
	if err != nil {
		return fmt.Errorf("failed to set ciphering order: %w", err)
	}

	err = op.SetIntegrity(integrityOrder)
	if err != nil {
		return fmt.Errorf("failed to set integrity order: %w", err)
	}

	_, err = opUpdateOperatorSecurityAlgorithms.Invoke(ctx, db, op)
	if err != nil {
		return err
	}

	return nil
}

// UpdateOperatorSPN updates the Service Provider Name (full and short).
func (db *Database) UpdateOperatorSPN(ctx context.Context, spnFullName, spnShortName string) (err error) {
	ctx, finish := startDBQuery(ctx, OperatorTableName, "UPDATE")
	defer func() { finish(err) }()

	op := &Operator{SpnFullName: spnFullName, SpnShortName: spnShortName}

	_, err = opUpdateOperatorSPN.Invoke(ctx, db, op)
	if err != nil {
		return err
	}

	return nil
}

// UpdateOperatorAMFIdentity updates the AMF Region ID and Set ID used to
// compute the 24-bit AMF ID (3GPP TS 23.003 §2.10.1).
func (db *Database) UpdateOperatorAMFIdentity(ctx context.Context, regionID, setID int) (err error) {
	if regionID < 0 || regionID > maxAmfRegionID {
		return fmt.Errorf("amfRegionID must be between 0 and %d (the node-type bit is reserved), got %d", maxAmfRegionID, regionID)
	}

	if setID < 0 || setID > maxAmfSetID {
		return fmt.Errorf("amfSetID must be between 0 and %d (constrained by the 10-bit AMF Set ID field), got %d", maxAmfSetID, setID)
	}

	ctx, finish := startDBQueryNamed(ctx, "UPDATE "+OperatorTableName+" (amf identity)", OperatorTableName, "UPDATE")
	defer func() { finish(err) }()

	op := &Operator{AmfRegionID: regionID, AmfSetID: setID}

	_, err = opUpdateOperatorAMFIdentity.Invoke(ctx, db, op)
	if err != nil {
		return err
	}

	return nil
}

// UpdateOperatorClusterID sets the cluster UUID in the operator row.
func (db *Database) UpdateOperatorClusterID(ctx context.Context, clusterID string) (err error) {
	ctx, finish := startDBQueryNamed(ctx, "UPDATE "+OperatorTableName+" (cluster id)", OperatorTableName, "UPDATE")
	defer func() { finish(err) }()

	op := &Operator{ClusterID: clusterID}

	_, err = opUpdateOperatorClusterID.Invoke(ctx, db, op)
	if err != nil {
		return err
	}

	return nil
}
