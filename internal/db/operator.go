// Copyright 2024 Ella Networks

package db

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/canonical/sqlair"
	"github.com/ellanetworks/core/internal/logger"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
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
	updateOperatorIDStmt                    = "UPDATE %s SET mcc=$Operator.mcc, mnc=$Operator.mnc WHERE id=1"
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
		logger.DBLog.Warn("Failed to unmarshal supported TACs", zap.Error(err))
		return nil
	}
	return supportedTACs
}

func (operator *Operator) GetHomeNetworkPublicKey() (string, error) {
	return deriveHomeNetworkPublicKey(operator.HomeNetworkPrivateKey)
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

func (operator *Operator) GetHexSd() string {
	return fmt.Sprintf("%06X", operator.Sd)
}

func (operator *Operator) SetSupportedTacs(supportedTACs []string) {
	supportedTACsBytes, err := json.Marshal(supportedTACs)
	if err != nil {
		logger.DBLog.Warn("Failed to marshal supported TACs", zap.Error(err))
		return
	}
	operator.SupportedTACs = string(supportedTACsBytes)
}

func (db *Database) InitializeOperator(initialOperator Operator, ctx context.Context) error {
	operation := "INSERT"
	target := OperatorTableName
	spanName := fmt.Sprintf("%s %s", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	stmt := fmt.Sprintf(initializeOperatorStmt, db.operatorTable)
	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(stmt),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
	)

	q, err := sqlair.Prepare(stmt, Operator{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "prepare failed")
		return fmt.Errorf("failed to prepare initialize operator configuration statement: %w", err)
	}
	if err := db.conn.Query(ctx, q, initialOperator).Run(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")
		return fmt.Errorf("failed to initialize operator configuration: %w", err)
	}

	span.SetStatus(codes.Ok, "")
	return nil
}

// GetOperator retrieves the operator row.
func (db *Database) GetOperator(ctx context.Context) (*Operator, error) {
	operation := "SELECT"
	target := OperatorTableName
	spanName := fmt.Sprintf("%s %s", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	stmt := fmt.Sprintf(getOperatorStmt, db.operatorTable)
	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(stmt),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
	)

	q, err := sqlair.Prepare(stmt, Operator{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "prepare failed")
		return nil, fmt.Errorf("failed to prepare get operator statement: %w", err)
	}

	var op Operator
	if err := db.conn.Query(ctx, q).Get(&op); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")
		return nil, fmt.Errorf("failed to get operator: %w", err)
	}

	span.SetStatus(codes.Ok, "")
	return &op, nil
}

// UpdateOperatorSlice updates SST/SD.
func (db *Database) UpdateOperatorSlice(sst int32, sd int, ctx context.Context) error {
	operation := "UPDATE"
	target := OperatorTableName
	spanName := fmt.Sprintf("%s %s", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	stmt := fmt.Sprintf(updateOperatorSliceStmt, db.operatorTable)
	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(stmt),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
	)

	op := Operator{Sst: sst, Sd: sd}
	q, err := sqlair.Prepare(stmt, Operator{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "prepare failed")
		return err
	}
	if err := db.conn.Query(ctx, q, op).Run(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")
		return fmt.Errorf("failed to update operator slice: %w", err)
	}

	logger.DBLog.Info("Updated operator slice information")
	span.SetStatus(codes.Ok, "")
	return nil
}

// UpdateOperatorTracking updates supported TACs.
func (db *Database) UpdateOperatorTracking(supportedTACs []string, ctx context.Context) error {
	operation := "UPDATE"
	target := OperatorTableName
	spanName := fmt.Sprintf("%s %s", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	stmt := fmt.Sprintf(updateOperatorTrackingStmt, db.operatorTable)
	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(stmt),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
	)

	op := Operator{}
	op.SetSupportedTacs(supportedTACs)
	q, err := sqlair.Prepare(stmt, Operator{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "prepare failed")
		return err
	}
	if err := db.conn.Query(ctx, q, op).Run(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")
		return fmt.Errorf("failed to update operator tracking area code: %w", err)
	}

	logger.DBLog.Info("Updated operator tracking area code")
	span.SetStatus(codes.Ok, "")
	return nil
}

