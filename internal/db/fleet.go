// Copyright 2026 Ella Networks

package db

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
)

const FleetTableName = "fleet"

const (
	getFleetStmt                  = "SELECT &Fleet.* FROM %s WHERE singleton"
	initializeFleetStmt           = "INSERT OR IGNORE INTO %s (singleton, url, private_key, certificate, ca_certificate, last_sync_at, config_revision) VALUES (TRUE, $Fleet.url, $Fleet.private_key, $Fleet.certificate, $Fleet.ca_certificate, $Fleet.last_sync_at, $Fleet.config_revision)"
	updateFleetKeyStmt            = "UPDATE %s SET private_key=$Fleet.private_key WHERE singleton"
	updateFleetCredentialsStmt    = "UPDATE %s SET certificate=$Fleet.certificate, ca_certificate=$Fleet.ca_certificate WHERE singleton"
	clearFleetCredentialsStmt     = "UPDATE %s SET certificate=$Fleet.certificate, ca_certificate=$Fleet.ca_certificate, last_sync_at=$Fleet.last_sync_at, config_revision=$Fleet.config_revision WHERE singleton"
	updateFleetSyncStatusStmt     = "UPDATE %s SET last_sync_at=$Fleet.last_sync_at WHERE singleton"
	updateFleetConfigRevisionStmt = "UPDATE %s SET config_revision=$Fleet.config_revision WHERE singleton"
	updateFleetURLStmt            = "UPDATE %s SET url=$Fleet.url WHERE singleton"
)

type Fleet struct {
	URL            string `db:"url" json:"url"`
	PrivateKey     []byte `db:"private_key" json:"private_key"`
	Certificate    []byte `db:"certificate" json:"certificate"`
	CACertificate  []byte `db:"ca_certificate" json:"ca_certificate"`
	LastSyncAt     string `db:"last_sync_at" json:"last_sync_at"`
	ConfigRevision int64  `db:"config_revision" json:"config_revision"`
}

