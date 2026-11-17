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

const oidcSchema = 29

const (
	userColumnsPreV29 = "id, email, roleID, hashedPassword, FALSE, FALSE"
	userColumns       = "id, email, roleID, hashedPassword, rolePinned, EXISTS (SELECT 1 FROM " + UserIdentitiesTableName + " i JOIN " +
		OIDCSettingsTableName + " o ON o.issuer = i.issuer WHERE i.userID = " + UsersTableName + ".id)"
)

const (
	createUserSQL        = "INSERT INTO " + UsersTableName + " (id, email, roleID, hashedPassword) VALUES (?, ?, ?, ?)"
	createPinnedUserSQL  = "INSERT INTO " + UsersTableName + " (id, email, roleID, hashedPassword, rolePinned) VALUES (?, ?, ?, ?, ?)"
	editUserSQL          = "UPDATE " + UsersTableName + " SET roleID=? WHERE email==?"
	editUserRoleSQL      = "UPDATE " + UsersTableName + " SET roleID=?, rolePinned=? WHERE id==?"
	editUserPasswordSQL  = "UPDATE " + UsersTableName + " SET hashedPassword=? WHERE email==?" // #nosec: G101
	deleteUserSQL        = "DELETE FROM " + UsersTableName + " WHERE email==?"
	countUsersSQL        = "SELECT COUNT(*) FROM " + UsersTableName
	emailTakenSQL        = "SELECT EXISTS (SELECT 1 FROM " + UsersTableName + " WHERE email = ? COLLATE NOCASE)"
	userIDByEmailSQL     = "SELECT id, roleID, hashedPassword FROM " + UsersTableName + " WHERE email==?"
	keepPasswordAdminSQL = "SELECT NOT EXISTS (SELECT 1 FROM " + UsersTableName + " WHERE id = ? AND roleID = ? AND hashedPassword != '') OR EXISTS (SELECT 1 FROM " + UsersTableName + " WHERE id != ? AND roleID = ? AND hashedPassword != '')" // #nosec: G101
	deleteSessionsOfUser = "DELETE FROM " + SessionsTableName + " WHERE user_id = ?"
	listUsersPageSQLTmpl = "SELECT %s, COUNT(*) OVER() FROM " + UsersTableName + " ORDER BY id LIMIT ? OFFSET ?"
	getUserSQLTmpl       = "SELECT %s FROM " + UsersTableName + " WHERE email==?"
	getUserByIDSQLTmpl   = "SELECT %s FROM " + UsersTableName + " WHERE id==?"
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
	RolePinned     bool
	HasIdentity    bool
}

type userRolePayload struct {
	Email      string `json:"email"`
	RoleID     RoleID `json:"roleID"`
	RolePinned bool   `json:"rolePinned"`
}

func (db *Database) userColumns(ctx context.Context) string {
	if db.appliedSchemaAtLeast(ctx, oidcSchema) {
		return userColumns
	}

	return userColumnsPreV29
}

