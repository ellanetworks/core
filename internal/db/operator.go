// Copyright 2024 Ella Networks

package db

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/canonical/sqlair"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/prometheus/client_golang/prometheus"
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
 		id INTEGER PRIMARY KEY CHECK (id = 1),

		mcc TEXT NOT NULL,
		mnc TEXT NOT NULL,
		operatorCode TEXT NOT NULL,
		supportedTACs TEXT DEFAULT '[]',
		sst INTEGER NOT NULL,
		sd BLOB NULLABLE,  -- 3 bytes
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
	Sd                    []byte `db:"sd"`
	HomeNetworkPrivateKey string `db:"homeNetworkPrivateKey"`
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
	if operator.Sd == nil {
		return ""
	}

	if len(operator.Sd) != 3 {
		logger.DBLog.Warn("SD length is not 3 bytes", zap.Int("length", len(operator.Sd)))
		return ""
	}

	return fmt.Sprintf("%02x%02x%02x", operator.Sd[0], operator.Sd[1], operator.Sd[2])
}

func (operator *Operator) SetSupportedTacs(supportedTACs []string) error {
	supportedTACsBytes, err := json.Marshal(supportedTACs)
	if err != nil {
		return fmt.Errorf("failed to marshal supported TACs: %w", err)
	}

	operator.SupportedTACs = string(supportedTACsBytes)

	return nil
}

func (db *Database) IsOperatorInitialized(ctx context.Context) bool {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "SELECT", OperatorTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("SELECT"),
			attribute.String("db.collection", OperatorTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(OperatorTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(OperatorTableName, "select").Inc()

	var op Operator

	err := db.conn.Query(ctx, db.getOperatorStmt).Get(&op)
	if err != nil {
		if err == sqlair.ErrNoRows {
			span.SetStatus(codes.Ok, "operator not initialized")
			return false
		}

		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")
		logger.DBLog.Error("Failed to get operator", zap.Error(err))

		return false
	}

	span.SetStatus(codes.Ok, "operator initialized")

	return op.ID > 0
}

func (db *Database) InitializeOperator(ctx context.Context, initialOperator *Operator) error {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "INSERT", OperatorTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("INSERT"),
			attribute.String("db.collection", OperatorTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(OperatorTableName, "insert"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(OperatorTableName, "insert").Inc()

	err := db.conn.Query(ctx, db.initializeOperatorStmt, initialOperator).Run()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")

		return fmt.Errorf("failed to initialize operator configuration: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

// GetOperator retrieves the operator row.
func (db *Database) GetOperator(ctx context.Context) (*Operator, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "SELECT", OperatorTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("SELECT"),
			attribute.String("db.collection", OperatorTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(OperatorTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(OperatorTableName, "select").Inc()

	var op Operator

	err := db.conn.Query(ctx, db.getOperatorStmt).Get(&op)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return nil, fmt.Errorf("failed to get operator: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return &op, nil
}

// UpdateOperatorSlice updates SST/SD.
func (db *Database) UpdateOperatorSlice(ctx context.Context, sst int32, sd []byte) error {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "UPDATE", OperatorTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("UPDATE"),
			attribute.String("db.collection", OperatorTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(OperatorTableName, "update"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(OperatorTableName, "update").Inc()

	op := Operator{Sst: sst, Sd: sd}

	err := db.conn.Query(ctx, db.updateOperatorSliceStmt, op).Run()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")

		return fmt.Errorf("failed to update operator slice: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

// UpdateOperatorTracking updates supported TACs.
func (db *Database) UpdateOperatorTracking(ctx context.Context, supportedTACs []string) error {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "UPDATE", OperatorTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("UPDATE"),
			attribute.String("db.collection", OperatorTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(OperatorTableName, "update"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(OperatorTableName, "update").Inc()

	op := Operator{}

	err := op.SetSupportedTacs(supportedTACs)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to set supported TACs")

		return fmt.Errorf("failed to set supported TACs: %w", err)
	}

	err = db.conn.Query(ctx, db.updateOperatorTrackingStmt, op).Run()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")

		return fmt.Errorf("failed to update operator tracking area code: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

// UpdateOperatorID updates MCC/MNC.
func (db *Database) UpdateOperatorID(ctx context.Context, mcc, mnc string) error {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "UPDATE", OperatorTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("UPDATE"),
			attribute.String("db.collection", OperatorTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(OperatorTableName, "update"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(OperatorTableName, "update").Inc()

	op := Operator{Mcc: mcc, Mnc: mnc}

	err := db.conn.Query(ctx, db.updateOperatorIDStmt, op).Run()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")

		return fmt.Errorf("failed to update operator ID: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

// GetOperatorCode fetches only the operatorCode field.
func (db *Database) GetOperatorCode(ctx context.Context) (string, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "SELECT", OperatorTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("SELECT"),
			attribute.String("db.collection", OperatorTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(OperatorTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(OperatorTableName, "select").Inc()

	var op Operator

	err := db.conn.Query(ctx, db.getOperatorStmt).Get(&op)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return "", err
	}

	span.SetStatus(codes.Ok, "")

	return op.OperatorCode, nil
}

// UpdateOperatorCode sets a new operatorCode.
func (db *Database) UpdateOperatorCode(ctx context.Context, operatorCode string) error {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "UPDATE", OperatorTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("UPDATE"),
			attribute.String("db.collection", OperatorTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(OperatorTableName, "update"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(OperatorTableName, "update").Inc()

	op := Operator{OperatorCode: operatorCode}

	err := db.conn.Query(ctx, db.updateOperatorCodeStmt, op).Run()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")

		return fmt.Errorf("failed to update operator code: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

// UpdateHomeNetworkPrivateKey updates the private key.
func (db *Database) UpdateHomeNetworkPrivateKey(ctx context.Context, privateKey string) error {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "UPDATE", OperatorTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("UPDATE"),
			attribute.String("db.collection", OperatorTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(OperatorTableName, "update"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(OperatorTableName, "update").Inc()

	op := Operator{HomeNetworkPrivateKey: privateKey}

	err := db.conn.Query(ctx, db.updateHomeNetworkPrivateKeyStmt, op).Run()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")

		return fmt.Errorf("failed to update operator home network private key: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

// GetHomeNetworkPrivateKey retrieves the private key.
func (db *Database) GetHomeNetworkPrivateKey(ctx context.Context) (string, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "SELECT", OperatorTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("SELECT"),
			attribute.String("db.collection", OperatorTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(OperatorTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(OperatorTableName, "select").Inc()

	var op Operator

	err := db.conn.Query(ctx, db.getOperatorStmt).Get(&op)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return "", err
	}

	span.SetStatus(codes.Ok, "")

	return op.HomeNetworkPrivateKey, nil
}
