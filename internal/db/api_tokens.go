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
	"go.opentelemetry.io/otel/attribute"
)

const APITokensTableName = "api_tokens"

type APIToken struct {
	ID        string // UUIDv7
	TokenID   string
	Name      string
	TokenHash string
	UserID    string // FK to users.id (UUID)
	ExpiresAt *time.Time
}

const (
	listAPITokensPagedStmt = `SELECT id, token_id, name, token_hash, user_id, expires_at, COUNT(*) OVER() FROM ` + APITokensTableName + ` WHERE user_id == ? ORDER BY id DESC LIMIT ? OFFSET ?`
	getByTokenIDStmt       = "SELECT id, token_id, name, token_hash, user_id, expires_at FROM " + APITokensTableName + " WHERE token_id==?"
	deleteAPITokenStmt     = "DELETE FROM " + APITokensTableName + " WHERE id==?"                                                                     // #nosec: G101
	createAPITokenStmt     = "INSERT INTO " + APITokensTableName + " (id, token_id, name, token_hash, user_id, expires_at) VALUES (?, ?, ?, ?, ?, ?)" // #nosec: G101
	countAPITokensStmt     = "SELECT COUNT(*) FROM " + APITokensTableName + " WHERE user_id==?"                                                       // #nosec: G101
)

func (db *Database) ListAPITokensPage(ctx context.Context, userID string, page int, perPage int) ([]APIToken, int, error) {
	ctx, span, timer := startDBQueryNamed(ctx, "SELECT "+APITokensTableName+" (paged)", APITokensTableName, "SELECT", "select", attribute.Int("ella.db.page", page), attribute.Int("ella.db.page_size", perPage))
	defer span.End()
	defer timer.ObserveDuration()

	tokens, count, err := queryPage(ctx, db.reader(), listAPITokensPagedStmt, scanAPIToken, userID, perPage, (page-1)*perPage)
	if err != nil {
		recordSpanError(span, err)

		return nil, 0, fmt.Errorf("query failed: %w", err)
	}

	if len(tokens) == 0 {
		fallbackCount, countErr := db.CountAPITokens(ctx, userID)
		if countErr != nil {
			return nil, 0, nil
		}

		return nil, fallbackCount, nil
	}

	return tokens, count, nil
}

// CreateAPIToken inserts a new api token with a span named "INSERT api_token".
func (db *Database) CreateAPIToken(ctx context.Context, apiToken *APIToken) error {
	ctx, span, timer := startDBQuery(ctx, APITokensTableName, "INSERT", "insert")
	defer span.End()
	defer timer.ObserveDuration()

	if apiToken.ID == "" {
		id, err := uuid.NewV7()
		if err != nil {
			recordSpanError(span, err)

			return fmt.Errorf("generate api token id: %w", err)
		}

		apiToken.ID = id.String()
	}

	_, err := opCreateAPIToken.Invoke(ctx, db, apiToken)
	if err != nil {
		recordSpanError(span, err)

		return err
	}

	return nil
}

func (db *Database) GetAPITokenByTokenID(ctx context.Context, tokenID string) (*APIToken, error) {
	ctx, span, timer := startDBQuery(ctx, APITokensTableName, "SELECT", "select")
	defer span.End()
	defer timer.ObserveDuration()

	row, err := scanAPIToken(db.reader().QueryRowContext(ctx, getByTokenIDStmt, tokenID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return &row, nil
}

func (db *Database) DeleteAPIToken(ctx context.Context, id string) error {
	ctx, span, timer := startDBQuery(ctx, APITokensTableName, "DELETE", "delete")
	defer span.End()
	defer timer.ObserveDuration()

	_, err := opDeleteAPIToken.Invoke(ctx, db, &stringPayload{Value: id})
	if err != nil {
		recordSpanError(span, err)

		return err
	}

	return nil
}

func (db *Database) CountAPITokens(ctx context.Context, userID string) (int, error) {
	ctx, span, timer := startDBQuery(ctx, APITokensTableName, "SELECT", "select")
	defer span.End()
	defer timer.ObserveDuration()

	count, err := queryCount(ctx, db.reader(), countAPITokensStmt, userID)
	if err != nil {
		recordSpanError(span, err)

		return 0, fmt.Errorf("query failed: %w", err)
	}

	return count, nil
}

func scanAPIToken(row scanner) (APIToken, error) {
	var v APIToken

	if err := row.Scan(&v.ID, &v.TokenID, &v.Name, &v.TokenHash, &v.UserID, &v.ExpiresAt); err != nil {
		return APIToken{}, err
	}

	return v, nil
}
