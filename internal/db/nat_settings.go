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
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "INSERT", NATSettingsTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("INSERT"),
			attribute.String("db.collection", NATSettingsTableName),
		),
	)
	defer span.End()

	natSettings := NATSettings{Enabled: NATDefaultEnabled}

	err := db.conn.Query(ctx, db.insertDefaultNATSettingsStmt, natSettings).Run()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")

		return fmt.Errorf("failed to insert default NAT settings: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

func (db *Database) IsNATEnabled(ctx context.Context) (bool, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "SELECT", NATSettingsTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("SELECT"),
			attribute.String("db.collection", NATSettingsTableName),
		),
	)
	defer span.End()

	var natSettings NATSettings

	err := db.conn.Query(ctx, db.getNATSettingsStmt).Get(&natSettings)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return false, fmt.Errorf("failed to get NAT settings: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return natSettings.Enabled, nil
}

func (db *Database) UpdateNATSettings(ctx context.Context, enabled bool) error {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "UPSERT", NATSettingsTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("UPSERT"),
			attribute.String("db.collection", NATSettingsTableName),
		),
	)
	defer span.End()

	arg := NATSettings{Enabled: enabled}

	err := db.conn.Query(ctx, db.upsertNATSettingsStmt, arg).Run()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")

		return fmt.Errorf("failed to upsert NAT settings: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return nil
}
