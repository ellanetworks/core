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
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

const OperatorTableName = "operator"

const (
	getOperatorStmt                           = "SELECT &Operator.* FROM %s WHERE id=1"
	updateOperatorCodeStmt                    = "UPDATE %s SET operatorCode=$Operator.operatorCode WHERE id=1"
	updateOperatorIDStmt                      = "UPDATE %s SET mcc=$Operator.mcc, mnc=$Operator.mnc WHERE id=1"
	updateOperatorTrackingStmt                = "UPDATE %s SET supportedTACs=$Operator.supportedTACs WHERE id=1"
	updateOperatorSecurityAlgorithmsStmtConst = "UPDATE %s SET ciphering=$Operator.ciphering, integrity=$Operator.integrity WHERE id=1"
	updateOperatorSPNStmtConst                = "UPDATE %s SET spnFullName=$Operator.spnFullName, spnShortName=$Operator.spnShortName WHERE id=1"
	updateOperatorAMFIdentityStmtConst        = "UPDATE %s SET amfRegionID=$Operator.amfRegionID, amfSetID=$Operator.amfSetID WHERE id=1"
	updateOperatorClusterIDStmtConst          = "UPDATE %s SET clusterID=$Operator.clusterID WHERE id=1"
	initializeOperatorStmt                    = "INSERT INTO %s (mcc, mnc, operatorCode, supportedTACs) VALUES ($Operator.mcc, $Operator.mnc, $Operator.operatorCode, $Operator.supportedTACs)"
)

type Operator struct {
	ID            int    `db:"id"`
	Mcc           string `db:"mcc"`
	Mnc           string `db:"mnc"`
	OperatorCode  string `db:"operatorCode"`
	SupportedTACs string `db:"supportedTACs"` // JSON-encoded list of TAC strings
	Ciphering     string `db:"ciphering"`     // JSON-encoded list of algorithm names, e.g. '["NEA2","NEA1"]'
	Integrity     string `db:"integrity"`     // JSON-encoded list of algorithm names, e.g. '["NIA2","NIA1"]'
	SpnFullName   string `db:"spnFullName"`
	SpnShortName  string `db:"spnShortName"`
	AmfRegionID   int    `db:"amfRegionID"`
	AmfSetID      int    `db:"amfSetID"`
	ClusterID     string `db:"clusterID"`
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

func (db *Database) IsOperatorInitialized(ctx context.Context) bool {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "SELECT", OperatorTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
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
	_, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "INSERT", OperatorTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("INSERT"),
			attribute.String("db.collection", OperatorTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(OperatorTableName, "insert"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(OperatorTableName, "insert").Inc()

	_, err := db.proposeChangeset(func(ctx context.Context) (any, error) { return db.applyInitializeOperator(ctx, initialOperator) }, "InitializeOperator")
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return err
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
			semconv.DBSystemNameSQLite,
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

// UpdateOperatorTracking updates supported TACs.
func (db *Database) UpdateOperatorTracking(ctx context.Context, supportedTACs []string) error {
	_, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "UPDATE", OperatorTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("UPDATE"),
			attribute.String("db.collection", OperatorTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(OperatorTableName, "update"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(OperatorTableName, "update").Inc()

	op := &Operator{}

	err := op.SetSupportedTacs(supportedTACs)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to set supported TACs")

		return fmt.Errorf("failed to set supported TACs: %w", err)
	}

	_, err = db.proposeChangeset(func(ctx context.Context) (any, error) { return db.applyUpdateOperatorTracking(ctx, op) }, "UpdateOperatorTracking")
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return err
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

// UpdateOperatorID updates MCC/MNC.
func (db *Database) UpdateOperatorID(ctx context.Context, mcc, mnc string) error {
	_, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "UPDATE", OperatorTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("UPDATE"),
			attribute.String("db.collection", OperatorTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(OperatorTableName, "update"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(OperatorTableName, "update").Inc()

	op := &Operator{Mcc: mcc, Mnc: mnc}

	_, err := db.proposeChangeset(func(ctx context.Context) (any, error) { return db.applyUpdateOperatorID(ctx, op) }, "UpdateOperatorID")
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return err
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
			semconv.DBSystemNameSQLite,
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
	_, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "UPDATE", OperatorTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("UPDATE"),
			attribute.String("db.collection", OperatorTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(OperatorTableName, "update"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(OperatorTableName, "update").Inc()

	op := &Operator{OperatorCode: operatorCode}

	_, err := db.proposeChangeset(func(ctx context.Context) (any, error) { return db.applyUpdateOperatorCode(ctx, op) }, "UpdateOperatorCode")
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return err
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

// UpdateOperatorSecurityAlgorithms updates the NAS security algorithm preference order.
func (db *Database) UpdateOperatorSecurityAlgorithms(ctx context.Context, cipheringOrder, integrityOrder []string) error {
	_, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "UPDATE", OperatorTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("UPDATE"),
			attribute.String("db.collection", OperatorTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(OperatorTableName, "update"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(OperatorTableName, "update").Inc()

	op := &Operator{}

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

	_, err = db.proposeChangeset(func(ctx context.Context) (any, error) {
		return db.applyUpdateOperatorSecurityAlgorithms(ctx, op)
	}, "UpdateOperatorSecurityAlgorithms")
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return err
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

// UpdateOperatorSPN updates the Service Provider Name (full and short).
func (db *Database) UpdateOperatorSPN(ctx context.Context, spnFullName, spnShortName string) error {
	_, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "UPDATE", OperatorTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("UPDATE"),
			attribute.String("db.collection", OperatorTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(OperatorTableName, "update"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(OperatorTableName, "update").Inc()

	op := &Operator{SpnFullName: spnFullName, SpnShortName: spnShortName}

	_, err := db.proposeChangeset(func(ctx context.Context) (any, error) { return db.applyUpdateOperatorSPN(ctx, op) }, "UpdateOperatorSPN")
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return err
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

// UpdateOperatorAMFIdentity updates the AMF Region ID and Set ID used to
// compute the 24-bit AMF ID (3GPP TS 23.003 §2.10.1).
func (db *Database) UpdateOperatorAMFIdentity(ctx context.Context, regionID, setID int) error {
	_, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s (amf identity)", "UPDATE", OperatorTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("UPDATE"),
			attribute.String("db.collection", OperatorTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(OperatorTableName, "update"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(OperatorTableName, "update").Inc()

	op := &Operator{AmfRegionID: regionID, AmfSetID: setID}

	_, err := db.proposeChangeset(func(ctx context.Context) (any, error) { return db.applyUpdateOperatorAMFIdentity(ctx, op) }, "UpdateOperatorAMFIdentity")
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return err
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

// UpdateOperatorClusterID sets the cluster UUID in the operator row.
func (db *Database) UpdateOperatorClusterID(ctx context.Context, clusterID string) error {
	_, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s (cluster id)", "UPDATE", OperatorTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("UPDATE"),
			attribute.String("db.collection", OperatorTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(OperatorTableName, "update"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(OperatorTableName, "update").Inc()

	op := &Operator{ClusterID: clusterID}

	_, err := db.proposeChangeset(func(ctx context.Context) (any, error) { return db.applyUpdateOperatorClusterID(ctx, op) }, "UpdateOperatorClusterID")
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return err
	}

	span.SetStatus(codes.Ok, "")

	return nil
}
