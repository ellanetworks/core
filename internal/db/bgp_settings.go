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
	BGPDefaultEnabled = false
	BGPDefaultLocalAS = 64512
)

const BGPSettingsTableName = "bgp_settings"

const insertDefaultBGPSettingsStmt = `INSERT OR IGNORE INTO %s (singleton, enabled, localAS, routerID, listenAddress) VALUES (TRUE, $BGPSettings.enabled, $BGPSettings.localAS, $BGPSettings.routerID, $BGPSettings.listenAddress);`

const upsertBGPSettingsStmt = `
INSERT INTO %s (singleton, enabled, localAS, routerID, listenAddress) VALUES (TRUE, $BGPSettings.enabled, $BGPSettings.localAS, $BGPSettings.routerID, $BGPSettings.listenAddress)
ON CONFLICT(singleton) DO UPDATE SET enabled=$BGPSettings.enabled, localAS=$BGPSettings.localAS, routerID=$BGPSettings.routerID, listenAddress=$BGPSettings.listenAddress;
`

const getBGPSettingsStmt = `SELECT &BGPSettings.* FROM %s WHERE singleton=TRUE;`

type BGPSettings struct {
	Enabled       bool   `db:"enabled"`
	LocalAS       int    `db:"localAS"`
	RouterID      string `db:"routerID"`
	ListenAddress string `db:"listenAddress"`
}

// InitializeBGPSettings inserts the default BGP settings into the database.
// If the settings already exist, it does nothing.
func (db *Database) InitializeBGPSettings(ctx context.Context) error {
	_, err := db.GetBGPSettings(ctx)
	if err == nil {
		return nil
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("failed to check BGP settings: %w", err)
	}

	return db.UpdateBGPSettings(ctx, &BGPSettings{
		Enabled:       BGPDefaultEnabled,
		LocalAS:       BGPDefaultLocalAS,
		RouterID:      "",
		ListenAddress: ":179",
	})
}

func (db *Database) GetBGPSettings(ctx context.Context) (*BGPSettings, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "SELECT", BGPSettingsTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			attribute.String("db.collection", BGPSettingsTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(BGPSettingsTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(BGPSettingsTableName, "select").Inc()

	var bgpSettings BGPSettings

	err := db.conn().Query(ctx, db.getBGPSettingsStmt).Get(&bgpSettings)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return nil, fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return &bgpSettings, nil
}

func (db *Database) IsBGPEnabled(ctx context.Context) (bool, error) {
	settings, err := db.GetBGPSettings(ctx)
	if err != nil {
		return false, err
	}

	return settings.Enabled, nil
}

func (db *Database) UpdateBGPSettings(ctx context.Context, settings *BGPSettings) error {
	_, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "UPSERT", BGPSettingsTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("UPSERT"),
			attribute.String("db.collection", BGPSettingsTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(BGPSettingsTableName, "update"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(BGPSettingsTableName, "update").Inc()

	_, err := opUpdateBGPSettings.Invoke(db, settings)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return err
	}

	span.SetStatus(codes.Ok, "")

	return nil
}
