package db

import (
	"context"
	"fmt"

	"github.com/canonical/sqlair"
	"github.com/ellanetworks/core/internal/logger"
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

const insertDefaultN3SettingsStmt = `INSERT OR IGNORE INTO %s (singleton, external_address) VALUES (TRUE, $N3Settings.external_address);`

const upsertN3SettingsStmt = `
INSERT INTO %s (singleton, external_address) VALUES (TRUE, $N3Settings.external_address)
ON CONFLICT(singleton) DO UPDATE SET external_address=$N3Settings.external_address;
`

const getN3SettingsStmt = `SELECT &N3Settings.* FROM %s WHERE singleton=TRUE;`

type N3Settings struct {
	ExternalAddress string `db:"external_address"`
}

func (db *Database) InitializeN3Settings(ctx context.Context) error {
	operation := "INSERT"
	target := N3SettingsTableName
	spanName := fmt.Sprintf("%s %s", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	stmt := fmt.Sprintf(insertDefaultN3SettingsStmt, db.n3SettingsTable)
	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(stmt),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
	)

	q, err := sqlair.Prepare(stmt, N3Settings{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "prepare failed")
		return fmt.Errorf("failed to prepare insert default N3 settings statement: %w", err)
	}

	n3Settings := N3Settings{ExternalAddress: N3DefaultExternalAddress}

	if err := db.conn.Query(ctx, q, n3Settings).Run(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")
		return fmt.Errorf("failed to insert default N3 settings: %w", err)
	}

	logger.DBLog.Info("Initialized N3 settings")
	span.SetStatus(codes.Ok, "")
	return nil
}

func (db *Database) UpdateN3Settings(ctx context.Context, externalAddress string) error {
	operation := "UPSERT"
	target := N3SettingsTableName
	spanName := fmt.Sprintf("%s %s", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	stmt := fmt.Sprintf(upsertN3SettingsStmt, db.n3SettingsTable)
	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(stmt),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
	)

	q, err := sqlair.Prepare(stmt, N3Settings{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "prepare failed")
		return fmt.Errorf("failed to prepare upsert N3 settings statement: %w", err)
	}

	arg := N3Settings{ExternalAddress: externalAddress}
	if err := db.conn.Query(ctx, q, arg).Run(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")
		return fmt.Errorf("failed to upsert N3 settings: %w", err)
	}

	span.SetStatus(codes.Ok, "")
	return nil
}

func (db *Database) GetN3Settings(ctx context.Context) (*N3Settings, error) {
	operation := "SELECT"
	target := N3SettingsTableName
	spanName := fmt.Sprintf("%s %s", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	stmt := fmt.Sprintf(getN3SettingsStmt, db.n3SettingsTable)
	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(stmt),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
	)

	q, err := sqlair.Prepare(stmt, N3Settings{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "prepare failed")
		return nil, fmt.Errorf("failed to prepare get N3 settings statement: %w", err)
	}

	var n3Settings N3Settings
	if err := db.conn.Query(ctx, q).Get(&n3Settings); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")
		return nil, fmt.Errorf("failed to get N3 settings: %w", err)
	}

	span.SetStatus(codes.Ok, "")
	return &n3Settings, nil
}
