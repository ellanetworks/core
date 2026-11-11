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
	createSessionStmt            = "INSERT INTO " + SessionsTableName + " (id, user_id, token_hash, created_at, expires_at) VALUES (?, ?, ?, ?, ?)"
	getSessionByTokenHashStmt    = "SELECT id, user_id, token_hash, created_at, expires_at FROM " + SessionsTableName + " WHERE token_hash==?"
	deleteSessionByTokenHashStmt = "DELETE FROM " + SessionsTableName + " WHERE token_hash==?"   // #nosec: G101
	deleteExpiredSessionsStmt    = "DELETE FROM " + SessionsTableName + " WHERE expires_at <= ?" // #nosec: G101
	countSessionsByUserStmt      = "SELECT COUNT(*) FROM " + SessionsTableName + " WHERE user_id==?"
	countExpiredSessionsStmt     = "SELECT COUNT(*) FROM " + SessionsTableName + " WHERE expires_at <= ?" // #nosec: G101
	deleteOldestSessionsStmt     = "DELETE FROM " + SessionsTableName + " WHERE id IN (SELECT id FROM " + SessionsTableName + " WHERE user_id==? ORDER BY created_at ASC LIMIT ?)"
	deleteAllSessionsForUserStmt = "DELETE FROM " + SessionsTableName + " WHERE user_id==?"
	deleteAllSessionsStmt        = "DELETE FROM " + SessionsTableName
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

func (db *Database) CreateSession(ctx context.Context, session *Session) error {
	ctx, span, timer := startDBQuery(ctx, SessionsTableName, "INSERT", "insert")
	defer span.End()
	defer timer.ObserveDuration()

	if session.ID == "" {
		id, err := uuid.NewV7()
		if err != nil {
			recordSpanError(span, err)

			return fmt.Errorf("generate session id: %w", err)
		}

		session.ID = id.String()
	}

	_, err := opCreateSession.Invoke(ctx, db, session)
	if err != nil {
		recordSpanError(span, err)

		return err
	}

	return nil
}

func (db *Database) GetSessionByTokenHash(ctx context.Context, tokenHash []byte) (*Session, error) {
	ctx, span, timer := startDBQuery(ctx, SessionsTableName, "SELECT", "select")
	defer span.End()
	defer timer.ObserveDuration()

	row, err := scanSession(db.reader().QueryRowContext(ctx, getSessionByTokenHashStmt, tokenHash))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return &row, nil
}

func (db *Database) DeleteSessionByTokenHash(ctx context.Context, tokenHash []byte) error {
	ctx, span, timer := startDBQuery(ctx, SessionsTableName, "DELETE", "delete")
	defer span.End()
	defer timer.ObserveDuration()

	_, err := opDeleteSessionByTokenHash.Invoke(ctx, db, &bytesPayload{Value: tokenHash})
	if err != nil {
		recordSpanError(span, err)

		return err
	}

	return nil
}

func (db *Database) DeleteExpiredSessions(ctx context.Context) (int, error) {
	ctx, span, timer := startDBQuery(ctx, SessionsTableName, "DELETE", "delete")
	defer span.End()
	defer timer.ObserveDuration()

	nowUnix := time.Now().Unix()

	count, err := opDeleteExpiredSessions.Invoke(ctx, db, &int64Payload{Value: nowUnix})
	if err != nil {
		recordSpanError(span, err)

		return 0, err
	}

	return count, nil
}

func (db *Database) CountSessionsByUser(ctx context.Context, userID string) (int, error) {
	ctx, span, timer := startDBQuery(ctx, SessionsTableName, "COUNT", "select")
	defer span.End()
	defer timer.ObserveDuration()

	count, err := queryCount(ctx, db.reader(), countSessionsByUserStmt, userID)
	if err != nil {
		recordSpanError(span, err)

		return 0, fmt.Errorf("query failed: %w", err)
	}

	return count, nil
}

func (db *Database) CountExpiredSessions(ctx context.Context, nowUnix int64) (int, error) {
	ctx, span, timer := startDBQuery(ctx, SessionsTableName, "COUNT", "select")
	defer span.End()
	defer timer.ObserveDuration()

	count, err := queryCount(ctx, db.reader(), countExpiredSessionsStmt, nowUnix)
	if err != nil {
		recordSpanError(span, err)

		return 0, fmt.Errorf("query failed: %w", err)
	}

	return count, nil
}

func (db *Database) DeleteOldestSessions(ctx context.Context, userID string, limit int) error {
	ctx, span, timer := startDBQuery(ctx, SessionsTableName, "DELETE", "delete")
	defer span.End()
	defer timer.ObserveDuration()

	_, err := opDeleteOldestSessions.Invoke(ctx, db, &DeleteOldestArgs{UserID: userID, Limit: limit})
	if err != nil {
		recordSpanError(span, err)

		return err
	}

	return nil
}

func (db *Database) DeleteAllSessionsForUser(ctx context.Context, userID string) error {
	ctx, span, timer := startDBQuery(ctx, SessionsTableName, "DELETE", "delete")
	defer span.End()
	defer timer.ObserveDuration()

	_, err := opDeleteAllSessionsForUser.Invoke(ctx, db, &stringPayload{Value: userID})
	if err != nil {
		recordSpanError(span, err)

		return err
	}

	return nil
}

func (db *Database) DeleteAllSessions(ctx context.Context) error {
	ctx, span, timer := startDBQueryNamed(ctx, "DELETE "+SessionsTableName+" (all)", SessionsTableName, "DELETE", "delete")
	defer span.End()
	defer timer.ObserveDuration()

	_, err := opDeleteAllSessions.Invoke(ctx, db, &emptyPayload{})
	if err != nil {
		recordSpanError(span, err)

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
