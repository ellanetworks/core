package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
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
  FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,

	UNIQUE (name, user_id)
);
` // #nosec: G101

type APIToken struct {
	ID        int        `db:"id"`
	TokenID   string     `db:"token_id"`
	Name      string     `db:"name"`
	TokenHash string     `db:"token_hash"`
	UserID    int64      `db:"user_id"`
	ExpiresAt *time.Time `db:"expires_at"`
}

const (
	listAPITokensPagedStmt = `SELECT &APIToken.*, COUNT(*) OVER() AS &NumItems.count FROM %s WHERE user_id == $APIToken.user_id ORDER BY id DESC LIMIT $ListArgs.limit OFFSET $ListArgs.offset`
	getByTokenIDStmt       = "SELECT &APIToken.* FROM %s WHERE token_id==$APIToken.token_id"
	getByNameStmt          = "SELECT &APIToken.* FROM %s WHERE user_id==$APIToken.user_id AND name==$APIToken.name"
	deleteAPITokenStmt     = "DELETE FROM %s WHERE id==$APIToken.id"                                                                                                                                       // #nosec: G101
	createAPITokenStmt     = "INSERT INTO %s (token_id, name, token_hash, user_id, expires_at) VALUES ($APIToken.token_id, $APIToken.name, $APIToken.token_hash, $APIToken.user_id, $APIToken.expires_at)" // #nosec: G101
	countAPITokensStmt     = "SELECT COUNT(*) AS &NumItems.count FROM %s WHERE user_id==$APIToken.user_id"                                                                                                 // #nosec: G101
)

func (db *Database) ListAPITokensPage(ctx context.Context, userID int64, page int, perPage int) ([]APIToken, int, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s (paged)", "SELECT", APITokensTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("SELECT"),
			attribute.String("db.collection", APITokensTableName),
			attribute.Int("page", page),
			attribute.Int("per_page", perPage),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(APITokensTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(APITokensTableName, "select").Inc()

	var tokens []APIToken

	var counts []NumItems

	args := ListArgs{
		Limit:  perPage,
		Offset: (page - 1) * perPage,
	}

	apiTokenArg := APIToken{UserID: userID}

	err := db.conn.Query(ctx, db.listAPITokensStmt, args, apiTokenArg).GetAll(&tokens, &counts)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			span.SetStatus(codes.Ok, "no rows")

			fallbackCount, countErr := db.CountAPITokens(ctx, userID)
			if countErr != nil {
				return nil, 0, nil
			}

			return nil, fallbackCount, nil
		}

		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return nil, 0, fmt.Errorf("query failed: %w", err)
	}

	count := 0
	if len(counts) > 0 {
		count = counts[0].Count
	}

	span.SetStatus(codes.Ok, "")

	return tokens, count, nil
}

// CreateAPIToken inserts a new api token with a span named "INSERT api_token".
func (db *Database) CreateAPIToken(ctx context.Context, apiToken *APIToken) error {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "INSERT", APITokensTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("INSERT"),
			attribute.String("db.collection", APITokensTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(APITokensTableName, "insert"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(APITokensTableName, "insert").Inc()

	err := db.conn.Query(ctx, db.createAPITokenStmt, apiToken).Run()
	if err != nil {
		if isUniqueNameError(err) {
			span.RecordError(ErrAlreadyExists)
			span.SetStatus(codes.Error, "unique constraint failed")

			return ErrAlreadyExists
		}

		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

func (db *Database) GetAPITokenByTokenID(ctx context.Context, tokenID string) (*APIToken, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "SELECT", APITokensTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("SELECT"),
			attribute.String("db.collection", APITokensTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(APITokensTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(APITokensTableName, "select").Inc()

	row := APIToken{TokenID: tokenID}

	err := db.conn.Query(ctx, db.getAPITokenByIDStmt, row).Get(&row)
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

func (db *Database) GetAPITokenByName(ctx context.Context, userID int64, name string) (*APIToken, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "SELECT", APITokensTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("SELECT"),
			attribute.String("db.collection", APITokensTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(APITokensTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(APITokensTableName, "select").Inc()

	row := APIToken{UserID: userID, Name: name}

	err := db.conn.Query(ctx, db.getAPITokenByNameStmt, row).Get(&row)
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

func (db *Database) DeleteAPIToken(ctx context.Context, id int) error {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "DELETE", APITokensTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("DELETE"),
			attribute.String("db.collection", APITokensTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(APITokensTableName, "delete"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(APITokensTableName, "delete").Inc()

	arg := APIToken{ID: id}

	err := db.conn.Query(ctx, db.deleteAPITokenStmt, arg).Run()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

func (db *Database) CountAPITokens(ctx context.Context, userID int64) (int, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "SELECT", APITokensTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("SELECT"),
			attribute.String("db.collection", APITokensTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(APITokensTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(APITokensTableName, "select").Inc()

	var result NumItems

	arg := APIToken{UserID: userID}

	err := db.conn.Query(ctx, db.countAPITokensStmt, arg).Get(&result)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return 0, fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return result.Count, nil
}