// UpdateOperatorID updates MCC/MNC.
func (db *Database) UpdateOperatorID(mcc, mnc string, ctx context.Context) error {
	operation := "UPDATE"
	target := OperatorTableName
	spanName := fmt.Sprintf("%s %s", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	stmt := fmt.Sprintf(updateOperatorIDStmt, db.operatorTable)
	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(stmt),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
	)

	op := Operator{Mcc: mcc, Mnc: mnc}
	q, err := sqlair.Prepare(stmt, Operator{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "prepare failed")
		return err
	}
	if err := db.conn.Query(ctx, q, op).Run(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")
		return fmt.Errorf("failed to update operator ID: %w", err)
	}

	logger.DBLog.Info("Updated operator ID")
	span.SetStatus(codes.Ok, "")
	return nil
}

// GetOperatorCode fetches only the operatorCode field.
func (db *Database) GetOperatorCode(ctx context.Context) (string, error) {
	operation := "SELECT"
	target := OperatorTableName
	spanName := fmt.Sprintf("%s %s", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	stmt := fmt.Sprintf(getOperatorStmt, db.operatorTable)
	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(stmt),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
	)

	q, err := sqlair.Prepare(stmt, Operator{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "prepare failed")
		return "", err
	}

	var op Operator
	if err := db.conn.Query(ctx, q).Get(&op); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")
		return "", err
	}

	span.SetStatus(codes.Ok, "")
	return op.OperatorCode, nil
}

// UpdateOperatorCode sets a new operatorCode.
func (db *Database) UpdateOperatorCode(operatorCode string, ctx context.Context) error {
	operation := "UPDATE"
	target := OperatorTableName
	spanName := fmt.Sprintf("%s %s", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	stmt := fmt.Sprintf(updateOperatorCodeStmt, db.operatorTable)
	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(stmt),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
	)

	op := Operator{OperatorCode: operatorCode}
	q, err := sqlair.Prepare(stmt, Operator{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "prepare failed")
		return err
	}
	if err := db.conn.Query(ctx, q, op).Run(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")
		return fmt.Errorf("failed to update operator code: %w", err)
	}

	logger.DBLog.Info("Updated operator code")
	span.SetStatus(codes.Ok, "")
	return nil
}

// UpdateHomeNetworkPrivateKey updates the private key.
func (db *Database) UpdateHomeNetworkPrivateKey(privateKey string, ctx context.Context) error {
	operation := "UPDATE"
	target := OperatorTableName
	spanName := fmt.Sprintf("%s %s", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	stmt := fmt.Sprintf(updateOperatorHomeNetworkPrivateKeyStmt, db.operatorTable)
	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(stmt),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
	)

	op := Operator{HomeNetworkPrivateKey: privateKey}
	q, err := sqlair.Prepare(stmt, Operator{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "prepare failed")
		return err
	}
	if err := db.conn.Query(ctx, q, op).Run(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")
		return fmt.Errorf("failed to update operator home network private key: %w", err)
	}

	logger.DBLog.Info("Updated operator home network private key")
	span.SetStatus(codes.Ok, "")
	return nil
}

// GetHomeNetworkPrivateKey retrieves the private key.
func (db *Database) GetHomeNetworkPrivateKey(ctx context.Context) (string, error) {
	operation := "SELECT"
	target := OperatorTableName
	spanName := fmt.Sprintf("%s %s", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	stmt := fmt.Sprintf(getOperatorStmt, db.operatorTable)
	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(stmt),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
	)

	q, err := sqlair.Prepare(stmt, Operator{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "prepare failed")
		return "", err
	}

	var op Operator
	if err := db.conn.Query(ctx, q).Get(&op); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")
		return "", err
	}

	span.SetStatus(codes.Ok, "")
	return op.HomeNetworkPrivateKey, nil
}
