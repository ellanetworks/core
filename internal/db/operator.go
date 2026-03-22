// Copyright 2024 Ella Networks

package db

import (
	"context"
	"crypto/ecdh"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/canonical/sqlair"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.27.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

const OperatorTableName = "operator"

const (
	getOperatorStmt                           = "SELECT &Operator.* FROM %s WHERE id=1"
	updateOperatorCodeStmt                    = "UPDATE %s SET operatorCode=$Operator.operatorCode WHERE id=1"
	updateOperatorIDStmt                      = "UPDATE %s SET mcc=$Operator.mcc, mnc=$Operator.mnc WHERE id=1"
	updateOperatorSliceStmt                   = "UPDATE %s SET sst=$Operator.sst, sd=$Operator.sd WHERE id=1"
	updateOperatorTrackingStmt                = "UPDATE %s SET supportedTACs=$Operator.supportedTACs WHERE id=1"
	updateOperatorSecurityAlgorithmsStmtConst = "UPDATE %s SET ciphering=$Operator.ciphering, integrity=$Operator.integrity WHERE id=1"
	updateOperatorSPNStmtConst                = "UPDATE %s SET spnFullName=$Operator.spnFullName, spnShortName=$Operator.spnShortName WHERE id=1"
	initializeOperatorStmt                    = "INSERT INTO %s (mcc, mnc, operatorCode, supportedTACs, sst, sd) VALUES ($Operator.mcc, $Operator.mnc, $Operator.operatorCode, $Operator.supportedTACs, $Operator.sst, $Operator.sd)"
)

type Operator struct {
	ID            int    `db:"id"`
	Mcc           string `db:"mcc"`
	Mnc           string `db:"mnc"`
	OperatorCode  string `db:"operatorCode"`
	SupportedTACs string `db:"supportedTACs"` // JSON-encoded list of strings
	Sst           int32  `db:"sst"`
	Sd            []byte `db:"sd"`
	Ciphering     string `db:"ciphering"` // JSON-encoded list of algorithm names, e.g. '["NEA2","NEA1"]'
	Integrity     string `db:"integrity"` // JSON-encoded list of algorithm names, e.g. '["NIA2","NIA1"]'
	SpnFullName   string `db:"spnFullName"`
	SpnShortName  string `db:"spnShortName"`
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
			semconv.DBOperationName("SELECT"),
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
		logger.WithTrace(ctx, logger.DBLog).Error("Failed to get operator", zap.Error(err))

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
			semconv.DBOperationName("INSERT"),
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
		span.SetStatus(codes.Error, "query failed")

		return fmt.Errorf("query failed: %w", err)
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
			semconv.DBOperationName("SELECT"),
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

		return nil, fmt.Errorf("query failed: %w", err)
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
			semconv.DBOperationName("UPDATE"),
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
		span.SetStatus(codes.Error, "query failed")

		return fmt.Errorf("query failed: %w", err)
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
			semconv.DBOperationName("UPDATE"),
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
		span.SetStatus(codes.Error, "query failed")

		return fmt.Errorf("query failed: %w", err)
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
			semconv.DBOperationName("UPDATE"),
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
		span.SetStatus(codes.Error, "query failed")

		return fmt.Errorf("query failed: %w", err)
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
			semconv.DBOperationName("SELECT"),
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

		return "", fmt.Errorf("query failed: %w", err)
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
			semconv.DBOperationName("UPDATE"),
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
		span.SetStatus(codes.Error, "query failed")

		return fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

// UpdateOperatorSecurityAlgorithms updates the NAS security algorithm preference order.
func (db *Database) UpdateOperatorSecurityAlgorithms(ctx context.Context, cipheringOrder, integrityOrder []string) error {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "UPDATE", OperatorTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationName("UPDATE"),
			attribute.String("db.collection", OperatorTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(OperatorTableName, "update"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(OperatorTableName, "update").Inc()

	op := Operator{}

	err := op.SetCiphering(cipheringOrder)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to set ciphering order")

		return fmt.Errorf("failed to set ciphering order: %w", err)
	}

	err = op.SetIntegrity(integrityOrder)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to set integrity order")

		return fmt.Errorf("failed to set integrity order: %w", err)
	}

	err = db.conn.Query(ctx, db.updateOperatorSecurityAlgorithmsStmt, op).Run()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

// UpdateOperatorSPN updates the Service Provider Name (full and short).
func (db *Database) UpdateOperatorSPN(ctx context.Context, spnFullName, spnShortName string) error {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "UPDATE", OperatorTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationName("UPDATE"),
			attribute.String("db.collection", OperatorTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(OperatorTableName, "update"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(OperatorTableName, "update").Inc()

	op := Operator{SpnFullName: spnFullName, SpnShortName: spnShortName}

	err := db.conn.Query(ctx, db.updateOperatorSPNStmt, op).Run()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return nil
}
