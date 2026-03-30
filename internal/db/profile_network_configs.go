// Copyright 2026 Ella Networks

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/canonical/sqlair"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
)

const ProfileNetworkConfigsTableName = "profile_network_configs"

const (
	listProfileNetworkConfigsByProfileStmt = "SELECT &ProfileNetworkConfig.* FROM %s WHERE profileID==$ProfileNetworkConfig.profileID"
	getProfileNetworkConfigStmt            = "SELECT &ProfileNetworkConfig.* FROM %s WHERE profileID==$ProfileNetworkConfig.profileID AND sliceID==$ProfileNetworkConfig.sliceID AND dataNetworkID==$ProfileNetworkConfig.dataNetworkID"
	createProfileNetworkConfigStmt         = "INSERT INTO %s (profileID, sliceID, dataNetworkID, var5qi, arp, sessionAmbrUplink, sessionAmbrDownlink) VALUES ($ProfileNetworkConfig.profileID, $ProfileNetworkConfig.sliceID, $ProfileNetworkConfig.dataNetworkID, $ProfileNetworkConfig.var5qi, $ProfileNetworkConfig.arp, $ProfileNetworkConfig.sessionAmbrUplink, $ProfileNetworkConfig.sessionAmbrDownlink)"
	updateProfileNetworkConfigStmt         = "UPDATE %s SET var5qi=$ProfileNetworkConfig.var5qi, arp=$ProfileNetworkConfig.arp, sessionAmbrUplink=$ProfileNetworkConfig.sessionAmbrUplink, sessionAmbrDownlink=$ProfileNetworkConfig.sessionAmbrDownlink, dataNetworkID=$ProfileNetworkConfig.dataNetworkID WHERE id==$ProfileNetworkConfig.id"
	deleteProfileNetworkConfigStmt         = "DELETE FROM %s WHERE profileID==$ProfileNetworkConfig.profileID AND sliceID==$ProfileNetworkConfig.sliceID AND dataNetworkID==$ProfileNetworkConfig.dataNetworkID"
	countConfigsInDataNetworkStmt          = "SELECT COUNT(*) AS &NumItems.count FROM %s WHERE dataNetworkID==$ProfileNetworkConfig.dataNetworkID"
	countConfigsInSliceStmt                = "SELECT COUNT(*) AS &NumItems.count FROM %s WHERE sliceID==$ProfileNetworkConfig.sliceID"
)

type ProfileNetworkConfig struct {
	ID                  int    `db:"id"`
	ProfileID           int    `db:"profileID"`
	SliceID             int    `db:"sliceID"`
	DataNetworkID       int    `db:"dataNetworkID"`
	Var5qi              int32  `db:"var5qi"`
	Arp                 int32  `db:"arp"`
	SessionAmbrUplink   string `db:"sessionAmbrUplink"`
	SessionAmbrDownlink string `db:"sessionAmbrDownlink"`
}

func (db *Database) ListProfileNetworkConfigs(ctx context.Context, profileID int) ([]ProfileNetworkConfig, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "SELECT", ProfileNetworkConfigsTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			attribute.String("db.collection", ProfileNetworkConfigsTableName),
			attribute.Int("profile_id", profileID),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(ProfileNetworkConfigsTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(ProfileNetworkConfigsTableName, "select").Inc()

	var configs []ProfileNetworkConfig

	err := db.conn.Query(ctx, db.listProfileNetworkConfigsByProfileStmt, ProfileNetworkConfig{ProfileID: profileID}).GetAll(&configs)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			span.SetStatus(codes.Ok, "no rows")
			return nil, nil
		}

		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return nil, fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return configs, nil
}

