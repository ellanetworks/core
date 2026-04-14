// Copyright 2024 Ella Networks

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	ellaraft "github.com/ellanetworks/core/internal/raft"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
)

const ProfilesTableName = "profiles"

const (
	listProfilesPagedStmt         = "SELECT &Profile.*, COUNT(*) OVER() AS &NumItems.count FROM %s LIMIT $ListArgs.limit OFFSET $ListArgs.offset"
	getProfileStmt                = "SELECT &Profile.* FROM %s WHERE name==$Profile.name"
	getProfileByIDStmt            = "SELECT &Profile.* FROM %s WHERE id==$Profile.id"
	createProfileStmt             = "INSERT INTO %s (name, ueAmbrUplink, ueAmbrDownlink) VALUES ($Profile.name, $Profile.ueAmbrUplink, $Profile.ueAmbrDownlink)"
	editProfileStmt               = "UPDATE %s SET ueAmbrUplink=$Profile.ueAmbrUplink, ueAmbrDownlink=$Profile.ueAmbrDownlink WHERE name==$Profile.name"
	deleteProfileStmt             = "DELETE FROM %s WHERE name==$Profile.name"
	countProfilesStmt             = "SELECT COUNT(*) AS &NumItems.count FROM %s"
	countSubscribersInProfileStmt = "SELECT COUNT(*) AS &NumItems.count FROM %s WHERE profileID=$Subscriber.profileID"
)

type Profile struct {
	ID             int    `db:"id"`
	Name           string `db:"name"`
	UeAmbrUplink   string `db:"ueAmbrUplink"`
	UeAmbrDownlink string `db:"ueAmbrDownlink"`
}

func (db *Database) ListProfilesPage(ctx context.Context, page, perPage int) ([]Profile, int, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s (paged)", "SELECT", ProfilesTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			attribute.String("db.collection", ProfilesTableName),
			attribute.Int("page", page),
			attribute.Int("per_page", perPage),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(ProfilesTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(ProfilesTableName, "select").Inc()

	var profiles []Profile

	var counts []NumItems

	args := ListArgs{
		Limit:  perPage,
		Offset: (page - 1) * perPage,
	}

	err := db.shared.Query(ctx, db.listProfilesStmt, args).GetAll(&profiles, &counts)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			span.SetStatus(codes.Ok, "no rows")

			fallbackCount, countErr := db.CountProfiles(ctx)
			if countErr != nil {
				return nil, 0, nil
			}

			return nil, fallbackCount, nil
		}

		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return nil, 0, fmt.Errorf("query failed: %w", err)
	}

	count := 0
	if len(counts) > 0 {
		count = counts[0].Count
	}

	span.SetStatus(codes.Ok, "")

	return profiles, count, nil
}

func (db *Database) GetProfile(ctx context.Context, name string) (*Profile, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "SELECT", ProfilesTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			attribute.String("db.collection", ProfilesTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(ProfilesTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(ProfilesTableName, "select").Inc()

	row := Profile{Name: name}

	err := db.shared.Query(ctx, db.getProfileStmt, row).Get(&row)
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

func (db *Database) GetProfileByID(ctx context.Context, id int) (*Profile, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "SELECT", ProfilesTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			attribute.String("db.collection", ProfilesTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(ProfilesTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(ProfilesTableName, "select").Inc()

	row := Profile{ID: id}

	err := db.shared.Query(ctx, db.getProfileByIDStmt, row).Get(&row)
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

func (db *Database) CreateProfile(ctx context.Context, profile *Profile) error {
	_, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "INSERT", ProfilesTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("INSERT"),
			attribute.String("db.collection", ProfilesTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(ProfilesTableName, "insert"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(ProfilesTableName, "insert").Inc()

	_, err := db.propose(ellaraft.CmdCreateProfile, profile)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return err
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

func (db *Database) UpdateProfile(ctx context.Context, profile *Profile) error {
	_, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "UPDATE", ProfilesTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("UPDATE"),
			attribute.String("db.collection", ProfilesTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(ProfilesTableName, "update"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(ProfilesTableName, "update").Inc()

	_, err := db.propose(ellaraft.CmdUpdateProfile, profile)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return err
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

func (db *Database) DeleteProfile(ctx context.Context, name string) error {
	_, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "DELETE", ProfilesTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("DELETE"),
			attribute.String("db.collection", ProfilesTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(ProfilesTableName, "delete"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(ProfilesTableName, "delete").Inc()

	_, err := db.propose(ellaraft.CmdDeleteProfile, &stringPayload{Value: name})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return err
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

func (db *Database) CountProfiles(ctx context.Context) (int, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "SELECT", ProfilesTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			attribute.String("db.collection", ProfilesTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(ProfilesTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(ProfilesTableName, "select").Inc()

	var result NumItems

	err := db.shared.Query(ctx, db.countProfilesStmt).Get(&result)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return 0, fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return result.Count, nil
}

func (db *Database) CountSubscribersInProfile(ctx context.Context, profileID int) (int, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s (by profile)", "SELECT", SubscribersTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			attribute.String("db.collection", SubscribersTableName),
			attribute.Int("profile_id", profileID),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(SubscribersTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(SubscribersTableName, "select").Inc()

	var result NumItems

	subscriber := Subscriber{ProfileID: profileID}

	err := db.shared.Query(ctx, db.countSubscribersByProfileStmt, subscriber).Get(&result)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return 0, fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return result.Count, nil
}

func (db *Database) SubscribersInProfile(ctx context.Context, name string) (bool, error) {
	ctx, span := tracer.Start(
		ctx,
		"SubscribersInProfile",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
		),
	)
	defer span.End()

	profile, err := db.GetProfile(ctx, name)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			span.RecordError(ErrNotFound)
			span.SetStatus(codes.Error, "profile not found")

			return false, ErrNotFound
		}

		span.RecordError(err)
		span.SetStatus(codes.Error, "profile not found")

		return false, fmt.Errorf("profile not found: %w", err)
	}

	count, err := db.CountSubscribersInProfile(ctx, profile.ID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "counting failed")

		return false, fmt.Errorf("counting failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return count > 0, nil
}
