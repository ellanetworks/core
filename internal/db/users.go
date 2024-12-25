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

		username TEXT NOT NULL,
		hashedPassword TEXT NOT NULL
)`

const (
	listUsersStmt  = "SELECT &User.* from %s"
	getUserStmt    = "SELECT &User.* from %s WHERE username==$User.username"
	createUserStmt = "INSERT INTO %s (username, hashedPassword) VALUES ($User.username, $User.hashedPassword)"
	editUserStmt   = "UPDATE %s SET hashedPassword=$User.hashedPassword WHERE username==$User.username"
	deleteUserStmt = "DELETE FROM %s WHERE username==$User.username"
)

type User struct {
	ID             int    `db:"id"`
	Username       string `db:"username"`
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

func (db *Database) GetUser(username string) (*User, error) {
	row := User{
		Username: username,
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
	_, err := db.GetUser(user.Username)
	if err == nil {
		return fmt.Errorf("user with username %s already exists", user.Username)
	}
	stmt, err := sqlair.Prepare(fmt.Sprintf(createUserStmt, db.usersTable), User{})
	if err != nil {
		return err
	}
	err = db.conn.Query(context.Background(), stmt, user).Run()
	return err
}

func (db *Database) UpdateUser(user *User) error {
	_, err := db.GetUser(user.Username)
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

func (db *Database) DeleteUser(username string) error {
	_, err := db.GetUser(username)
	if err != nil {
		return err
	}
	stmt, err := sqlair.Prepare(fmt.Sprintf(deleteUserStmt, db.usersTable), User{})
	if err != nil {
		return err
	}
	row := User{
		Username: username,
	}
	err = db.conn.Query(context.Background(), stmt, row).Run()
	return err
}
