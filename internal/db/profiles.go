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

const ProfilesTableName = "profiles"

const (
	listProfilesPagedStmt = "SELECT &Profile.*, COUNT(*) OVER() AS &NumItems.count FROM %s LIMIT $ListArgs.limit OFFSET $ListArgs.offset"
	getProfileByNameStmt  = "SELECT &Profile.* FROM %s WHERE name==$Profile.name"
	getProfileByIDStmt    = "SELECT &Profile.* FROM %s WHERE id==$Profile.id"
	createProfileStmt     = "INSERT INTO %s (name) VALUES ($Profile.name)"
	deleteProfileStmt     = "DELETE FROM %s WHERE name==$Profile.name"
	countProfilesStmt     = "SELECT COUNT(*) AS &NumItems.count FROM %s"
)

type Profile struct {
	ID   int    `db:"id"`
	Name string `db:"name"`
}

func (db *Database) ListProfilesPage(ctx context.Context, page int, perPage int) ([]Profile, int, error) {
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

	err := db.conn.Query(ctx, db.listProfilesStmt, args).GetAll(&profiles, &counts)
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

	err := db.conn.Query(ctx, db.getProfileByNameStmt, row).Get(&row)
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

	err := db.conn.Query(ctx, db.getProfileByIDStmt, row).Get(&row)
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
	ctx, span := tracer.Start(
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

	err := db.conn.Query(ctx, db.createProfileStmt, profile).Run()
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

func (db *Database) DeleteProfile(ctx context.Context, name string) error {
	ctx, span := tracer.Start(
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

	var outcome sqlair.Outcome

	err := db.conn.Query(ctx, db.deleteProfileStmt, Profile{Name: name}).Get(&outcome)
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

	err := db.conn.Query(ctx, db.countProfilesStmt).Get(&result)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return 0, fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return result.Count, nil
}
