package db

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
)

const FleetTableName = "fleet"

const QueryCreateFleetTable = `
CREATE TABLE IF NOT EXISTS %s (
  singleton BOOLEAN PRIMARY KEY DEFAULT TRUE,
  enabled   BOOLEAN NOT NULL DEFAULT TRUE,
	private_key BLOB NOT NULL,
	certificate BLOB NOT NULL,
	ca_certificate BLOB NOT NULL,
  CHECK (singleton)
);
`

const (
	getFleetStmt               = "SELECT &Fleet.* FROM %s WHERE singleton"
	updateFleetKeyStmt         = "UPDATE %s SET private_key=$Fleet.private_key WHERE singleton"
	updateFleetCredentialsStmt = "UPDATE %s SET certificate=$Fleet.certificate, ca_certificate=$Fleet.ca_certificate WHERE singleton"
	initializeFleetStmt        = "INSERT OR IGNORE INTO %s (singleton, enabled, private_key, certificate, ca_certificate) VALUES (TRUE, TRUE, $Fleet.private_key, $Fleet.certificate, $Fleet.ca_certificate)"
)

type Fleet struct {
	Enabled       bool   `db:"enabled"`
	PrivateKey    []byte `db:"private_key"`
	Certificate   []byte `db:"certificate"`
	CACertificate []byte `db:"ca_certificate"`
}

// InitializeFleet inserts the default fleet row if it does not exist.
func (db *Database) InitializeFleet(ctx context.Context) error {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "INSERT", FleetTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("INSERT"),
			attribute.String("db.collection", FleetTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(FleetTableName, "insert"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(FleetTableName, "insert").Inc()

	fleet := Fleet{
		PrivateKey:    []byte{},
		Certificate:   []byte{},
		CACertificate: []byte{},
	}

	err := db.conn.Query(ctx, db.initializeFleetStmt, fleet).Run()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")

		return fmt.Errorf("failed to insert default fleet row: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

func (db *Database) GetFleet(ctx context.Context) (*Fleet, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "SELECT", FleetTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("SELECT"),
			attribute.String("db.collection", FleetTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(FleetTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(FleetTableName, "select").Inc()

	var fleet Fleet

	err := db.conn.Query(ctx, db.getFleetStmt).Get(&fleet)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return nil, fmt.Errorf("failed to get fleet: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return &fleet, nil
}

func (db *Database) LoadOrGenerateFleetKey(ctx context.Context) (*ecdsa.PrivateKey, error) {
	fleet, err := db.GetFleet(ctx)
	if err != nil {
		return nil, fmt.Errorf("loading fleet from database: %w", err)
	}

	if len(fleet.PrivateKey) > 0 {
		key, err := x509.ParseECPrivateKey(fleet.PrivateKey)
		if err == nil {
			return key, nil
		}
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generating private key: %w", err)
	}

	if err := db.UpdateFleetKey(ctx, key); err != nil {
		return nil, fmt.Errorf("storing generated fleet key in database: %w", err)
	}

	return key, nil
}

func (db *Database) UpdateFleetKey(ctx context.Context, key *ecdsa.PrivateKey) error {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "UPDATE", FleetTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("UPDATE"),
			attribute.String("db.collection", FleetTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(FleetTableName, "update"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(FleetTableName, "update").Inc()

	keyBytes, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return fmt.Errorf("marshalling fleet private key: %w", err)
	}

	fleet := Fleet{
		PrivateKey: keyBytes,
	}

	err = db.conn.Query(ctx, db.updateFleetKeyStmt, fleet).Run()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")

		return fmt.Errorf("failed to update fleet key: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

func (db *Database) UpdateFleetCredentials(ctx context.Context, certificate []byte, caCertificate []byte) error {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "UPDATE", FleetTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("UPDATE"),
			attribute.String("db.collection", FleetTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(FleetTableName, "update"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(FleetTableName, "update").Inc()

	fleet := Fleet{
		Certificate:   certificate,
		CACertificate: caCertificate,
	}

	err := db.conn.Query(ctx, db.updateFleetCredentialsStmt, fleet).Run()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")

		return fmt.Errorf("failed to update fleet credentials: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return nil
}
