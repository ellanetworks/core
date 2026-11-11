// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/ellanetworks/core/internal/tracing/attrs"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const ProfilesTableName = "profiles"

const (
	profileColumns               = "id, name, ueAmbrUplink, ueAmbrDownlink, allow4G, allow5G"
	listProfilesPagedSQL         = "SELECT " + profileColumns + ", COUNT(*) OVER() FROM " + ProfilesTableName + " LIMIT ? OFFSET ?"
	getProfileSQL                = "SELECT " + profileColumns + " FROM " + ProfilesTableName + " WHERE name==?"
	getProfileByIDSQL            = "SELECT " + profileColumns + " FROM " + ProfilesTableName + " WHERE id==?"
	createProfileSQL             = "INSERT INTO " + ProfilesTableName + " (id, name, ueAmbrUplink, ueAmbrDownlink, allow4G, allow5G) VALUES (?, ?, ?, ?, ?, ?)"
	editProfileSQL               = "UPDATE " + ProfilesTableName + " SET ueAmbrUplink=?, ueAmbrDownlink=?, allow4G=?, allow5G=? WHERE name==?"
	deleteProfileSQL             = "DELETE FROM " + ProfilesTableName + " WHERE name==?"
	countProfilesSQL             = "SELECT COUNT(*) FROM " + ProfilesTableName
	countSubscribersInProfileSQL = "SELECT COUNT(*) FROM " + SubscribersTableName + " WHERE profileID=?"
)

type Profile struct {
	ID             string // UUIDv7
	Name           string
	UeAmbrUplink   string
	UeAmbrDownlink string
	Allow4G        bool
	Allow5G        bool
}

func scanProfile(row scanner) (Profile, error) {
	var p Profile

	if err := row.Scan(&p.ID, &p.Name, &p.UeAmbrUplink, &p.UeAmbrDownlink, &p.Allow4G, &p.Allow5G); err != nil {
		return Profile{}, err
	}

	return p, nil
}

func (db *Database) ListProfilesPage(ctx context.Context, page, perPage int) ([]Profile, int, error) {
	ctx, span, timer := startDBQueryNamed(ctx, "SELECT "+ProfilesTableName+" (paged)", ProfilesTableName, "SELECT", "select", attribute.Int("ella.db.page", page), attribute.Int("ella.db.page_size", perPage))
	defer span.End()
	defer timer.ObserveDuration()

	profiles, count, err := queryPageWithTotal(ctx, db.reader(), listProfilesPagedSQL, scanProfile, perPage, (page-1)*perPage)
	if err != nil {
		recordSpanError(span, err)

		return nil, 0, fmt.Errorf("query failed: %w", err)
	}

	if len(profiles) == 0 {
		fallbackCount, err := db.CountProfiles(ctx)
		if err != nil {
			recordSpanError(span, err)

			return nil, 0, fmt.Errorf("count failed: %w", err)
		}

		return nil, fallbackCount, nil
	}

	return profiles, count, nil
}

func (db *Database) GetProfile(ctx context.Context, name string) (*Profile, error) {
	ctx, span, timer := startDBQuery(ctx, ProfilesTableName, "SELECT", "select")
	defer span.End()
	defer timer.ObserveDuration()

	row, err := scanProfile(db.reader().QueryRowContext(ctx, getProfileSQL, name))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return &row, nil
}

func (db *Database) GetProfileByID(ctx context.Context, id string) (*Profile, error) {
	ctx, span, timer := startDBQuery(ctx, ProfilesTableName, "SELECT", "select")
	defer span.End()
	defer timer.ObserveDuration()

	row, err := scanProfile(db.reader().QueryRowContext(ctx, getProfileByIDSQL, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return &row, nil
}

func (db *Database) CreateProfile(ctx context.Context, profile *Profile) error {
	ctx, span, timer := startDBQuery(ctx, ProfilesTableName, "INSERT", "insert")
	defer span.End()
	defer timer.ObserveDuration()

	if profile.ID == "" {
		id, err := uuid.NewV7()
		if err != nil {
			recordSpanError(span, err)

			return fmt.Errorf("generate profile id: %w", err)
		}

		profile.ID = id.String()
	}

	_, err := opCreateProfile.Invoke(ctx, db, profile)
	if err != nil {
		recordSpanError(span, err)

		return err
	}

	return nil
}

func (db *Database) UpdateProfile(ctx context.Context, profile *Profile) error {
	ctx, span, timer := startDBQuery(ctx, ProfilesTableName, "UPDATE", "update")
	defer span.End()
	defer timer.ObserveDuration()

	_, err := opUpdateProfile.Invoke(ctx, db, profile)
	if err != nil {
		recordSpanError(span, err)

		return err
	}

	return nil
}

func (db *Database) DeleteProfile(ctx context.Context, name string) error {
	ctx, span, timer := startDBQuery(ctx, ProfilesTableName, "DELETE", "delete")
	defer span.End()
	defer timer.ObserveDuration()

	_, err := opDeleteProfile.Invoke(ctx, db, &stringPayload{Value: name})
	if err != nil {
		recordSpanError(span, err)

		return err
	}

	return nil
}

func (db *Database) CountProfiles(ctx context.Context) (int, error) {
	ctx, span, timer := startDBQuery(ctx, ProfilesTableName, "SELECT", "select")
	defer span.End()
	defer timer.ObserveDuration()

	count, err := queryCount(ctx, db.reader(), countProfilesSQL)
	if err != nil {
		recordSpanError(span, err)

		return 0, fmt.Errorf("query failed: %w", err)
	}

	return count, nil
}

func (db *Database) CountSubscribersInProfile(ctx context.Context, profileID string) (int, error) {
	ctx, span, timer := startDBQueryNamed(ctx, "SELECT "+SubscribersTableName+" (by profile)", SubscribersTableName, "SELECT", "select", attrs.ProfileID(profileID))
	defer span.End()
	defer timer.ObserveDuration()

	count, err := queryCount(ctx, db.reader(), countSubscribersInProfileSQL, profileID)
	if err != nil {
		recordSpanError(span, err)

		return 0, fmt.Errorf("query failed: %w", err)
	}

	return count, nil
}

func (db *Database) SubscribersInProfile(ctx context.Context, name string) (bool, error) {
	ctx, span := tracer.Start(ctx, "db/subscribers_in_profile",
		trace.WithSpanKind(trace.SpanKindInternal),
	)
	defer span.End()

	profile, err := db.GetProfile(ctx, name)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			recordSpanError(span, ErrNotFound)

			return false, ErrNotFound
		}

		recordSpanError(span, err)

		return false, fmt.Errorf("profile not found: %w", err)
	}

	count, err := db.CountSubscribersInProfile(ctx, profile.ID)
	if err != nil {
		recordSpanError(span, err)

		return false, fmt.Errorf("counting failed: %w", err)
	}

	return count > 0, nil
}
