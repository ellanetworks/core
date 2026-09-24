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
	"github.com/prometheus/client_golang/prometheus"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
)

const SessionsTableName = "sessions"

const (
	createSessionStmt            = "INSERT INTO %s (id, user_id, token_hash, created_at, expires_at) VALUES ($Session.id, $Session.user_id, $Session.token_hash, $Session.created_at, $Session.expires_at)"
	getSessionByTokenHashStmt    = "SELECT &Session.* FROM %s WHERE token_hash==$Session.token_hash"
	deleteSessionByTokenHashStmt = "DELETE FROM %s WHERE token_hash==$Session.token_hash"       // #nosec: G101
	deleteExpiredSessionsStmt    = "DELETE FROM %s WHERE expires_at <= $SessionCutoff.now_unix" // #nosec: G101
	countSessionsByUserStmt      = "SELECT COUNT(*) AS &NumItems.count FROM %s WHERE user_id==$UserIDArgs.user_id"
	countExpiredSessionsStmt     = "SELECT COUNT(*) AS &NumItems.count FROM %s WHERE expires_at <= $SessionCutoff.now_unix" // #nosec: G101
	deleteOldestSessionsStmt     = "DELETE FROM %s WHERE id IN (SELECT id FROM %s WHERE user_id==$DeleteOldestArgs.user_id ORDER BY created_at ASC LIMIT $DeleteOldestArgs.limit)"
	deleteAllSessionsForUserStmt = "DELETE FROM %s WHERE user_id==$UserIDArgs.user_id"
	deleteAllSessionsStmt        = "DELETE FROM %s"
)

type Session struct {
	ID        string `db:"id"`      // UUIDv7
	UserID    string `db:"user_id"` // FK to users.id (UUID)
	TokenHash []byte `db:"token_hash"`
	CreatedAt int64  `db:"created_at"` // store as Unix timestamp (seconds since epoch)
	ExpiresAt int64  `db:"expires_at"` // store as Unix timestamp (seconds since epoch)
}

type SessionCutoff struct {
	NowUnix int64 `db:"now_unix"`
}

type UserIDArgs struct {
	UserID string `db:"user_id"`
}

type DeleteOldestArgs struct {
	UserID string `db:"user_id"`
	Limit  int    `db:"limit"`
}

func (db *Database) CreateSession(ctx context.Context, session *Session) error {
	querySummary := fmt.Sprintf("%s %s", "INSERT", SessionsTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("INSERT"),
			semconv.DBCollectionName(SessionsTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(SessionsTableName, "insert"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(SessionsTableName, "insert").Inc()

	if session.ID == "" {
		id, err := uuid.NewV7()
		if err != nil {
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
	querySummary := fmt.Sprintf("%s %s", "SELECT", SessionsTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			semconv.DBCollectionName(SessionsTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(SessionsTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(SessionsTableName, "select").Inc()

	row := Session{TokenHash: tokenHash}

	err := db.conn().Query(ctx, db.getSessionByTokenHashStmt, row).Get(&row)
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
	querySummary := fmt.Sprintf("%s %s", "DELETE", SessionsTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("DELETE"),
			semconv.DBCollectionName(SessionsTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(SessionsTableName, "delete"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(SessionsTableName, "delete").Inc()

	_, err := opDeleteSessionByTokenHash.Invoke(ctx, db, &bytesPayload{Value: tokenHash})
	if err != nil {
		recordSpanError(span, err)

		return err
	}

	return nil
}

func (db *Database) DeleteExpiredSessions(ctx context.Context) (int, error) {
	querySummary := fmt.Sprintf("%s %s", "DELETE", SessionsTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("DELETE"),
			semconv.DBCollectionName(SessionsTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(SessionsTableName, "delete"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(SessionsTableName, "delete").Inc()

	nowUnix := time.Now().Unix()

	count, err := opDeleteExpiredSessions.Invoke(ctx, db, &int64Payload{Value: nowUnix})
	if err != nil {
		recordSpanError(span, err)

		return 0, err
	}

	return count, nil
}

func (db *Database) CountSessionsByUser(ctx context.Context, userID string) (int, error) {
	querySummary := fmt.Sprintf("%s %s", "COUNT", SessionsTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("COUNT"),
			semconv.DBCollectionName(SessionsTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(SessionsTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(SessionsTableName, "select").Inc()

	args := UserIDArgs{UserID: userID}

	var result NumItems

	err := db.conn().Query(ctx, db.countSessionsByUserStmt, args).Get(&result)
	if err != nil {
		recordSpanError(span, err)

		return 0, fmt.Errorf("query failed: %w", err)
	}

	return result.Count, nil
}

func (db *Database) CountExpiredSessions(ctx context.Context, nowUnix int64) (int, error) {
	querySummary := fmt.Sprintf("%s %s", "COUNT", SessionsTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("COUNT"),
			semconv.DBCollectionName(SessionsTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(SessionsTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(SessionsTableName, "select").Inc()

	var result NumItems

	err := db.conn().Query(ctx, db.countExpiredSessionsStmt, SessionCutoff{NowUnix: nowUnix}).Get(&result)
	if err != nil {
		recordSpanError(span, err)

		return 0, fmt.Errorf("query failed: %w", err)
	}

	return result.Count, nil
}

func (db *Database) DeleteOldestSessions(ctx context.Context, userID string, limit int) error {
	querySummary := fmt.Sprintf("%s %s", "DELETE", SessionsTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("DELETE"),
			semconv.DBCollectionName(SessionsTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(SessionsTableName, "delete"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(SessionsTableName, "delete").Inc()

	_, err := opDeleteOldestSessions.Invoke(ctx, db, &DeleteOldestArgs{UserID: userID, Limit: limit})
	if err != nil {
		recordSpanError(span, err)

		return err
	}

	return nil
}

func (db *Database) DeleteAllSessionsForUser(ctx context.Context, userID string) error {
	querySummary := fmt.Sprintf("%s %s", "DELETE", SessionsTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("DELETE"),
			semconv.DBCollectionName(SessionsTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(SessionsTableName, "delete"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(SessionsTableName, "delete").Inc()

	_, err := opDeleteAllSessionsForUser.Invoke(ctx, db, &stringPayload{Value: userID})
	if err != nil {
		recordSpanError(span, err)

		return err
	}

	return nil
}

func (db *Database) DeleteAllSessions(ctx context.Context) error {
	querySummary := fmt.Sprintf("%s %s (all)", "DELETE", SessionsTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("DELETE"),
			semconv.DBCollectionName(SessionsTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(SessionsTableName, "delete"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(SessionsTableName, "delete").Inc()

	_, err := opDeleteAllSessions.Invoke(ctx, db, &emptyPayload{})
	if err != nil {
		recordSpanError(span, err)

		return err
	}

	return nil
}
