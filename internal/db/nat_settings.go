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
	NATDefaultEnabled = true
)

const NATSettingsTableName = "nat_settings"

const QueryCreateNATSettingsTable = `
CREATE TABLE IF NOT EXISTS %s (
  singleton BOOLEAN PRIMARY KEY DEFAULT TRUE,
  enabled   BOOLEAN NOT NULL DEFAULT TRUE,
  CHECK (singleton)
);
`

const insertDefaultNATSettingsStmt = `INSERT OR IGNORE INTO %s (singleton, enabled) VALUES (TRUE, $NATSettings.enabled);`

const upsertNATSettingsStmt = `
INSERT INTO %s (singleton, enabled) VALUES (TRUE, $NATSettings.enabled)
ON CONFLICT(singleton) DO UPDATE SET enabled=$NATSettings.enabled;
`

const getNATSettingsStmt = `SELECT &NATSettings.* FROM %s WHERE singleton=TRUE;`

type NATSettings struct {
	Enabled bool `db:"enabled"`
}

// InitializeNATSettings inserts the default NAT settings into the database.
// If the settings already exist, it does nothing.
func (db *Database) InitializeNATSettings(ctx context.Context) error {
	operation := "INSERT"
	target := NATSettingsTableName
	spanName := fmt.Sprintf("%s %s", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	stmt := fmt.Sprintf(insertDefaultNATSettingsStmt, db.natSettingsTable)
	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(stmt),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
	)

	q, err := sqlair.Prepare(stmt, NATSettings{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "prepare failed")
		return fmt.Errorf("failed to prepare insert default NAT settings statement: %w", err)
	}

	natSettings := NATSettings{Enabled: NATDefaultEnabled}

	if err := db.conn.Query(ctx, q, natSettings).Run(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")
		return fmt.Errorf("failed to insert default NAT settings: %w", err)
	}

	logger.DBLog.Debug("Initialized NAT settings")
	span.SetStatus(codes.Ok, "")
	return nil
}

func (db *Database) IsNATEnabled(ctx context.Context) (bool, error) {
	operation := "SELECT"
	target := NATSettingsTableName
	spanName := fmt.Sprintf("%s %s", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	stmt := fmt.Sprintf(getNATSettingsStmt, db.natSettingsTable)
	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(stmt),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
	)

	q, err := sqlair.Prepare(stmt, NATSettings{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "prepare failed")
		return false, fmt.Errorf("failed to prepare get NAT settings statement: %w", err)
	}

	var natSettings NATSettings
	if err := db.conn.Query(ctx, q).Get(&natSettings); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")
		return false, fmt.Errorf("failed to get NAT settings: %w", err)
	}

	span.SetStatus(codes.Ok, "")
	return natSettings.Enabled, nil
}

func (db *Database) UpdateNATSettings(ctx context.Context, enabled bool) error {
	operation := "UPSERT"
	target := NATSettingsTableName
	spanName := fmt.Sprintf("%s %s", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	stmt := fmt.Sprintf(upsertNATSettingsStmt, db.natSettingsTable)
	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(stmt),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
	)

	q, err := sqlair.Prepare(stmt, NATSettings{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "prepare failed")
		return fmt.Errorf("failed to prepare upsert NAT settings statement: %w", err)
	}

	arg := NATSettings{Enabled: enabled}
	if err := db.conn.Query(ctx, q, arg).Run(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")
		return fmt.Errorf("failed to upsert NAT settings: %w", err)
	}

	span.SetStatus(codes.Ok, "")
	return nil
}
