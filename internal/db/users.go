// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// SPDX-FileCopyrightText: Ella Networks Inc.

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
)

const UsersTableName = "users"

const (
	listUsersPageStmt    = "SELECT &User.*, COUNT(*) OVER() AS &NumItems.count from %s ORDER BY id LIMIT $ListArgs.limit OFFSET $ListArgs.offset"
	getUserStmt          = "SELECT &User.* from %s WHERE email==$User.email"
	getUserByIDStmt      = "SELECT &User.* from %s WHERE id==$User.id"
	createUserStmt       = "INSERT INTO %s (id, email, roleID, hashedPassword) VALUES ($User.id, $User.email, $User.roleID, $User.hashedPassword)"
	editUserStmt         = "UPDATE %s SET roleID=$User.roleID WHERE email==$User.email"
	editUserPasswordStmt = "UPDATE %s SET hashedPassword=$User.hashedPassword WHERE email==$User.email" // #nosec: G101
	deleteUserStmt       = "DELETE FROM %s WHERE email==$User.email"
	countUsersStmt       = "SELECT COUNT(*) AS &NumItems.count FROM %s"
)

type RoleID int

const (
	RoleAdmin          RoleID = 1
	RoleReadOnly       RoleID = 2
	RoleNetworkManager RoleID = 3
)

type User struct {
	ID             string `db:"id"` // UUIDv7
	Email          string `db:"email"`
	RoleID         RoleID `db:"roleID"`
	HashedPassword string `db:"hashedPassword"`
}

func (db *Database) ListUsersPage(ctx context.Context, page, perPage int) ([]User, int, error) {
	querySummary := fmt.Sprintf("%s %s (paged)", "SELECT", UsersTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			semconv.DBCollectionName(UsersTableName),
			attribute.Int("ella.db.page", page),
			attribute.Int("ella.db.page_size", perPage),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(UsersTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(UsersTableName, "select").Inc()

	args := ListArgs{
		Limit:  perPage,
		Offset: (page - 1) * perPage,
	}

	var users []User

	var counts []NumItems

	err := db.conn().Query(ctx, db.listUsersStmt, args).GetAll(&users, &counts)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			fallbackCount, countErr := db.CountUsers(ctx)
			if countErr != nil {
				return nil, 0, nil
			}

			return nil, fallbackCount, nil
		}

		recordSpanError(span, err)

		return nil, 0, fmt.Errorf("query failed: %w", err)
	}

	count := 0
	if len(counts) > 0 {
		count = counts[0].Count
	}

	return users, count, nil
}

// GetUser fetches a single user by email with a span named "SELECT users".
func (db *Database) GetUser(ctx context.Context, email string) (*User, error) {
	querySummary := fmt.Sprintf("%s %s", "SELECT", UsersTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			semconv.DBCollectionName(UsersTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(UsersTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(UsersTableName, "select").Inc()

	row := User{Email: email}

	err := db.conn().Query(ctx, db.getUserStmt, row).Get(&row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		recordSpanError(span, err)

		return nil, err
	}

	return &row, nil
}

// GetUserByID fetches a single user by ID with a span named "SELECT users".
func (db *Database) GetUserByID(ctx context.Context, id string) (*User, error) {
	querySummary := fmt.Sprintf("%s %s", "SELECT", UsersTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			semconv.DBCollectionName(UsersTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(UsersTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(UsersTableName, "select").Inc()

	row := User{ID: id}

	err := db.conn().Query(ctx, db.getUserByIDStmt, row).Get(&row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		recordSpanError(span, err)

		return nil, err
	}

	return &row, nil
}

func (db *Database) CreateUser(ctx context.Context, user *User) (string, error) {
	querySummary := fmt.Sprintf("%s %s", "INSERT", UsersTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("INSERT"),
			semconv.DBCollectionName(UsersTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(UsersTableName, "insert"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(UsersTableName, "insert").Inc()

	if user.ID == "" {
		id, err := uuid.NewV7()
		if err != nil {
			recordSpanError(span, err)

			return "", fmt.Errorf("generate user id: %w", err)
		}

		user.ID = id.String()
	}

	if _, err := opCreateUser.Invoke(ctx, db, user); err != nil {
		recordSpanError(span, err)

		return "", err
	}

	return user.ID, nil
}

// UpdateUser updates a user's role with a span named "UPDATE users".
func (db *Database) UpdateUser(ctx context.Context, email string, roleID RoleID) error {
	querySummary := fmt.Sprintf("%s %s", "UPDATE", UsersTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("UPDATE"),
			semconv.DBCollectionName(UsersTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(UsersTableName, "update"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(UsersTableName, "update").Inc()

	user := &User{
		Email:  email,
		RoleID: roleID,
	}

	_, err := opUpdateUser.Invoke(ctx, db, user)
	if err != nil {
		recordSpanError(span, err)

		return err
	}

	return nil
}

// UpdateUserPassword sets a new password hash with a span named "UPDATE users".
func (db *Database) UpdateUserPassword(ctx context.Context, email string, hashedPassword string) error {
	querySummary := fmt.Sprintf("%s %s", "UPDATE", UsersTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("UPDATE"),
			semconv.DBCollectionName(UsersTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(UsersTableName, "update"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(UsersTableName, "update").Inc()

	user := &User{
		Email:          email,
		HashedPassword: hashedPassword,
	}

	_, err := opUpdateUserPassword.Invoke(ctx, db, user)
	if err != nil {
		recordSpanError(span, err)

		return err
	}

	return nil
}

// DeleteUser removes a user by email with a span named "DELETE users".
func (db *Database) DeleteUser(ctx context.Context, email string) error {
	querySummary := fmt.Sprintf("%s %s", "DELETE", UsersTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("DELETE"),
			semconv.DBCollectionName(UsersTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(UsersTableName, "delete"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(UsersTableName, "delete").Inc()

	_, err := opDeleteUser.Invoke(ctx, db, &stringPayload{Value: email})
	if err != nil {
		recordSpanError(span, err)

		return err
	}

	return nil
}

// CountUsers returns user count with a span named "SELECT users".
func (db *Database) CountUsers(ctx context.Context) (int, error) {
	querySummary := fmt.Sprintf("%s %s", "SELECT", UsersTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			semconv.DBCollectionName(UsersTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(UsersTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(UsersTableName, "select").Inc()

	var result NumItems

	err := db.conn().Query(ctx, db.countUsersStmt).Get(&result)
	if err != nil {
		recordSpanError(span, err)

		return 0, err
	}

	return result.Count, nil
}
