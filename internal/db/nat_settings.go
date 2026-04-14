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
	NATDefaultEnabled = true
)

const NATSettingsTableName = "nat_settings"

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
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("INSERT"),
			attribute.String("db.collection", NATSettingsTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(NATSettingsTableName, "insert"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(NATSettingsTableName, "insert").Inc()

	natSettings := NATSettings{Enabled: NATDefaultEnabled}

	err := db.conn.Query(ctx, db.insertDefaultNATSettingsStmt, natSettings).Run()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return fmt.Errorf("query failed: %w", err)
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
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			attribute.String("db.collection", NATSettingsTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(NATSettingsTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(NATSettingsTableName, "select").Inc()

	var natSettings NATSettings

	err := db.conn.Query(ctx, db.getNATSettingsStmt).Get(&natSettings)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return false, fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return natSettings.Enabled, nil
}

func (db *Database) UpdateNATSettings(ctx context.Context, enabled bool) error {
	_, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "UPSERT", NATSettingsTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("UPSERT"),
			attribute.String("db.collection", NATSettingsTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(NATSettingsTableName, "update"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(NATSettingsTableName, "update").Inc()

	_, err := db.propose(ellaraft.CmdUpdateNATSettings, &boolPayload{Value: enabled})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return err
	}

	span.SetStatus(codes.Ok, "")

	return nil
}