// InitializeFleet inserts the default fleet row if it does not exist.
func (db *Database) InitializeFleet(ctx context.Context) error {
	_, err := db.GetFleet(ctx)
	if err == nil {
		return nil
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("failed to check fleet row: %w", err)
	}

	_, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "INSERT", FleetTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("INSERT"),
			attribute.String("db.collection", FleetTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(FleetTableName, "insert"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(FleetTableName, "insert").Inc()

	err = db.applyInitializeFleet(ctx, &Fleet{
		URL:            "",
		PrivateKey:     []byte{},
		Certificate:    []byte{},
		CACertificate:  []byte{},
		LastSyncAt:     "",
		ConfigRevision: 0,
	})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return err
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

func (db *Database) applyInitializeFleet(ctx context.Context, fleet *Fleet) error {
	if err := db.runner(ctx).Query(ctx, db.initializeFleetStmt, *fleet).Run(); err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	return nil
}

// GetFleet retrieves the singleton fleet row. Returns sql.ErrNoRows when
// the row has not yet been initialized.
func (db *Database) GetFleet(ctx context.Context) (*Fleet, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "SELECT", FleetTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			attribute.String("db.collection", FleetTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(FleetTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(FleetTableName, "select").Inc()

	var fleet Fleet

	err := db.conn().Query(ctx, db.getFleetStmt).Get(&fleet)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return nil, err
	}

	span.SetStatus(codes.Ok, "")

	return &fleet, nil
}

// LoadOrGenerateFleetKey returns the Core's fleet client key, generating
// and persisting a fresh ECDSA P-256 key on first use.
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
	_, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s (key)", "UPDATE", FleetTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("UPDATE"),
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

	err = db.applyUpdateFleetKey(ctx, &Fleet{PrivateKey: keyBytes})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return err
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

func (db *Database) applyUpdateFleetKey(ctx context.Context, fleet *Fleet) error {
	if err := db.runner(ctx).Query(ctx, db.updateFleetKeyStmt, *fleet).Run(); err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	return nil
}

func (db *Database) UpdateFleetCredentials(ctx context.Context, certificate []byte, caCertificate []byte) error {
	_, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s (credentials)", "UPDATE", FleetTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("UPDATE"),
			attribute.String("db.collection", FleetTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(FleetTableName, "update"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(FleetTableName, "update").Inc()

	err := db.applyUpdateFleetCredentials(ctx, &Fleet{Certificate: certificate, CACertificate: caCertificate})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return err
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

func (db *Database) applyUpdateFleetCredentials(ctx context.Context, fleet *Fleet) error {
	if err := db.runner(ctx).Query(ctx, db.updateFleetCredentialsStmt, *fleet).Run(); err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	return nil
}

// IsFleetManaged returns true when the Core is registered to a Fleet
// (has non-empty certificate and CA certificate).
func (db *Database) IsFleetManaged(ctx context.Context) (bool, error) {
	fleet, err := db.GetFleet(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}

		return false, err
	}

	return len(fleet.Certificate) > 0 && len(fleet.CACertificate) > 0, nil
}

// UpdateFleetSyncStatus records the timestamp of the last successful sync.
func (db *Database) UpdateFleetSyncStatus(ctx context.Context) error {
	_, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s (sync status)", "UPDATE", FleetTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("UPDATE"),
			attribute.String("db.collection", FleetTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(FleetTableName, "update"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(FleetTableName, "update").Inc()

	ts := time.Now().UTC().Format(time.RFC3339)

	err := db.applyUpdateFleetSyncStatus(ctx, &Fleet{LastSyncAt: ts})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return err
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

func (db *Database) applyUpdateFleetSyncStatus(ctx context.Context, fleet *Fleet) error {
	if err := db.runner(ctx).Query(ctx, db.updateFleetSyncStatusStmt, *fleet).Run(); err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	return nil
}

// ClearFleetCredentials removes the certificate, CA certificate, sync
// status, and config revision — effectively unregistering the Core.
func (db *Database) ClearFleetCredentials(ctx context.Context) error {
	_, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s (clear)", "UPDATE", FleetTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("UPDATE"),
			attribute.String("db.collection", FleetTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(FleetTableName, "update"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(FleetTableName, "update").Inc()

	err := db.applyClearFleetCredentials(ctx, &Fleet{
		Certificate:    []byte{},
		CACertificate:  []byte{},
		LastSyncAt:     "",
		ConfigRevision: 0,
	})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return err
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

func (db *Database) applyClearFleetCredentials(ctx context.Context, fleet *Fleet) error {
	if err := db.runner(ctx).Query(ctx, db.clearFleetCredentialsStmt, *fleet).Run(); err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	return nil
}

// UpdateFleetConfigRevision stores the latest config revision received from Fleet.
func (db *Database) UpdateFleetConfigRevision(ctx context.Context, revision int64) error {
	_, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s (config revision)", "UPDATE", FleetTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("UPDATE"),
			attribute.String("db.collection", FleetTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(FleetTableName, "update"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(FleetTableName, "update").Inc()

	err := db.applyUpdateFleetConfigRevision(ctx, &Fleet{ConfigRevision: revision})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return err
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

func (db *Database) applyUpdateFleetConfigRevision(ctx context.Context, fleet *Fleet) error {
	if err := db.runner(ctx).Query(ctx, db.updateFleetConfigRevisionStmt, *fleet).Run(); err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	return nil
}

// UpdateFleetURL stores the user-specified Fleet server URL.
func (db *Database) UpdateFleetURL(ctx context.Context, url string) error {
	_, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s (url)", "UPDATE", FleetTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("UPDATE"),
			attribute.String("db.collection", FleetTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(FleetTableName, "update"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(FleetTableName, "update").Inc()

	err := db.applyUpdateFleetURL(ctx, &Fleet{URL: url})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return err
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

func (db *Database) applyUpdateFleetURL(ctx context.Context, fleet *Fleet) error {
	if err := db.runner(ctx).Query(ctx, db.updateFleetURLStmt, *fleet).Run(); err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	return nil
}

// GetFleetURL returns the user-specified Fleet server URL.
func (db *Database) GetFleetURL(ctx context.Context) (string, error) {
	fleet, err := db.GetFleet(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}

		return "", err
	}

	return fleet.URL, nil
}
