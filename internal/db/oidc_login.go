// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

const (
	createOIDCSessionSQL  = "INSERT INTO " + SessionsTableName + " (id, user_id, token_hash, created_at, expires_at, method, identity_id) VALUES (?, ?, ?, ?, ?, ?, ?)"
	trimUserSessionsSQL   = "DELETE FROM " + SessionsTableName + " WHERE user_id = ? AND id NOT IN (SELECT id FROM " + SessionsTableName + " WHERE user_id = ? ORDER BY created_at DESC, id DESC LIMIT ?)"
	setUserRoleFromGroups = "UPDATE " + UsersTableName + " SET roleID = ? WHERE id = ? AND NOT rolePinned"
	deleteUsedStatesSQL   = "DELETE FROM " + OIDCUsedStatesTableName + " WHERE expiresAt <= ?"
	insertUsedStateSQL    = "INSERT INTO " + OIDCUsedStatesTableName + " (stateHash, expiresAt) VALUES (?, ?)"
	identityOfSubjectSQL  = "SELECT id, userID FROM " + UserIdentitiesTableName + " WHERE issuer = ? AND subject = ?"
	createOIDCUserSQL     = "INSERT INTO " + UsersTableName + " (id, email, roleID, hashedPassword) VALUES (?, ?, ?, '')"
	usersByEmailNoCaseSQL = "SELECT " + userColumns + " FROM " + UsersTableName + " WHERE email = ? COLLATE NOCASE ORDER BY email = ? DESC, id LIMIT 2"
	getUserByIDCurrentSQL = "SELECT " + userColumns + " FROM " + UsersTableName + " WHERE id = ?"
)

type OIDCLogin struct {
	Issuer             string  `json:"issuer"`
	ClientID           string  `json:"clientID"`
	Subject            string  `json:"subject"`
	Email              string  `json:"email"`
	LinkEmail          string  `json:"linkEmail"`
	MapsGroups         bool    `json:"mapsGroups"`
	RoleID             RoleID  `json:"roleID"`
	MaxUsers           int     `json:"maxUsers"`
	MaxSessionsPerUser int     `json:"maxSessionsPerUser"`
	NewUserID          string  `json:"newUserID"`
	NewIdentityID      string  `json:"newIdentityID"`
	Session            Session `json:"session"`
}

type OIDCLoginResult struct {
	User         User   `json:"user"`
	IdentityID   string `json:"identityID"`
	SessionID    string `json:"sessionID"`
	Linked       bool   `json:"linked"`
	Created      bool   `json:"created"`
	PreviousRole RoleID `json:"previousRole"`
	RoleKept     bool   `json:"roleKept"`
}

func (r OIDCLoginResult) RoleChanged() bool {
	return !r.Created && r.User.RoleID != r.PreviousRole
}

type usedStatePayload struct {
	StateHash []byte `json:"stateHash"`
	ExpiresAt int64  `json:"expiresAt"`
	Now       int64  `json:"now"`
}

func (db *Database) LogInWithOIDC(ctx context.Context, l *OIDCLogin) (_ OIDCLoginResult, err error) {
	ctx, finish := startDBQueryNamed(ctx, "OIDC login", UserIdentitiesTableName, "UPSERT")
	defer func() { finish(err) }()

	return opLogInWithOIDC.Invoke(ctx, db, l)
}

func (db *Database) UseOIDCState(ctx context.Context, stateHash []byte, expiresAt, now int64) (err error) {
	ctx, finish := startDBQuery(ctx, OIDCUsedStatesTableName, "INSERT")
	defer func() { finish(err) }()

	_, err = opUseOIDCState.Invoke(ctx, db, &usedStatePayload{StateHash: stateHash, ExpiresAt: expiresAt, Now: now})

	return err
}

func (db *Database) applyUseOIDCState(ctx context.Context, p *usedStatePayload) (any, error) {
	if _, err := db.runner(ctx).ExecContext(ctx, deleteUsedStatesSQL, p.Now); err != nil {
		return nil, fmt.Errorf("delete used states: %w", err)
	}

	if _, err := db.runner(ctx).ExecContext(ctx, insertUsedStateSQL, p.StateHash, p.ExpiresAt); err != nil {
		if isPrimaryKeyError(err) || isUniqueNameError(err) {
			return nil, ErrStateUsed
		}

		return nil, fmt.Errorf("insert used state: %w", err)
	}

	return nil, nil
}

func (db *Database) applyLogInWithOIDC(ctx context.Context, l *OIDCLogin) (any, error) {
	if err := db.checkOIDCLogin(ctx, l.Issuer, l.ClientID); err != nil {
		return nil, err
	}

	res, err := db.findOIDCUser(ctx, l)
	if err != nil {
		return nil, err
	}

	res.PreviousRole = res.User.RoleID

	if !res.Created {
		if res.User, res.RoleKept, err = db.syncRoleFromGroups(ctx, res.User, l); err != nil {
			return nil, err
		}
	}

	if _, err := db.runner(ctx).ExecContext(ctx, touchIdentitySQL, l.Email, l.Session.CreatedAt, res.IdentityID); err != nil {
		return nil, fmt.Errorf("update identity: %w", err)
	}

	s := l.Session
	if _, err := db.runner(ctx).ExecContext(ctx, createOIDCSessionSQL, s.ID, res.User.ID, s.TokenHash, s.CreatedAt, s.ExpiresAt,
		SessionOIDC, res.IdentityID); err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	if _, err := db.runner(ctx).ExecContext(ctx, trimUserSessionsSQL, res.User.ID, res.User.ID, l.MaxSessionsPerUser); err != nil {
		return nil, fmt.Errorf("trim sessions: %w", err)
	}

	res.SessionID = s.ID
	res.User.HasIdentity = true
	res.User.HashedPassword = ""

	return res, nil
}

