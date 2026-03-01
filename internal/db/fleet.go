package db

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"fmt"
	"time"

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
	url TEXT NOT NULL DEFAULT '',
	private_key BLOB NOT NULL,
	certificate BLOB NOT NULL,
	ca_certificate BLOB NOT NULL,
	last_sync_at TEXT NOT NULL DEFAULT '',
	config_revision INTEGER NOT NULL DEFAULT 0,
  CHECK (singleton)
);
`

const (
	getFleetStmt                  = "SELECT &Fleet.* FROM %s WHERE singleton"
	updateFleetKeyStmt            = "UPDATE %s SET private_key=$Fleet.private_key WHERE singleton"
	updateFleetCredentialsStmt    = "UPDATE %s SET certificate=$Fleet.certificate, ca_certificate=$Fleet.ca_certificate WHERE singleton"
	clearFleetCredentialsStmt     = "UPDATE %s SET certificate=$Fleet.certificate, ca_certificate=$Fleet.ca_certificate, last_sync_at=$Fleet.last_sync_at, config_revision=$Fleet.config_revision WHERE singleton"
	initializeFleetStmt           = "INSERT OR IGNORE INTO %s (singleton, enabled, url, private_key, certificate, ca_certificate, last_sync_at, config_revision) VALUES (TRUE, TRUE, $Fleet.url, $Fleet.private_key, $Fleet.certificate, $Fleet.ca_certificate, $Fleet.last_sync_at, $Fleet.config_revision)"
	updateFleetSyncStatusStmt     = "UPDATE %s SET last_sync_at=$Fleet.last_sync_at WHERE singleton"
	updateFleetConfigRevisionStmt = "UPDATE %s SET config_revision=$Fleet.config_revision WHERE singleton"
	updateFleetURLStmt            = "UPDATE %s SET url=$Fleet.url WHERE singleton"
)

type Fleet struct {
	Enabled        bool   `db:"enabled"`
	URL            string `db:"url"`
	PrivateKey     []byte `db:"private_key"`
	Certificate    []byte `db:"certificate"`
	CACertificate  []byte `db:"ca_certificate"`
	LastSyncAt     string `db:"last_sync_at"`
	ConfigRevision int64  `db:"config_revision"`
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
		URL:            "",
		PrivateKey:     []byte{},
		Certificate:    []byte{},
		CACertificate:  []byte{},
		LastSyncAt:     "",
		ConfigRevision: 0,
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

// IsFleetManaged returns true when the Core has been registered to a Fleet
// (i.e. it has non-empty certificate and CA certificate).
func (db *Database) IsFleetManaged(ctx context.Context) (bool, error) {
	fleet, err := db.GetFleet(ctx)
	if err != nil {
		return false, err
	}

	return len(fleet.Certificate) > 0 && len(fleet.CACertificate) > 0, nil
}

// UpdateFleetSyncStatus records the timestamp of the last successful sync.
func (db *Database) UpdateFleetSyncStatus(ctx context.Context) error {
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
		LastSyncAt: time.Now().UTC().Format(time.RFC3339),
	}

	err := db.conn.Query(ctx, db.updateFleetSyncStatusStmt, fleet).Run()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")

		return fmt.Errorf("failed to update fleet sync status: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

// ClearFleetCredentials removes the certificate and CA certificate,
// effectively unregistering the Core from Fleet.
func (db *Database) ClearFleetCredentials(ctx context.Context) error {
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
		Certificate:    []byte{},
		CACertificate:  []byte{},
		LastSyncAt:     "",
		ConfigRevision: 0,
	}

	err := db.conn.Query(ctx, db.clearFleetCredentialsStmt, fleet).Run()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")

		return fmt.Errorf("failed to clear fleet credentials: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

// UpdateFleetConfigRevision stores the latest config revision received from Fleet.
func (db *Database) UpdateFleetConfigRevision(ctx context.Context, revision int64) error {
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
		ConfigRevision: revision,
	}

	err := db.conn.Query(ctx, db.updateFleetConfigRevisionStmt, fleet).Run()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")

		return fmt.Errorf("failed to update fleet config revision: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

// UpdateFleetURL stores the user-specified Fleet server URL.
func (db *Database) UpdateFleetURL(ctx context.Context, url string) error {
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
		URL: url,
	}

	err := db.conn.Query(ctx, db.updateFleetURLStmt, fleet).Run()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")

		return fmt.Errorf("failed to update fleet URL: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

// GetFleetURL returns the user-specified Fleet server URL.
func (db *Database) GetFleetURL(ctx context.Context) (string, error) {
	fleet, err := db.GetFleet(ctx)
	if err != nil {
		return "", err
	}

	return fleet.URL, nil
}
