// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

const SessionsTableName = "sessions"

const (
	createSessionSQL            = "INSERT INTO " + SessionsTableName + " (id, user_id, token_hash, created_at, expires_at) VALUES (?, ?, ?, ?, ?)"
	getSessionByTokenHashSQL    = "SELECT id, user_id, token_hash, created_at, expires_at FROM " + SessionsTableName + " WHERE token_hash==?"
	deleteSessionByTokenHashSQL = "DELETE FROM " + SessionsTableName + " WHERE token_hash==?"   // #nosec: G101
	deleteExpiredSessionsSQL    = "DELETE FROM " + SessionsTableName + " WHERE expires_at <= ?" // #nosec: G101
	countSessionsByUserSQL      = "SELECT COUNT(*) FROM " + SessionsTableName + " WHERE user_id==?"
	countExpiredSessionsSQL     = "SELECT COUNT(*) FROM " + SessionsTableName + " WHERE expires_at <= ?" // #nosec: G101
	deleteOldestSessionsSQL     = "DELETE FROM " + SessionsTableName + " WHERE id IN (SELECT id FROM " + SessionsTableName + " WHERE user_id==? ORDER BY created_at ASC LIMIT ?)"
	deleteAllSessionsForUserSQL = "DELETE FROM " + SessionsTableName + " WHERE user_id==?"
	deleteAllSessionsSQL        = "DELETE FROM " + SessionsTableName
)

type Session struct {
	ID        string // UUIDv7
	UserID    string // FK to users.id (UUID)
	TokenHash []byte
	CreatedAt int64 // store as Unix timestamp (seconds since epoch)
	ExpiresAt int64 // store as Unix timestamp (seconds since epoch)
}

type SessionCutoff struct {
	NowUnix int64
}

type UserIDArgs struct {
	UserID string
}

type DeleteOldestArgs struct {
	UserID string
	Limit  int
}

func (db *Database) CreateSession(ctx context.Context, session *Session) (err error) {
	ctx, finish := startDBQuery(ctx, SessionsTableName, "INSERT")
	defer func() { finish(err) }()

	if session.ID == "" {
		id, err := uuid.NewV7()
		if err != nil {
			return fmt.Errorf("generate session id: %w", err)
		}

		session.ID = id.String()
	}

	_, err = opCreateSession.Invoke(ctx, db, session)
	if err != nil {
		return err
	}

	return nil
}

func (db *Database) GetSessionByTokenHash(ctx context.Context, tokenHash []byte) (_ *Session, err error) {
	ctx, finish := startDBQuery(ctx, SessionsTableName, "SELECT")
	defer func() { finish(err) }()

	row, err := scanSession(db.reader().QueryRowContext(ctx, getSessionByTokenHashSQL, tokenHash))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return &row, nil
}

func (db *Database) DeleteSessionByTokenHash(ctx context.Context, tokenHash []byte) (err error) {
	ctx, finish := startDBQuery(ctx, SessionsTableName, "DELETE")
	defer func() { finish(err) }()

	_, err = opDeleteSessionByTokenHash.Invoke(ctx, db, &bytesPayload{Value: tokenHash})
	if err != nil {
		return err
	}

	return nil
}

func (db *Database) DeleteExpiredSessions(ctx context.Context) (_ int, err error) {
	ctx, finish := startDBQuery(ctx, SessionsTableName, "DELETE")
	defer func() { finish(err) }()

	nowUnix := time.Now().Unix()

	count, err := opDeleteExpiredSessions.Invoke(ctx, db, &int64Payload{Value: nowUnix})
	if err != nil {
		return 0, err
	}

	return count, nil
}

func (db *Database) CountSessionsByUser(ctx context.Context, userID string) (_ int, err error) {
	ctx, finish := startDBQuery(ctx, SessionsTableName, "SELECT")
	defer func() { finish(err) }()

	count, err := queryCount(ctx, db.reader(), countSessionsByUserSQL, userID)
	if err != nil {
		return 0, fmt.Errorf("query failed: %w", err)
	}

	return count, nil
}

func (db *Database) CountExpiredSessions(ctx context.Context, nowUnix int64) (_ int, err error) {
	ctx, finish := startDBQuery(ctx, SessionsTableName, "SELECT")
	defer func() { finish(err) }()

	count, err := queryCount(ctx, db.reader(), countExpiredSessionsSQL, nowUnix)
	if err != nil {
		return 0, fmt.Errorf("query failed: %w", err)
	}

	return count, nil
}

func (db *Database) DeleteOldestSessions(ctx context.Context, userID string, limit int) (err error) {
	ctx, finish := startDBQuery(ctx, SessionsTableName, "DELETE")
	defer func() { finish(err) }()

	_, err = opDeleteOldestSessions.Invoke(ctx, db, &DeleteOldestArgs{UserID: userID, Limit: limit})
	if err != nil {
		return err
	}

	return nil
}

func (db *Database) DeleteAllSessionsForUser(ctx context.Context, userID string) (err error) {
	ctx, finish := startDBQuery(ctx, SessionsTableName, "DELETE")
	defer func() { finish(err) }()

	_, err = opDeleteAllSessionsForUser.Invoke(ctx, db, &stringPayload{Value: userID})
	if err != nil {
		return err
	}

	return nil
}

func (db *Database) DeleteAllSessions(ctx context.Context) (err error) {
	ctx, finish := startDBQueryNamed(ctx, "DELETE "+SessionsTableName+" (all)", SessionsTableName, "DELETE")
	defer func() { finish(err) }()

	_, err = opDeleteAllSessions.Invoke(ctx, db, &emptyPayload{})
	if err != nil {
		return err
	}

	return nil
}

func scanSession(row scanner) (Session, error) {
	var v Session

	if err := row.Scan(&v.ID, &v.UserID, &v.TokenHash, &v.CreatedAt, &v.ExpiresAt); err != nil {
		return Session{}, err
	}

	return v, nil
}
