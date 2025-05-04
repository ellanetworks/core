// Copyright 2024 Ella Networks

package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/canonical/sqlair"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
)

const ProfilesTableName = "profiles"

const QueryCreateProfilesTable = `
	CREATE TABLE IF NOT EXISTS %s (
 		id INTEGER PRIMARY KEY AUTOINCREMENT,

		name TEXT NOT NULL,

		ueIPPool TEXT NOT NULL,
		dns TEXT NOT NULL,
		mtu INTEGER NOT NULL,
		bitrateUplink TEXT NOT NULL,
		bitrateDownlink TEXT NOT NULL,
		var5qi INTEGER NOT NULL,
		priorityLevel INTEGER NOT NULL
)`

const (
	listProfilesStmt   = "SELECT &Profile.* from %s"
	getProfileStmt     = "SELECT &Profile.* from %s WHERE name==$Profile.name"
	getProfileByIDStmt = "SELECT &Profile.* FROM %s WHERE id==$Profile.id"
	createProfileStmt  = "INSERT INTO %s (name, ueIPPool, dns, mtu, bitrateUplink, bitrateDownlink, var5qi, priorityLevel) VALUES ($Profile.name, $Profile.ueIPPool, $Profile.dns, $Profile.mtu, $Profile.bitrateUplink, $Profile.bitrateDownlink, $Profile.var5qi, $Profile.priorityLevel)"
	editProfileStmt    = "UPDATE %s SET ueIPPool=$Profile.ueIPPool, dns=$Profile.dns, mtu=$Profile.mtu, bitrateUplink=$Profile.bitrateUplink, bitrateDownlink=$Profile.bitrateDownlink, var5qi=$Profile.var5qi, priorityLevel=$Profile.priorityLevel WHERE name==$Profile.name"
	deleteProfileStmt  = "DELETE FROM %s WHERE name==$Profile.name"
)

type Profile struct {
	ID              int    `db:"id"`
	Name            string `db:"name"`
	UeIPPool        string `db:"ueIPPool"`
	DNS             string `db:"dns"`
	Mtu             int32  `db:"mtu"`
	BitrateUplink   string `db:"bitrateUplink"`
	BitrateDownlink string `db:"bitrateDownlink"`
	Var5qi          int32  `db:"var5qi"`
	PriorityLevel   int32  `db:"priorityLevel"`
}

func (db *Database) ListProfiles(ctx context.Context) ([]Profile, error) {
	operation := "SELECT"
	target := ProfilesTableName
	spanName := fmt.Sprintf("%s %s", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	stmt := fmt.Sprintf(listProfilesStmt, db.profilesTable)
	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(stmt),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
	)

	q, err := sqlair.Prepare(stmt, Profile{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "prepare failed")
		return nil, err
	}

	var profiles []Profile
	if err := db.conn.Query(ctx, q).GetAll(&profiles); err != nil {
		if err == sql.ErrNoRows {
			span.SetStatus(codes.Ok, "no rows")
			return nil, nil
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")
		return nil, err
	}

	span.SetStatus(codes.Ok, "")
	return profiles, nil
}

func (db *Database) GetProfile(name string, ctx context.Context) (*Profile, error) {
	operation := "SELECT"
	target := ProfilesTableName
	spanName := fmt.Sprintf("%s %s", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	stmt := fmt.Sprintf(getProfileStmt, db.profilesTable)
	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(stmt),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
	)

	row := Profile{Name: name}
	q, err := sqlair.Prepare(stmt, Profile{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "prepare failed")
		return nil, err
	}

	if err := db.conn.Query(ctx, q, row).Get(&row); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")
		return nil, err
	}

	span.SetStatus(codes.Ok, "")
	return &row, nil
}

func (db *Database) GetProfileByID(id int, ctx context.Context) (*Profile, error) {
	operation := "SELECT"
	target := ProfilesTableName
	spanName := fmt.Sprintf("%s %s", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	stmt := fmt.Sprintf(getProfileByIDStmt, db.profilesTable)
	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(stmt),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
	)

	row := Profile{ID: id}
	q, err := sqlair.Prepare(stmt, Profile{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "prepare failed")
		return nil, err
	}

	if err := db.conn.Query(ctx, q, row).Get(&row); err != nil {
		if err == sql.ErrNoRows {
			span.RecordError(err)
			span.SetStatus(codes.Error, "not found")
			return nil, fmt.Errorf("profile with ID %d not found", id)
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")
		return nil, err
	}

	span.SetStatus(codes.Ok, "")
	return &row, nil
}

func (db *Database) CreateProfile(profile *Profile, ctx context.Context) error {
	operation := "INSERT"
	target := ProfilesTableName
	spanName := fmt.Sprintf("%s %s", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	stmt := fmt.Sprintf(createProfileStmt, db.profilesTable)
	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(stmt),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
	)

	// ensure unique name
	if _, err := db.GetProfile(profile.Name, ctx); err == nil {
		dup := fmt.Errorf("profile with name %s already exists", profile.Name)
		span.RecordError(dup)
		span.SetStatus(codes.Error, "duplicate key")
		return dup
	}

	q, err := sqlair.Prepare(stmt, Profile{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "prepare failed")
		return err
	}
	if err := db.conn.Query(ctx, q, profile).Run(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")
		return err
	}

	span.SetStatus(codes.Ok, "")
	return nil
}

func (db *Database) UpdateProfile(profile *Profile, ctx context.Context) error {
	operation := "UPDATE"
	target := ProfilesTableName
	spanName := fmt.Sprintf("%s %s", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	stmt := fmt.Sprintf(editProfileStmt, db.profilesTable)
	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(stmt),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
	)

	// ensure exists
	if _, err := db.GetProfile(profile.Name, ctx); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "not found")
		return err
	}

	q, err := sqlair.Prepare(stmt, Profile{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "prepare failed")
		return err
	}
	if err := db.conn.Query(ctx, q, profile).Run(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")
		return err
	}

	span.SetStatus(codes.Ok, "")
	return nil
}

func (db *Database) DeleteProfile(name string, ctx context.Context) error {
	operation := "DELETE"
	target := ProfilesTableName
	spanName := fmt.Sprintf("%s %s", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	stmt := fmt.Sprintf(deleteProfileStmt, db.profilesTable)
	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(stmt),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
	)

	// ensure exists
	if _, err := db.GetProfile(name, ctx); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "not found")
		return err
	}

	q, err := sqlair.Prepare(stmt, Profile{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "prepare failed")
		return err
	}
	if err := db.conn.Query(ctx, q, Profile{Name: name}).Run(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")
		return err
	}

	span.SetStatus(codes.Ok, "")
	return nil
}
