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
		hashedPassword TEXT NOT NULL
)`

const (
	listUsersStmt   = "SELECT &User.* from %s"
	getUserStmt     = "SELECT &User.* from %s WHERE email==$User.email"
	createUserStmt  = "INSERT INTO %s (email, hashedPassword) VALUES ($User.email, $User.hashedPassword)"
	editUserStmt    = "UPDATE %s SET hashedPassword=$User.hashedPassword WHERE email==$User.email"
	deleteUserStmt  = "DELETE FROM %s WHERE email==$User.email"
	getNumUsersStmt = "SELECT COUNT(*) AS &NumUsers.count FROM %s"
)

type NumUsers struct {
	Count int `db:"count"`
}

type User struct {
	ID             int    `db:"id"`
	Email          string `db:"email"`
	HashedPassword string `db:"hashedPassword"`
}

func (db *Database) ListUsers() ([]User, error) {
	stmt, err := sqlair.Prepare(fmt.Sprintf(listUsersStmt, db.usersTable), User{})
	if err != nil {
		return nil, err
	}
	var users []User
	err = db.conn.Query(context.Background(), stmt).GetAll(&users)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return users, nil
}

func (db *Database) GetUser(email string) (*User, error) {
	row := User{
		Email: email,
	}
	stmt, err := sqlair.Prepare(fmt.Sprintf(getUserStmt, db.usersTable), User{})
	if err != nil {
		return nil, err
	}
	err = db.conn.Query(context.Background(), stmt, row).Get(&row)
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (db *Database) CreateUser(user *User) error {
	_, err := db.GetUser(user.Email)
	if err == nil {
		return fmt.Errorf("user with email %s already exists", user.Email)
	}
	stmt, err := sqlair.Prepare(fmt.Sprintf(createUserStmt, db.usersTable), User{})
	if err != nil {
		return err
	}
	err = db.conn.Query(context.Background(), stmt, user).Run()
	return err
}

func (db *Database) UpdateUser(user *User) error {
	_, err := db.GetUser(user.Email)
	if err != nil {
		return err
	}
	stmt, err := sqlair.Prepare(fmt.Sprintf(editUserStmt, db.usersTable), User{})
	if err != nil {
		return err
	}
	err = db.conn.Query(context.Background(), stmt, user).Run()
	return err
}

func (db *Database) DeleteUser(email string) error {
	_, err := db.GetUser(email)
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
	err = db.conn.Query(context.Background(), stmt, row).Run()
	return err
}

func (db *Database) NumUsers() (int, error) {
	stmt, err := sqlair.Prepare(fmt.Sprintf(getNumUsersStmt, db.usersTable), NumUsers{})
	if err != nil {
		return 0, fmt.Errorf("failed to prepare statement: %v", err)
	}
	result := NumUsers{}
	err = db.conn.Query(context.Background(), stmt).Get(&result)
	if err != nil {
		return 0, fmt.Errorf("failed to get number of users: %v", err)
	}
	return result.Count, nil
}