func (db *Database) checkOIDCLogin(ctx context.Context, issuer, clientID string) error {
	var (
		enabled                  bool
		current, currentClientID string
	)

	err := db.runner(ctx).QueryRowContext(ctx, getOIDCIssuerSQL).Scan(&enabled, &current, &currentClientID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrOIDCDisabled
	}

	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	switch {
	case !enabled:
		return ErrOIDCDisabled
	case current != issuer || currentClientID != clientID:
		return ErrOIDCChanged
	}

	return nil
}

func (db *Database) findOIDCUser(ctx context.Context, l *OIDCLogin) (OIDCLoginResult, error) {
	var (
		res    OIDCLoginResult
		userID string
	)

	err := db.runner(ctx).QueryRowContext(ctx, identityOfSubjectSQL, l.Issuer, l.Subject).Scan(&res.IdentityID, &userID)
	if err == nil {
		res.User, err = scanUser(db.runner(ctx).QueryRowContext(ctx, getUserByIDCurrentSQL, userID))
		if err != nil {
			return OIDCLoginResult{}, fmt.Errorf("query failed: %w", err)
		}

		return res, nil
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return OIDCLoginResult{}, fmt.Errorf("query failed: %w", err)
	}

	if l.LinkEmail == "" {
		return OIDCLoginResult{}, ErrNotLinked
	}

	res.IdentityID = l.NewIdentityID

	u, err := db.userOfEmail(ctx, l.LinkEmail)
	if err == nil {
		if err := db.createIdentity(ctx, res.IdentityID, u.ID, l); err != nil {
			return OIDCLoginResult{}, err
		}

		res.User, res.Linked = u, true

		return res, nil
	}

	if !errors.Is(err, ErrNotFound) {
		return OIDCLoginResult{}, err
	}

	switch {
	case !l.MapsGroups:
		return OIDCLoginResult{}, ErrNoUser
	case l.RoleID == 0:
		return OIDCLoginResult{}, ErrNoRole
	}

	n, err := queryCount(ctx, db.runner(ctx), countUsersSQL)
	if err != nil {
		return OIDCLoginResult{}, fmt.Errorf("query failed: %w", err)
	}

	if n >= l.MaxUsers {
		return OIDCLoginResult{}, ErrUserLimit
	}

	if _, err := db.runner(ctx).ExecContext(ctx, createOIDCUserSQL, l.NewUserID, l.LinkEmail, l.RoleID); err != nil {
		return OIDCLoginResult{}, fmt.Errorf("create user: %w", err)
	}

	if err := db.createIdentity(ctx, res.IdentityID, l.NewUserID, l); err != nil {
		return OIDCLoginResult{}, err
	}

	res.User = User{ID: l.NewUserID, Email: l.LinkEmail, RoleID: l.RoleID}
	res.Created = true

	return res, nil
}

func (db *Database) userOfEmail(ctx context.Context, email string) (User, error) {
	users, err := queryAll(ctx, db.runner(ctx), usersByEmailNoCaseSQL, scanUser, email, email)
	if err != nil {
		return User{}, fmt.Errorf("query failed: %w", err)
	}

	switch {
	case len(users) == 0:
		return User{}, ErrNotFound
	case users[0].Email == email || len(users) == 1:
		return users[0], nil
	default:
		return User{}, ErrAmbiguousEmail
	}
}

func (db *Database) createIdentity(ctx context.Context, id, userID string, l *OIDCLogin) error {
	_, err := db.runner(ctx).ExecContext(ctx, createIdentitySQL, id, userID, l.Issuer, l.Subject, l.Email, l.Session.CreatedAt,
		l.Session.CreatedAt)
	if isUniqueNameError(err) {
		return ErrLinkedElsewhere
	}

	if err != nil {
		return fmt.Errorf("create identity: %w", err)
	}

	return nil
}

func (db *Database) syncRoleFromGroups(ctx context.Context, u User, l *OIDCLogin) (User, bool, error) {
	switch {
	case !l.MapsGroups || u.RolePinned:
		return u, false, nil
	case l.RoleID == 0:
		return User{}, false, ErrNoRole
	case l.RoleID == u.RoleID:
		return u, false, nil
	}

	if l.RoleID != RoleAdmin {
		err := db.keepPasswordAdmin(ctx, u.ID)
		if errors.Is(err, ErrLastPasswordAdmin) {
			return u, true, nil
		}

		if err != nil {
			return User{}, false, err
		}
	}

	if _, err := db.runner(ctx).ExecContext(ctx, setUserRoleFromGroups, l.RoleID, u.ID); err != nil {
		return User{}, false, fmt.Errorf("update role: %w", err)
	}

	if _, err := db.runner(ctx).ExecContext(ctx, deleteSessionsOfUser, u.ID); err != nil {
		return User{}, false, fmt.Errorf("delete sessions: %w", err)
	}

	u.RoleID = l.RoleID

	return u, false, nil
}
