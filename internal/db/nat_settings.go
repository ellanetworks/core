package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

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

// InitializeNATSettings inserts the default NAT settings row if the
// singleton row does not yet exist. Idempotent: an existing row (whether
// holding the default or an operator-set value) is left untouched.
func (db *Database) InitializeNATSettings(ctx context.Context) error {
	_, err := db.IsNATEnabled(ctx)
	if err == nil {
		return nil
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("failed to check NAT settings: %w", err)
	}

	return db.UpdateNATSettings(ctx, NATDefaultEnabled)
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

	err := db.conn().Query(ctx, db.getNATSettingsStmt).Get(&natSettings)
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

	_, err := db.applyUpdateNATSettings(ctx, &boolPayload{Value: enabled})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return err
	}

	db.publishOpTopics([]Topic{TopicNATSettings}, 0)
	span.SetStatus(codes.Ok, "")

	return nil
}
