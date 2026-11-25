// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

const UserIdentitiesTableName = "user_identities"

const (
	getIdentitySQL        = "SELECT id, userID, issuer, subject, email, createdAt, lastLoginAt FROM " + UserIdentitiesTableName + " WHERE issuer = ? AND subject = ?"
	createIdentitySQL     = "INSERT INTO " + UserIdentitiesTableName + " (id, userID, issuer, subject, email, createdAt, lastLoginAt) VALUES (?, ?, ?, ?, ?, ?, ?)"
	touchIdentitySQL      = "UPDATE " + UserIdentitiesTableName + " SET email = ?, lastLoginAt = ? WHERE id = ?"
	deleteIdentitiesSQL   = "DELETE FROM " + UserIdentitiesTableName + " WHERE userID = ?"
	deleteLinkedSessions  = "DELETE FROM " + SessionsTableName + " WHERE identity_id IN (SELECT id FROM " + UserIdentitiesTableName + " WHERE userID = ?)"
	userLinkedToIssuerSQL = "SELECT EXISTS (SELECT 1 FROM " + UserIdentitiesTableName + " i JOIN " + OIDCSettingsTableName +
		" o ON o.issuer = i.issuer WHERE i.userID = ?)"
	userIDOfEmailSQL = "SELECT id FROM " + UsersTableName + " WHERE email==?"
)

type Identity struct {
	ID          string
	UserID      string
	Issuer      string
	Subject     string
	Email       string
	CreatedAt   int64
	LastLoginAt int64
}

func (db *Database) GetIdentity(ctx context.Context, issuer, subject string) (_ *Identity, err error) {
	ctx, finish := startDBQuery(ctx, UserIdentitiesTableName, "SELECT")
	defer func() { finish(err) }()

	var i Identity

	err = db.readQuerier(ctx).QueryRowContext(ctx, getIdentitySQL, issuer, subject).Scan(&i.ID, &i.UserID, &i.Issuer,
		&i.Subject, &i.Email, &i.CreatedAt, &i.LastLoginAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return &i, nil
}

func (db *Database) UnlinkUser(ctx context.Context, email string) (err error) {
	ctx, finish := startDBQuery(ctx, UserIdentitiesTableName, "DELETE")
	defer func() { finish(err) }()

	_, err = opUnlinkUser.Invoke(ctx, db, &stringPayload{Value: email})

	return err
}

func (db *Database) applyUnlinkUser(ctx context.Context, p *stringPayload) (any, error) {
	var userID string

	err := db.runner(ctx).QueryRowContext(ctx, userIDOfEmailSQL, p.Value).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	var linked bool
	if err := db.runner(ctx).QueryRowContext(ctx, userLinkedToIssuerSQL, userID).Scan(&linked); err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	if !linked {
		return nil, ErrNotFound
	}

	if _, err := db.runner(ctx).ExecContext(ctx, deleteLinkedSessions, userID); err != nil {
		return nil, fmt.Errorf("delete sessions: %w", err)
	}

	if _, err := db.runner(ctx).ExecContext(ctx, deleteIdentitiesSQL, userID); err != nil {
		return nil, fmt.Errorf("delete identities: %w", err)
	}

	return nil, nil
}
