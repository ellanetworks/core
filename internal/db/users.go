// Copyright 2024 Ella Networks

package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/canonical/sqlair"
)

const UsersTableName = "users"

const QueryCreateUsersTable = `
	CREATE TABLE IF NOT EXISTS %s (
 		id INTEGER PRIMARY KEY AUTOINCREMENT,

		email TEXT NOT NULL,
		role INTEGER NOT NULL,
		hashedPassword TEXT NOT NULL
)`

const (
	listUsersStmt        = "SELECT &User.* from %s"
	getUserStmt          = "SELECT &User.* from %s WHERE email==$User.email"
	createUserStmt       = "INSERT INTO %s (email, role, hashedPassword) VALUES ($User.email, $User.role, $User.hashedPassword)"
	editUserStmt         = "UPDATE %s SET role=$User.role WHERE email==$User.email"
	editUserPasswordStmt = "UPDATE %s SET hashedPassword=$User.hashedPassword WHERE email==$User.email" // #nosec: G101
	deleteUserStmt       = "DELETE FROM %s WHERE email==$User.email"
	getNumUsersStmt      = "SELECT COUNT(*) AS &NumUsers.count FROM %s"
)

type Role int

const (
	AdminRole Role = iota
	ReadOnlyRole
	NetworkManagerRole
)

func (r Role) String() string {
	switch r {
	case AdminRole:
		return "admin"
	case ReadOnlyRole:
		return "readonly"
	case NetworkManagerRole:
		return "network-manager"
	default:
		return "unknown"
	}
}

type NumUsers struct {
	Count int `db:"count"`
}

type User struct {
	ID             int    `db:"id"`
	Email          string `db:"email"`
	Role           Role   `db:"role"`
	HashedPassword string `db:"hashedPassword"`
}

func (db *Database) ListUsers(ctx context.Context) ([]User, error) {
	stmt, err := sqlair.Prepare(fmt.Sprintf(listUsersStmt, db.usersTable), User{})
	if err != nil {
		return nil, err
	}
	var users []User
	err = db.conn.Query(ctx, stmt).GetAll(&users)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return users, nil
}

func (db *Database) GetUser(email string, ctx context.Context) (*User, error) {
	row := User{
		Email: email,
	}
	stmt, err := sqlair.Prepare(fmt.Sprintf(getUserStmt, db.usersTable), User{})
	if err != nil {
		return nil, err
	}
	err = db.conn.Query(ctx, stmt, row).Get(&row)
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (db *Database) CreateUser(user *User, ctx context.Context) error {
	_, err := db.GetUser(user.Email, ctx)
	if err == nil {
		return fmt.Errorf("user with email %s already exists", user.Email)
	}
	stmt, err := sqlair.Prepare(fmt.Sprintf(createUserStmt, db.usersTable), User{})
	if err != nil {
		return err
	}
	err = db.conn.Query(ctx, stmt, user).Run()
	return err
}

func (db *Database) UpdateUser(email string, role Role, ctx context.Context) error {
	user, err := db.GetUser(email, ctx)
	if err != nil {
		return err
	}
	stmt, err := sqlair.Prepare(fmt.Sprintf(editUserStmt, db.usersTable), User{})
	if err != nil {
		return err
	}
	user.Role = role
	err = db.conn.Query(ctx, stmt, user).Run()
	return err
}

func (db *Database) UpdateUserPassword(email string, hashedPassword string, ctx context.Context) error {
	user, err := db.GetUser(email, ctx)
	if err != nil {
		return err
	}
	stmt, err := sqlair.Prepare(fmt.Sprintf(editUserPasswordStmt, db.usersTable), User{})
	if err != nil {
		return err
	}
	user.HashedPassword = hashedPassword
	err = db.conn.Query(ctx, stmt, user).Run()
	return err
}

func (db *Database) DeleteUser(email string, ctx context.Context) error {
	_, err := db.GetUser(email, ctx)
	if err != nil {
		return err
	}
	stmt, err := sqlair.Prepare(fmt.Sprintf(deleteUserStmt, db.usersTable), User{})
	if err != nil {
		return err
	}
	row := User{
		Email: email,
	}
	err = db.conn.Query(ctx, stmt, row).Run()
	return err
}

func (db *Database) NumUsers(ctx context.Context) (int, error) {
	stmt, err := sqlair.Prepare(fmt.Sprintf(getNumUsersStmt, db.usersTable), NumUsers{})
	if err != nil {
		return 0, fmt.Errorf("failed to prepare statement: %v", err)
	}
	result := NumUsers{}
	err = db.conn.Query(ctx, stmt).Get(&result)
	if err != nil {
		return 0, fmt.Errorf("failed to get number of users: %v", err)
	}
	return result.Count, nil
}
