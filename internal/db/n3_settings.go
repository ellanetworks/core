package db

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
)

const (
	N3DefaultExternalAddress = ""
)

const N3SettingsTableName = "n3_settings"

const QueryCreateN3SettingsTable = `
CREATE TABLE IF NOT EXISTS %s (
  singleton BOOLEAN PRIMARY KEY DEFAULT TRUE,
  external_address   TEXT NOT NULL DEFAULT TRUE,
  CHECK (singleton)
);
`

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
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("INSERT"),
			attribute.String("db.collection", N3SettingsTableName),
		),
	)
	defer span.End()

	n3Settings := N3Settings{ExternalAddress: N3DefaultExternalAddress}

	if err := db.conn.Query(ctx, db.insertDefaultN3SettingsStmt, n3Settings).Run(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")
		return fmt.Errorf("failed to insert default N3 settings: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

func (db *Database) UpdateN3Settings(ctx context.Context, externalAddress string) error {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "UPSERT", N3SettingsTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("UPSERT"),
			attribute.String("db.collection", N3SettingsTableName),
		),
	)
	defer span.End()

	arg := N3Settings{ExternalAddress: externalAddress}

	err := db.conn.Query(ctx, db.updateN3SettingsStmt, arg).Run()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")
		return fmt.Errorf("failed to upsert N3 settings: %w", err)
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
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("SELECT"),
			attribute.String("db.collection", N3SettingsTableName),
		),
	)
	defer span.End()

	var n3Settings N3Settings

	if err := db.conn.Query(ctx, db.getN3SettingsStmt).Get(&n3Settings); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")
		return nil, fmt.Errorf("failed to get N3 settings: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return &n3Settings, nil
}