func (db *Database) GetProfileNetworkConfig(ctx context.Context, profileID, sliceID, dataNetworkID int) (*ProfileNetworkConfig, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "SELECT", ProfileNetworkConfigsTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			attribute.String("db.collection", ProfileNetworkConfigsTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(ProfileNetworkConfigsTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(ProfileNetworkConfigsTableName, "select").Inc()

	row := ProfileNetworkConfig{
		ProfileID:     profileID,
		SliceID:       sliceID,
		DataNetworkID: dataNetworkID,
	}

	err := db.conn.Query(ctx, db.getProfileNetworkConfigStmt, row).Get(&row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			span.SetStatus(codes.Ok, "no rows")
			return nil, ErrNotFound
		}

		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return nil, fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return &row, nil
}

func (db *Database) CreateProfileNetworkConfig(ctx context.Context, config *ProfileNetworkConfig) error {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "INSERT", ProfileNetworkConfigsTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("INSERT"),
			attribute.String("db.collection", ProfileNetworkConfigsTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(ProfileNetworkConfigsTableName, "insert"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(ProfileNetworkConfigsTableName, "insert").Inc()

	err := db.conn.Query(ctx, db.createProfileNetworkConfigStmt, config).Run()
	if err != nil {
		if isUniqueNameError(err) {
			span.RecordError(ErrAlreadyExists)
			span.SetStatus(codes.Error, "unique constraint failed")

			return ErrAlreadyExists
		}

		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

func (db *Database) UpdateProfileNetworkConfig(ctx context.Context, config *ProfileNetworkConfig) error {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "UPDATE", ProfileNetworkConfigsTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("UPDATE"),
			attribute.String("db.collection", ProfileNetworkConfigsTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(ProfileNetworkConfigsTableName, "update"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(ProfileNetworkConfigsTableName, "update").Inc()

	var outcome sqlair.Outcome

	err := db.conn.Query(ctx, db.updateProfileNetworkConfigStmt, config).Get(&outcome)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return fmt.Errorf("query failed: %w", err)
	}

	rowsAffected, err := outcome.Result().RowsAffected()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "retrieving rows affected failed")

		return fmt.Errorf("retrieving rows affected failed: %w", err)
	}

	if rowsAffected == 0 {
		span.RecordError(ErrNotFound)
		span.SetStatus(codes.Error, "not found")

		return ErrNotFound
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

func (db *Database) DeleteProfileNetworkConfig(ctx context.Context, profileID, sliceID, dataNetworkID int) error {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "DELETE", ProfileNetworkConfigsTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("DELETE"),
			attribute.String("db.collection", ProfileNetworkConfigsTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(ProfileNetworkConfigsTableName, "delete"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(ProfileNetworkConfigsTableName, "delete").Inc()

	var outcome sqlair.Outcome

	err := db.conn.Query(ctx, db.deleteProfileNetworkConfigStmt, ProfileNetworkConfig{
		ProfileID:     profileID,
		SliceID:       sliceID,
		DataNetworkID: dataNetworkID,
	}).Get(&outcome)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return fmt.Errorf("query failed: %w", err)
	}

	rowsAffected, err := outcome.Result().RowsAffected()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "retrieving rows affected failed")

		return fmt.Errorf("retrieving rows affected failed: %w", err)
	}

	if rowsAffected == 0 {
		span.RecordError(ErrNotFound)
		span.SetStatus(codes.Error, "not found")

		return ErrNotFound
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

// NetworkConfigsInDataNetwork checks if any profile_network_configs reference
// the given data network name. Used as a deletion guard.
func (db *Database) NetworkConfigsInDataNetwork(ctx context.Context, dnnName string) (bool, error) {
	ctx, span := tracer.Start(
		ctx,
		"NetworkConfigsInDataNetwork",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
		),
	)
	defer span.End()

	dataNetwork, err := db.GetDataNetwork(ctx, dnnName)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "data network not found")

		return false, fmt.Errorf("data network not found: %w", err)
	}

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(ProfileNetworkConfigsTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(ProfileNetworkConfigsTableName, "select").Inc()

	var result NumItems

	err = db.conn.Query(ctx, db.countConfigsInDataNetworkStmt, ProfileNetworkConfig{DataNetworkID: dataNetwork.ID}).Get(&result)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return false, fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return result.Count > 0, nil
}

// NetworkConfigsInSlice checks if any profile_network_configs reference the
// given slice ID. Used as a deletion guard.
func (db *Database) NetworkConfigsInSlice(ctx context.Context, sliceID int) (bool, error) {
	ctx, span := tracer.Start(
		ctx,
		"NetworkConfigsInSlice",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(ProfileNetworkConfigsTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(ProfileNetworkConfigsTableName, "select").Inc()

	var result NumItems

	err := db.conn.Query(ctx, db.countConfigsInSliceStmt, ProfileNetworkConfig{SliceID: sliceID}).Get(&result)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return false, fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return result.Count > 0, nil
}

// GetSessionPolicy resolves the session QoS configuration for a subscriber's
// PDU session by IMSI, S-NSSAI (sst/sd), and DNN. Returns the matching
// profile_network_configs row if authorized, or ErrNotFound if the subscriber's
// profile does not authorize the requested (slice, DNN) pair.
func (db *Database) GetSessionPolicy(ctx context.Context, imsi string, sst int32, sd *string, dnn string) (*ProfileNetworkConfig, error) {
	ctx, span := tracer.Start(
		ctx,
		"GetSessionPolicy",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
		),
	)
	defer span.End()

	subscriber, err := db.GetSubscriber(ctx, imsi)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "subscriber not found")

		return nil, fmt.Errorf("subscriber not found: %w", err)
	}

	slice, err := db.GetNetworkSliceBySstSd(ctx, sst, sd)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "network slice not found")

		return nil, fmt.Errorf("network slice not found: %w", err)
	}

	dataNetwork, err := db.GetDataNetwork(ctx, dnn)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "data network not found")

		return nil, fmt.Errorf("data network not found: %w", err)
	}

	config, err := db.GetProfileNetworkConfig(ctx, subscriber.ProfileID, slice.ID, dataNetwork.ID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "session policy not found")

		return nil, fmt.Errorf("session policy not found for (slice=%d, dnn=%s): %w", slice.ID, dnn, err)
	}

	span.SetStatus(codes.Ok, "")

	return config, nil
}
