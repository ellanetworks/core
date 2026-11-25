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
	listAPITokensPagedSQL = `SELECT id, token_id, name, token_hash, user_id, expires_at, COUNT(*) OVER() FROM ` + APITokensTableName + ` WHERE user_id == ? ORDER BY id DESC LIMIT ? OFFSET ?`
	getByTokenIDSQL       = "SELECT id, token_id, name, token_hash, user_id, expires_at FROM " + APITokensTableName + " WHERE token_id==?"
	deleteAPITokenSQL     = "DELETE FROM " + APITokensTableName + " WHERE id==?"                                                                     // #nosec: G101
	createAPITokenSQL     = "INSERT INTO " + APITokensTableName + " (id, token_id, name, token_hash, user_id, expires_at) VALUES (?, ?, ?, ?, ?, ?)" // #nosec: G101
	countAPITokensSQL     = "SELECT COUNT(*) FROM " + APITokensTableName + " WHERE user_id==?"                                                       // #nosec: G101
)

func (db *Database) ListAPITokensPage(ctx context.Context, userID string, page int, perPage int) (_ []APIToken, _ int, err error) {
	ctx, finish := startDBQueryNamed(ctx, "SELECT "+APITokensTableName+" (paged)", APITokensTableName, "SELECT", attribute.Int("ella.db.page", page), attribute.Int("ella.db.page_size", perPage))
	defer func() { finish(err) }()

	tokens, count, err := queryPageWithTotal(ctx, db.reader(), listAPITokensPagedSQL, scanAPIToken, userID, perPage, (page-1)*perPage)
	if err != nil {
		return nil, 0, fmt.Errorf("query failed: %w", err)
	}

	if len(tokens) == 0 {
		fallbackCount, err := db.CountAPITokens(ctx, userID)
		if err != nil {
			return nil, 0, fmt.Errorf("count failed: %w", err)
		}

		return nil, fallbackCount, nil
	}

	return tokens, count, nil
}

// CreateAPIToken inserts a new api token with a span named "INSERT api_token".
func (db *Database) CreateAPIToken(ctx context.Context, apiToken *APIToken) (err error) {
	ctx, finish := startDBQuery(ctx, APITokensTableName, "INSERT")
	defer func() { finish(err) }()

	if apiToken.ID == "" {
		id, err := uuid.NewV7()
		if err != nil {
			return fmt.Errorf("generate api token id: %w", err)
		}

		apiToken.ID = id.String()
	}

	_, err = opCreateAPIToken.Invoke(ctx, db, apiToken)
	if err != nil {
		return err
	}

	return nil
}

func (db *Database) GetAPITokenByTokenID(ctx context.Context, tokenID string) (_ *APIToken, err error) {
	ctx, finish := startDBQuery(ctx, APITokensTableName, "SELECT")
	defer func() { finish(err) }()

	row, err := scanAPIToken(db.reader().QueryRowContext(ctx, getByTokenIDSQL, tokenID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return &row, nil
}

func (db *Database) DeleteAPIToken(ctx context.Context, id string) (err error) {
	ctx, finish := startDBQuery(ctx, APITokensTableName, "DELETE")
	defer func() { finish(err) }()

	_, err = opDeleteAPIToken.Invoke(ctx, db, &stringPayload{Value: id})
	if err != nil {
		return err
	}

	return nil
}

func (db *Database) CountAPITokens(ctx context.Context, userID string) (_ int, err error) {
	ctx, finish := startDBQuery(ctx, APITokensTableName, "SELECT")
	defer func() { finish(err) }()

	count, err := queryCount(ctx, db.reader(), countAPITokensSQL, userID)
	if err != nil {
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
