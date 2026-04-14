package db

import (
	"context"
	"fmt"

	ellaraft "github.com/ellanetworks/core/internal/raft"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
)

const (
	N3DefaultExternalAddress = ""
)

const N3SettingsTableName = "n3_settings"

const insertDefaultN3SettingsStmt = `
INSERT OR IGNORE INTO %s (singleton, external_address)
VALUES (TRUE, $N3Settings.external_address);
`

const upsertN3SettingsStmt = `
INSERT INTO %s (singleton, external_address) VALUES (TRUE, $N3Settings.external_address)
ON CONFLICT(singleton) DO UPDATE SET external_address=$N3Settings.external_address;
`

const getN3SettingsStmt = `SELECT &N3Settings.* FROM %s WHERE singleton=TRUE;`

type N3Settings struct {
	ExternalAddress string `db:"external_address"`
}

func (db *Database) InitializeN3Settings(ctx context.Context) error {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "INSERT", N3SettingsTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("INSERT"),
			attribute.String("db.collection", N3SettingsTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(N3SettingsTableName, "insert"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(N3SettingsTableName, "insert").Inc()

	n3Settings := N3Settings{ExternalAddress: N3DefaultExternalAddress}

	if err := db.conn.Query(ctx, db.insertDefaultN3SettingsStmt, n3Settings).Run(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

func (db *Database) UpdateN3Settings(ctx context.Context, externalAddress string) error {
	_, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "UPSERT", N3SettingsTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("UPSERT"),
			attribute.String("db.collection", N3SettingsTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(N3SettingsTableName, "update"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(N3SettingsTableName, "update").Inc()

	_, err := db.propose(ellaraft.CmdUpdateN3Settings, &stringPayload{Value: externalAddress})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return err
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

func (db *Database) GetN3Settings(ctx context.Context) (*N3Settings, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "SELECT", N3SettingsTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			attribute.String("db.collection", N3SettingsTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(N3SettingsTableName, "select"))
	defer timer.ObserveDuration()

	var n3Settings N3Settings

	if err := db.conn.Query(ctx, db.getN3SettingsStmt).Get(&n3Settings); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return nil, fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return &n3Settings, nil
}
