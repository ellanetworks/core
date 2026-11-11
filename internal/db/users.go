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
	"go.opentelemetry.io/otel/attribute"
)

const UsersTableName = "users"

const (
	listUsersPageStmt    = "SELECT id, email, roleID, hashedPassword, COUNT(*) OVER() from " + UsersTableName + " ORDER BY id LIMIT ? OFFSET ?"
	getUserStmt          = "SELECT id, email, roleID, hashedPassword from " + UsersTableName + " WHERE email==?"
	getUserByIDStmt      = "SELECT id, email, roleID, hashedPassword from " + UsersTableName + " WHERE id==?"
	createUserStmt       = "INSERT INTO " + UsersTableName + " (id, email, roleID, hashedPassword) VALUES (?, ?, ?, ?)"
	editUserStmt         = "UPDATE " + UsersTableName + " SET roleID=? WHERE email==?"
	editUserPasswordStmt = "UPDATE " + UsersTableName + " SET hashedPassword=? WHERE email==?" // #nosec: G101
	deleteUserStmt       = "DELETE FROM " + UsersTableName + " WHERE email==?"
	countUsersStmt       = "SELECT COUNT(*) FROM " + UsersTableName
)

type RoleID int

const (
	RoleAdmin          RoleID = 1
	RoleReadOnly       RoleID = 2
	RoleNetworkManager RoleID = 3
)

type User struct {
	ID             string // UUIDv7
	Email          string
	RoleID         RoleID
	HashedPassword string
}

func (db *Database) ListUsersPage(ctx context.Context, page, perPage int) ([]User, int, error) {
	ctx, span, timer := startDBQueryNamed(ctx, "SELECT "+UsersTableName+" (paged)", UsersTableName, "SELECT", "select", attribute.Int("ella.db.page", page), attribute.Int("ella.db.page_size", perPage))
	defer span.End()
	defer timer.ObserveDuration()

	users, count, err := queryPage(ctx, db.reader(), listUsersPageStmt, scanUser, perPage, (page-1)*perPage)
	if err != nil {
		recordSpanError(span, err)

		return nil, 0, fmt.Errorf("query failed: %w", err)
	}

	if len(users) == 0 {
		fallbackCount, countErr := db.CountUsers(ctx)
		if countErr != nil {
			return nil, 0, nil
		}

		return nil, fallbackCount, nil
	}

	return users, count, nil
}

// GetUser fetches a single user by email with a span named "SELECT users".
func (db *Database) GetUser(ctx context.Context, email string) (*User, error) {
	ctx, span, timer := startDBQuery(ctx, UsersTableName, "SELECT", "select")
	defer span.End()
	defer timer.ObserveDuration()

	row, err := scanUser(db.reader().QueryRowContext(ctx, getUserStmt, email))
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
	ctx, span, timer := startDBQuery(ctx, UsersTableName, "SELECT", "select")
	defer span.End()
	defer timer.ObserveDuration()

	row, err := scanUser(db.reader().QueryRowContext(ctx, getUserByIDStmt, id))
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
	ctx, span, timer := startDBQuery(ctx, UsersTableName, "INSERT", "insert")
	defer span.End()
	defer timer.ObserveDuration()

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
	ctx, span, timer := startDBQuery(ctx, UsersTableName, "UPDATE", "update")
	defer span.End()
	defer timer.ObserveDuration()

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
	ctx, span, timer := startDBQuery(ctx, UsersTableName, "UPDATE", "update")
	defer span.End()
	defer timer.ObserveDuration()

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
	ctx, span, timer := startDBQuery(ctx, UsersTableName, "DELETE", "delete")
	defer span.End()
	defer timer.ObserveDuration()

	_, err := opDeleteUser.Invoke(ctx, db, &stringPayload{Value: email})
	if err != nil {
		recordSpanError(span, err)

		return err
	}

	return nil
}

// CountUsers returns user count with a span named "SELECT users".
func (db *Database) CountUsers(ctx context.Context) (int, error) {
	ctx, span, timer := startDBQuery(ctx, UsersTableName, "SELECT", "select")
	defer span.End()
	defer timer.ObserveDuration()

	count, err := queryCount(ctx, db.reader(), countUsersStmt)
	if err != nil {
		recordSpanError(span, err)

		return 0, err
	}

	return count, nil
}

func scanUser(row scanner) (User, error) {
	var v User

	if err := row.Scan(&v.ID, &v.Email, &v.RoleID, &v.HashedPassword); err != nil {
		return User{}, err
	}

	return v, nil
}
