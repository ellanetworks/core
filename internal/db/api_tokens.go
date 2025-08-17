package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/canonical/sqlair"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
)

const APITokensTableName = "api_tokens"

const QueryCreateAPITokensTable = `
	CREATE TABLE IF NOT EXISTS %s (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
	token_id    TEXT NOT NULL UNIQUE,
  name        TEXT NOT NULL,
  token_hash  TEXT NOT NULL,
  user_id     INTEGER NOT NULL,
  expires_at  DATETIME,
  FOREIGN KEY (user_id) REFERENCES users(id)
);
` // #nosec: G101

type NumAPITokens struct {
	Count int `db:"count"`
}

type APIToken struct {
	ID        int        `db:"id"`
	TokenID   string     `db:"token_id"`
	Name      string     `db:"name"`
	TokenHash string     `db:"token_hash"`
	UserID    int        `db:"user_id"`
	ExpiresAt *time.Time `db:"expires_at"`
}

const (
	listAPITokensStmt  = `SELECT &APIToken.* FROM %s WHERE user_id == $APIToken.user_id`
	getByTokenIDStmt   = "SELECT &APIToken.* FROM %s WHERE token_id==$APIToken.token_id"
	getByNameStmt      = "SELECT &APIToken.* FROM %s WHERE user_id==$APIToken.user_id AND name==$APIToken.name"
	deleteAPITokenStmt = "DELETE FROM %s WHERE id==$APIToken.id"                                                                                                                                       // #nosec: G101
	createAPITokenStmt = "INSERT INTO %s (token_id, name, token_hash, user_id, expires_at) VALUES ($APIToken.token_id, $APIToken.name, $APIToken.token_hash, $APIToken.user_id, $APIToken.expires_at)" // #nosec: G101
	numAPITokensStmt   = "SELECT COUNT(*) AS &NumAPITokens.count FROM %s WHERE user_id==$APIToken.user_id"
)

func (db *Database) ListAPITokens(ctx context.Context, userID int) ([]APIToken, error) {
	operation := "SELECT"
	target := APITokensTableName
	spanName := fmt.Sprintf("%s %s", operation, target)
	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	stmt := fmt.Sprintf(listAPITokensStmt, db.apiTokensTable)

	q, err := sqlair.Prepare(stmt, APIToken{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "prepare failed")
		return nil, err
	}

	var tokens []APIToken
	arg := APIToken{UserID: userID}
	if err := db.conn.Query(ctx, q, arg).GetAll(&tokens); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			span.SetStatus(codes.Ok, "no rows")
			return []APIToken{}, nil
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")
		return nil, err
	}

	span.SetStatus(codes.Ok, "")
	return tokens, nil
}

// CreateAPIToken inserts a new api token with a span named "INSERT api_token".
func (db *Database) CreateAPIToken(ctx context.Context, apiToken *APIToken) error {
	operation := "INSERT"
	target := APITokensTableName
	spanName := fmt.Sprintf("%s %s", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	stmt := fmt.Sprintf(createAPITokenStmt, db.apiTokensTable)
	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(stmt),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
	)

	q, err := sqlair.Prepare(stmt, APIToken{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "prepare failed")
		return err
	}
	if err := db.conn.Query(ctx, q, apiToken).Run(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")
		return err
	}

	span.SetStatus(codes.Ok, "")
	return nil
}

func (db *Database) GetAPITokenByTokenID(ctx context.Context, tokenID string) (*APIToken, error) {
	stmt := fmt.Sprintf(getByTokenIDStmt, db.apiTokensTable)
	q, err := sqlair.Prepare(stmt, APIToken{})
	if err != nil {
		return nil, err
	}

	row := APIToken{TokenID: tokenID}
	if err := db.conn.Query(ctx, q, row).Get(&row); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

func (db *Database) GetAPITokenByName(ctx context.Context, userID int, name string) (*APIToken, error) {
	operation := "SELECT"
	target := APITokensTableName
	spanName := fmt.Sprintf("%s %s", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	stmt := fmt.Sprintf(getByNameStmt, db.apiTokensTable)
	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(stmt),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
	)

	row := APIToken{UserID: userID, Name: name}

	q, err := sqlair.Prepare(stmt, APIToken{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "prepare failed")
		return nil, err
	}

	if err := db.conn.Query(ctx, q, row).Get(&row); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")
		return nil, err
	}

	span.SetStatus(codes.Ok, "")
	return &row, nil
}

func (db *Database) DeleteAPIToken(ctx context.Context, id int) error {
	operation := "DELETE"
	target := APITokensTableName
	spanName := fmt.Sprintf("%s %s", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	stmt := fmt.Sprintf(deleteAPITokenStmt, db.apiTokensTable)
	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(stmt),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
	)

	q, err := sqlair.Prepare(stmt, APIToken{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "prepare failed")
		return err
	}
	arg := APIToken{ID: id}
	if err := db.conn.Query(ctx, q, arg).Run(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")
		return err
	}

	span.SetStatus(codes.Ok, "")
	return nil
}

func (db *Database) NumAPITokens(ctx context.Context, userID int) (int, error) {
	operation := "SELECT"
	target := APITokensTableName
	spanName := fmt.Sprintf("%s %s", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	stmt := fmt.Sprintf(numAPITokensStmt, db.apiTokensTable)
	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(stmt),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
	)

	q, err := sqlair.Prepare(stmt, APIToken{}, NumAPITokens{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "prepare failed")
		return 0, err
	}

	var result NumAPITokens

	arg := APIToken{UserID: userID}

	if err := db.conn.Query(ctx, q, arg).Get(&result); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")
		return 0, err
	}

	span.SetStatus(codes.Ok, "")
	return result.Count, nil
}