func (db *Database) ListUsersPage(ctx context.Context, page, perPage int) (_ []User, _ int, err error) {
	ctx, finish := startDBQueryNamed(ctx, "SELECT "+UsersTableName+" (paged)", UsersTableName, "SELECT", attribute.Int("ella.db.page", page), attribute.Int("ella.db.page_size", perPage))
	defer func() { finish(err) }()

	query := fmt.Sprintf(listUsersPageSQLTmpl, db.userColumns(ctx))

	users, count, err := queryPageWithTotal(ctx, db.reader(), query, scanUser, perPage, (page-1)*perPage)
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

	row, err := scanUser(db.reader().QueryRowContext(ctx, fmt.Sprintf(getUserSQLTmpl, db.userColumns(ctx)), email))
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

	row, err := scanUser(db.reader().QueryRowContext(ctx, fmt.Sprintf(getUserByIDSQLTmpl, db.userColumns(ctx)), id))
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

	if user.RolePinned && !db.appliedSchemaAtLeast(ctx, oidcSchema) {
		return "", ErrMigrationPending
	}

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
func (db *Database) UpdateUser(ctx context.Context, email string, roleID RoleID, rolePinned bool) (err error) {
	ctx, finish := startDBQuery(ctx, UsersTableName, "UPDATE")
	defer func() { finish(err) }()

	if !db.appliedSchemaAtLeast(ctx, oidcSchema) {
		if rolePinned {
			return ErrMigrationPending
		}

		_, err = opUpdateUser.Invoke(ctx, db, &User{Email: email, RoleID: roleID})

		return err
	}

	_, err = opUpdateUserRole.Invoke(ctx, db, &userRolePayload{Email: email, RoleID: roleID, RolePinned: rolePinned})

	return err
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

	if err := row.Scan(&v.ID, &v.Email, &v.RoleID, &v.HashedPassword, &v.RolePinned, &v.HasIdentity); err != nil {
		return User{}, err
	}

	return v, nil
}

func (db *Database) applyCreateUser(ctx context.Context, u *User) (any, error) {
	var taken bool
	if err := db.runner(ctx).QueryRowContext(ctx, emailTakenSQL, u.Email).Scan(&taken); err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	if taken {
		return nil, ErrAlreadyExists
	}

	query, args := createUserSQL, []any{u.ID, u.Email, u.RoleID, u.HashedPassword}
	if u.RolePinned {
		query, args = createPinnedUserSQL, append(args, u.RolePinned)
	}

	res, err := db.runner(ctx).ExecContext(ctx, query, args...)
	if err != nil {
		if isUniqueNameError(err) {
			return nil, ErrAlreadyExists
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("last insert id: %w", err)
	}

	return id, nil
}

func (db *Database) applyUpdateUser(ctx context.Context, u *User) (any, error) {
	return nil, db.setUserRole(ctx, u.Email, u.RoleID, nil)
}

func (db *Database) applyUpdateUserRole(ctx context.Context, p *userRolePayload) (any, error) {
	return nil, db.setUserRole(ctx, p.Email, p.RoleID, &p.RolePinned)
}

func (db *Database) setUserRole(ctx context.Context, email string, roleID RoleID, rolePinned *bool) error {
	id, current, _, err := db.userOfEmailForUpdate(ctx, email)
	if err != nil {
		return err
	}

	if roleID != RoleAdmin {
		if err := db.keepPasswordAdmin(ctx, id); err != nil {
			return err
		}
	}

	if rolePinned == nil {
		_, err = db.runner(ctx).ExecContext(ctx, editUserSQL, roleID, email)
	} else {
		_, err = db.runner(ctx).ExecContext(ctx, editUserRoleSQL, roleID, *rolePinned, id)
	}

	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	if roleID == current {
		return nil
	}

	if _, err := db.runner(ctx).ExecContext(ctx, deleteSessionsOfUser, id); err != nil {
		return fmt.Errorf("delete sessions: %w", err)
	}

	return nil
}

func (db *Database) applyUpdateUserPassword(ctx context.Context, u *User) (any, error) {
	id, _, hash, err := db.userOfEmailForUpdate(ctx, u.Email)
	if err != nil {
		return nil, err
	}

	if u.HashedPassword == "" {
		if hash == "" {
			return nil, ErrNoPassword
		}

		if err := db.keepPasswordAdmin(ctx, id); err != nil {
			return nil, err
		}
	}

	if _, err := db.runner(ctx).ExecContext(ctx, editUserPasswordSQL, u.HashedPassword, u.Email); err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyDeleteUser(ctx context.Context, p *stringPayload) (any, error) {
	id, _, _, err := db.userOfEmailForUpdate(ctx, p.Value)
	if err != nil {
		return nil, err
	}

	if err := db.keepPasswordAdmin(ctx, id); err != nil {
		return nil, err
	}

	if _, err := db.runner(ctx).ExecContext(ctx, deleteUserSQL, p.Value); err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) userOfEmailForUpdate(ctx context.Context, email string) (string, RoleID, string, error) {
	var (
		id   string
		role RoleID
		hash string
	)

	err := db.runner(ctx).QueryRowContext(ctx, userIDByEmailSQL, email).Scan(&id, &role, &hash)
	if errors.Is(err, sql.ErrNoRows) {
		return "", 0, "", ErrNotFound
	}

	if err != nil {
		return "", 0, "", fmt.Errorf("query failed: %w", err)
	}

	return id, role, hash, nil
}

func (db *Database) keepPasswordAdmin(ctx context.Context, userID string) error {
	var ok bool
	if err := db.runner(ctx).QueryRowContext(ctx, keepPasswordAdminSQL, userID, RoleAdmin, userID, RoleAdmin).Scan(&ok); err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	if !ok {
		return ErrLastPasswordAdmin
	}

	return nil
}
