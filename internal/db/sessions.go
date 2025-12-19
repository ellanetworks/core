package db

import (
	"context"
	"fmt"

	"github.com/canonical/sqlair"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
)

const SessionsTableName = "sessions"

const createSessionsTableSQL = `
  CREATE TABLE IF NOT EXISTS sessions (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id     INTEGER NOT NULL,
  token_hash  BLOB    NOT NULL UNIQUE,
  created_at  INTEGER NOT NULL DEFAULT (strftime('%s','now')),
  expires_at  INTEGER NOT NULL,
  FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
)`

const (
	createSessionStmt            = "INSERT INTO %s (user_id, token_hash, expires_at) VALUES ($Session.user_id, $Session.token_hash, $Session.expires_at)"
	getSessionByTokenHashStmt    = "SELECT &Session.* FROM %s WHERE token_hash==$Session.token_hash"
	deleteSessionByTokenHashStmt = "DELETE FROM %s WHERE token_hash==$Session.token_hash"       // #nosec: G101
	deleteExpiredSessionsStmt    = "DELETE FROM %s WHERE expires_at <= (strftime('%%s','now'))" // #nosec: G101
)

type Session struct {
	ID        int    `db:"id"`
	UserID    int64  `db:"user_id"`
	TokenHash []byte `db:"token_hash"`
	CreatedAt int64  `db:"created_at"` // store as Unix timestamp (seconds since epoch)
	ExpiresAt int64  `db:"expires_at"` // store as Unix timestamp (seconds since epoch)
}

func (db *Database) CreateSession(ctx context.Context, session *Session) (int64, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "INSERT", SessionsTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("INSERT"),
			attribute.String("db.collection", SessionsTableName),
		),
	)
	defer span.End()

	var outcome sqlair.Outcome

	err := db.conn.Query(ctx, db.createSessionStmt, session).Get(&outcome)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")
		return 0, err
	}

	id, err := outcome.Result().LastInsertId()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "retrieving insert ID failed")
		return 0, err
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
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("SELECT"),
			attribute.String("db.collection", SessionsTableName),
		),
	)
	defer span.End()

	row := Session{TokenHash: tokenHash}

	err := db.conn.Query(ctx, db.getSessionByTokenHashStmt, row).Get(&row)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")
		return nil, err
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
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("DELETE"),
			attribute.String("db.collection", SessionsTableName),
		),
	)
	defer span.End()

	arg := Session{TokenHash: tokenHash}

	err := db.conn.Query(ctx, db.deleteSessionByTokenHashStmt, arg).Run()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")
		return err
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
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("DELETE"),
			attribute.String("db.collection", SessionsTableName),
		),
	)
	defer span.End()

	var outcome sqlair.Outcome

	err := db.conn.Query(ctx, db.deleteExpiredSessionsStmt).Get(&outcome)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")
		return 0, err
	}

	rowsAffected, err := outcome.Result().RowsAffected()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "retrieving rows affected failed")
		return 0, err
	}

	span.SetStatus(codes.Ok, "")

	return int(rowsAffected), nil
}
