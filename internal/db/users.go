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

const UsersTableName = "users"

const QueryCreateUsersTable = `
	CREATE TABLE IF NOT EXISTS %s (
 		id INTEGER PRIMARY KEY AUTOINCREMENT,

		email TEXT NOT NULL UNIQUE,
		roleID INTEGER NOT NULL,
		hashedPassword TEXT NOT NULL
)`

const (
	listUsersPageStmt    = "SELECT &User.* from %s ORDER BY id LIMIT $ListArgs.limit OFFSET $ListArgs.offset"
	getUserStmt          = "SELECT &User.* from %s WHERE email==$User.email"
	getUserByIDStmt      = "SELECT &User.* from %s WHERE id==$User.id"
	createUserStmt       = "INSERT INTO %s (email, roleID, hashedPassword) VALUES ($User.email, $User.roleID, $User.hashedPassword)"
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
	ID             int64  `db:"id"`
	Email          string `db:"email"`
	RoleID         RoleID `db:"roleID"`
	HashedPassword string `db:"hashedPassword"`
}

func (db *Database) ListUsersPage(ctx context.Context, page, perPage int) ([]User, int, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s (paged)", "SELECT", UsersTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("SELECT"),
			attribute.String("db.collection", UsersTableName),
			attribute.Int("page", page),
			attribute.Int("per_page", perPage),
		),
	)
	defer span.End()

	args := ListArgs{
		Limit:  perPage,
		Offset: (page - 1) * perPage,
	}

	count, err := db.CountUsers(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "count failed")

		return nil, 0, err
	}

	var users []User

	err = db.conn.Query(ctx, db.listUsersStmt, args).GetAll(&users)
	if err != nil {
		if err == sql.ErrNoRows {
			span.SetStatus(codes.Ok, "no rows")
			return nil, count, nil
		}

		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return nil, 0, err
	}

	span.SetStatus(codes.Ok, "")

	return users, count, nil
}

// GetUser fetches a single user by email with a span named "SELECT users".
func (db *Database) GetUser(ctx context.Context, email string) (*User, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "SELECT", UsersTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("SELECT"),
			attribute.String("db.collection", UsersTableName),
		),
	)
	defer span.End()

	row := User{Email: email}

	err := db.conn.Query(ctx, db.getUserStmt, row).Get(&row)
	if err != nil {
		if err == sql.ErrNoRows {
			span.SetStatus(codes.Ok, "no rows")
			return nil, ErrNotFound
		}

		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return nil, err
	}

	span.SetStatus(codes.Ok, "")

	return &row, nil
}

// GetUserByID fetches a single user by ID with a span named "SELECT users".
func (db *Database) GetUserByID(ctx context.Context, id int64) (*User, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "SELECT", UsersTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("SELECT"),
			attribute.String("db.collection", UsersTableName),
		),
	)
	defer span.End()

	row := User{ID: id}

	err := db.conn.Query(ctx, db.getUserByIDStmt, row).Get(&row)
	if err != nil {
		if err == sql.ErrNoRows {
			span.SetStatus(codes.Ok, "no rows")
			return nil, ErrNotFound
		}

		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return nil, err
	}

	span.SetStatus(codes.Ok, "")

	return &row, nil
}

func (db *Database) CreateUser(ctx context.Context, user *User) (int64, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "INSERT", UsersTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("INSERT"),
			attribute.String("db.collection", UsersTableName),
		),
	)
	defer span.End()

	var outcome sqlair.Outcome

	err := db.conn.Query(ctx, db.createUserStmt, user).Get(&outcome)
	if err != nil {
		if isUniqueNameError(err) {
			span.RecordError(ErrAlreadyExists)
			span.SetStatus(codes.Error, "unique constraint failed")

			return 0, ErrAlreadyExists
		}

		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")

		return 0, err
	}

	id, err := outcome.Result().LastInsertId()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "retrieving insert ID failed")

		return 0, err
	}

	span.SetStatus(codes.Ok, "")

	return id, nil
}

// UpdateUser updates a user's role with a span named "UPDATE users".
func (db *Database) UpdateUser(ctx context.Context, email string, roleID RoleID) error {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "UPDATE", UsersTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("UPDATE"),
			attribute.String("db.collection", UsersTableName),
		),
	)
	defer span.End()

	user := &User{
		Email:  email,
		RoleID: roleID,
	}

	var outcome sqlair.Outcome

	err := db.conn.Query(ctx, db.editUserStmt, user).Get(&outcome)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")

		return err
	}

	rowsAffected, err := outcome.Result().RowsAffected()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "retrieving rows affected failed")

		return err
	}

	if rowsAffected == 0 {
		span.RecordError(ErrNotFound)
		span.SetStatus(codes.Error, "not found")

		return ErrNotFound
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

// UpdateUserPassword sets a new password hash with a span named "UPDATE users".
func (db *Database) UpdateUserPassword(ctx context.Context, email string, hashedPassword string) error {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "UPDATE", UsersTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("UPDATE"),
			attribute.String("db.collection", UsersTableName),
		),
	)
	defer span.End()

	user := &User{
		Email:          email,
		HashedPassword: hashedPassword,
	}

	var outcome sqlair.Outcome

	err := db.conn.Query(ctx, db.editUserPasswordStmt, user).Get(&outcome)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")

		return err
	}

	rowsAffected, err := outcome.Result().RowsAffected()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "retrieving rows affected failed")

		return err
	}

	if rowsAffected == 0 {
		span.RecordError(ErrNotFound)
		span.SetStatus(codes.Error, "not found")

		return ErrNotFound
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

// DeleteUser removes a user by email with a span named "DELETE users".
func (db *Database) DeleteUser(ctx context.Context, email string) error {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "DELETE", UsersTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("DELETE"),
			attribute.String("db.collection", UsersTableName),
		),
	)
	defer span.End()

	var outcome sqlair.Outcome

	err := db.conn.Query(ctx, db.deleteUserStmt, User{Email: email}).Get(&outcome)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")

		return err
	}

	rowsAffected, err := outcome.Result().RowsAffected()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "retrieving rows affected failed")

		return err
	}

	if rowsAffected == 0 {
		span.RecordError(ErrNotFound)
		span.SetStatus(codes.Error, "not found")

		return ErrNotFound
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

// CountUsers returns user count with a span named "SELECT users".
func (db *Database) CountUsers(ctx context.Context) (int, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "SELECT", UsersTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("SELECT"),
			attribute.String("db.collection", UsersTableName),
		),
	)
	defer span.End()

	var result NumItems

	err := db.conn.Query(ctx, db.countUsersStmt).Get(&result)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")

		return 0, err
	}

	span.SetStatus(codes.Ok, "")

	return result.Count, nil
}
