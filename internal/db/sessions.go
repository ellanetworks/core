package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/canonical/sqlair"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
)

const SessionsTableName = "sessions"

const (
	createSessionStmt            = "INSERT INTO %s (user_id, token_hash, created_at, expires_at) VALUES ($Session.user_id, $Session.token_hash, $Session.created_at, $Session.expires_at)"
	getSessionByTokenHashStmt    = "SELECT &Session.* FROM %s WHERE token_hash==$Session.token_hash"
	deleteSessionByTokenHashStmt = "DELETE FROM %s WHERE token_hash==$Session.token_hash"       // #nosec: G101
	deleteExpiredSessionsStmt    = "DELETE FROM %s WHERE expires_at <= $SessionCutoff.now_unix" // #nosec: G101
	countSessionsByUserStmt      = "SELECT COUNT(*) AS &NumItems.count FROM %s WHERE user_id==$UserIDArgs.user_id"
	deleteOldestSessionsStmt     = "DELETE FROM %s WHERE id IN (SELECT id FROM %s WHERE user_id==$DeleteOldestArgs.user_id ORDER BY created_at ASC LIMIT $DeleteOldestArgs.limit)"
	deleteAllSessionsForUserStmt = "DELETE FROM %s WHERE user_id==$UserIDArgs.user_id"
	deleteAllSessionsStmt        = "DELETE FROM %s"
)

type Session struct {
	ID        int    `db:"id"`
	UserID    int64  `db:"user_id"`
	TokenHash []byte `db:"token_hash"`
	CreatedAt int64  `db:"created_at"` // store as Unix timestamp (seconds since epoch)
	ExpiresAt int64  `db:"expires_at"` // store as Unix timestamp (seconds since epoch)
}

type SessionCutoff struct {
	NowUnix int64 `db:"now_unix"`
}

type UserIDArgs struct {
	UserID int64 `db:"user_id"`
}

type DeleteOldestArgs struct {
	UserID int64 `db:"user_id"`
	Limit  int   `db:"limit"`
}

func (db *Database) CreateSession(ctx context.Context, session *Session) (int64, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "INSERT", SessionsTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("INSERT"),
			attribute.String("db.collection", SessionsTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(SessionsTableName, "insert"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(SessionsTableName, "insert").Inc()

	var outcome sqlair.Outcome

	err := db.conn.Query(ctx, db.createSessionStmt, session).Get(&outcome)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return 0, fmt.Errorf("query failed: %w", err)
	}

	id, err := outcome.Result().LastInsertId()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "retrieving insert ID failed")

		return 0, fmt.Errorf("retrieving insert ID failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return id, nil
}

func (db *Database) GetSessionByTokenHash(ctx context.Context, tokenHash []byte) (*Session, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "SELECT", SessionsTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			attribute.String("db.collection", SessionsTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(SessionsTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(SessionsTableName, "select").Inc()

	row := Session{TokenHash: tokenHash}

	err := db.conn.Query(ctx, db.getSessionByTokenHashStmt, row).Get(&row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			span.SetStatus(codes.Ok, "no rows")
			return nil, ErrNotFound
		}

		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return nil, fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return &row, nil
}

func (db *Database) DeleteSessionByTokenHash(ctx context.Context, tokenHash []byte) error {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "DELETE", SessionsTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("DELETE"),
			attribute.String("db.collection", SessionsTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(SessionsTableName, "delete"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(SessionsTableName, "delete").Inc()

	arg := Session{TokenHash: tokenHash}

	err := db.conn.Query(ctx, db.deleteSessionByTokenHashStmt, arg).Run()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

func (db *Database) DeleteExpiredSessions(ctx context.Context) (int, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "DELETE", SessionsTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("DELETE"),
			attribute.String("db.collection", SessionsTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(SessionsTableName, "delete"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(SessionsTableName, "delete").Inc()

	var outcome sqlair.Outcome

	cutoff := SessionCutoff{NowUnix: time.Now().Unix()}

	err := db.conn.Query(ctx, db.deleteExpiredSessionsStmt, cutoff).Get(&outcome)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return 0, fmt.Errorf("query failed: %w", err)
	}

	rowsAffected, err := outcome.Result().RowsAffected()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "retrieving rows affected failed")

		return 0, fmt.Errorf("retrieving rows affected failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return int(rowsAffected), nil
}

func (db *Database) CountSessionsByUser(ctx context.Context, userID int64) (int, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "COUNT", SessionsTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("COUNT"),
			attribute.String("db.collection", SessionsTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(SessionsTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(SessionsTableName, "select").Inc()

	args := UserIDArgs{UserID: userID}

	var result NumItems

	err := db.conn.Query(ctx, db.countSessionsByUserStmt, args).Get(&result)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return 0, fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return result.Count, nil
}

func (db *Database) DeleteOldestSessions(ctx context.Context, userID int64, limit int) error {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "DELETE", SessionsTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("DELETE"),
			attribute.String("db.collection", SessionsTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(SessionsTableName, "delete"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(SessionsTableName, "delete").Inc()

	args := DeleteOldestArgs{UserID: userID, Limit: limit}

	err := db.conn.Query(ctx, db.deleteOldestSessionsStmt, args).Run()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

func (db *Database) DeleteAllSessionsForUser(ctx context.Context, userID int64) error {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "DELETE", SessionsTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("DELETE"),
			attribute.String("db.collection", SessionsTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(SessionsTableName, "delete"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(SessionsTableName, "delete").Inc()

	args := UserIDArgs{UserID: userID}

	err := db.conn.Query(ctx, db.deleteAllSessionsForUserStmt, args).Run()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

func (db *Database) DeleteAllSessions(ctx context.Context) error {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "DELETE_ALL", SessionsTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("DELETE"),
			attribute.String("db.collection", SessionsTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(SessionsTableName, "delete"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(SessionsTableName, "delete").Inc()

	err := db.conn.Query(ctx, db.deleteAllSessionsStmt).Run()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return nil
}
