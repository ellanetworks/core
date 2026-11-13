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
	listUsersPageSQL    = "SELECT id, email, roleID, hashedPassword, COUNT(*) OVER() from " + UsersTableName + " ORDER BY id LIMIT ? OFFSET ?"
	getUserSQL          = "SELECT id, email, roleID, hashedPassword from " + UsersTableName + " WHERE email==?"
	getUserByIDSQL      = "SELECT id, email, roleID, hashedPassword from " + UsersTableName + " WHERE id==?"
	createUserSQL       = "INSERT INTO " + UsersTableName + " (id, email, roleID, hashedPassword) VALUES (?, ?, ?, ?)"
	editUserSQL         = "UPDATE " + UsersTableName + " SET roleID=? WHERE email==?"
	editUserPasswordSQL = "UPDATE " + UsersTableName + " SET hashedPassword=? WHERE email==?" // #nosec: G101
	deleteUserSQL       = "DELETE FROM " + UsersTableName + " WHERE email==?"
	countUsersSQL       = "SELECT COUNT(*) FROM " + UsersTableName
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

func (db *Database) ListUsersPage(ctx context.Context, page, perPage int) (_ []User, _ int, err error) {
	ctx, finish := startDBQueryNamed(ctx, "SELECT "+UsersTableName+" (paged)", UsersTableName, "SELECT", attribute.Int("ella.db.page", page), attribute.Int("ella.db.page_size", perPage))
	defer func() { finish(err) }()

	users, count, err := queryPageWithTotal(ctx, db.reader(), listUsersPageSQL, scanUser, perPage, (page-1)*perPage)
	if err != nil {
		return nil, 0, fmt.Errorf("query failed: %w", err)
	}

	if len(users) == 0 {
		fallbackCount, err := db.CountUsers(ctx)
		if err != nil {
			return nil, 0, fmt.Errorf("count failed: %w", err)
		}

		return nil, fallbackCount, nil
	}

	return users, count, nil
}

// GetUser fetches a single user by email with a span named "SELECT users".
func (db *Database) GetUser(ctx context.Context, email string) (_ *User, err error) {
	ctx, finish := startDBQuery(ctx, UsersTableName, "SELECT")
	defer func() { finish(err) }()

	row, err := scanUser(db.reader().QueryRowContext(ctx, getUserSQL, email))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		return nil, err
	}

	return &row, nil
}

// GetUserByID fetches a single user by ID with a span named "SELECT users".
func (db *Database) GetUserByID(ctx context.Context, id string) (_ *User, err error) {
	ctx, finish := startDBQuery(ctx, UsersTableName, "SELECT")
	defer func() { finish(err) }()

	row, err := scanUser(db.reader().QueryRowContext(ctx, getUserByIDSQL, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		return nil, err
	}

	return &row, nil
}

func (db *Database) CreateUser(ctx context.Context, user *User) (_ string, err error) {
	ctx, finish := startDBQuery(ctx, UsersTableName, "INSERT")
	defer func() { finish(err) }()

	if user.ID == "" {
		id, err := uuid.NewV7()
		if err != nil {
			return "", fmt.Errorf("generate user id: %w", err)
		}

		user.ID = id.String()
	}

	if _, err := opCreateUser.Invoke(ctx, db, user); err != nil {
		return "", err
	}

	return user.ID, nil
}

// UpdateUser updates a user's role with a span named "UPDATE users".
func (db *Database) UpdateUser(ctx context.Context, email string, roleID RoleID) (err error) {
	ctx, finish := startDBQuery(ctx, UsersTableName, "UPDATE")
	defer func() { finish(err) }()

	user := &User{
		Email:  email,
		RoleID: roleID,
	}

	_, err = opUpdateUser.Invoke(ctx, db, user)
	if err != nil {
		return err
	}

	return nil
}

// UpdateUserPassword sets a new password hash with a span named "UPDATE users".
func (db *Database) UpdateUserPassword(ctx context.Context, email string, hashedPassword string) (err error) {
	ctx, finish := startDBQuery(ctx, UsersTableName, "UPDATE")
	defer func() { finish(err) }()

	user := &User{
		Email:          email,
		HashedPassword: hashedPassword,
	}

	_, err = opUpdateUserPassword.Invoke(ctx, db, user)
	if err != nil {
		return err
	}

	return nil
}

// DeleteUser removes a user by email with a span named "DELETE users".
func (db *Database) DeleteUser(ctx context.Context, email string) (err error) {
	ctx, finish := startDBQuery(ctx, UsersTableName, "DELETE")
	defer func() { finish(err) }()

	_, err = opDeleteUser.Invoke(ctx, db, &stringPayload{Value: email})
	if err != nil {
		return err
	}

	return nil
}

// CountUsers returns user count with a span named "SELECT users".
func (db *Database) CountUsers(ctx context.Context) (_ int, err error) {
	ctx, finish := startDBQuery(ctx, UsersTableName, "SELECT")
	defer func() { finish(err) }()

	count, err := queryCount(ctx, db.reader(), countUsersSQL)
	if err != nil {
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
