// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
)

const JWTSecretTableName = "jwt_secret"

const (
	getJWTSecretSQL    = `SELECT secret FROM ` + JWTSecretTableName + ` WHERE singleton=TRUE`                                                                      // #nosec: G101
	upsertJWTSecretSQL = `INSERT INTO ` + JWTSecretTableName + ` (singleton, secret) VALUES (TRUE, ?) ON CONFLICT(singleton) DO UPDATE SET secret=excluded.secret` // #nosec: G101
)

type JWTSecret struct {
	Secret []byte
}

// InitializeJWTSecret generates and stores a JWT secret if one does not already exist.
// If a secret already exists, this is a no-op.
func (db *Database) InitializeJWTSecret(ctx context.Context) error {
	_, err := db.GetJWTSecret(ctx)
	if err == nil {
		return nil
	}

	if !errors.Is(err, ErrNotFound) {
		return fmt.Errorf("failed to check for existing JWT secret: %w", err)
	}

	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return fmt.Errorf("failed to generate JWT secret: %w", err)
	}

	return db.SetJWTSecret(ctx, secret)
}

func (db *Database) GetJWTSecret(ctx context.Context) (_ []byte, err error) {
	ctx, finish := startDBQuery(ctx, JWTSecretTableName, "SELECT")
	defer func() { finish(err) }()

	var row JWTSecret

	err = db.reader().QueryRowContext(ctx, getJWTSecretSQL).Scan(&row.Secret)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return row.Secret, nil
}

func (db *Database) SetJWTSecret(ctx context.Context, secret []byte) (err error) {
	ctx, finish := startDBQuery(ctx, JWTSecretTableName, "UPSERT")
	defer func() { finish(err) }()

	_, err = opSetJWTSecret.Invoke(ctx, db, &bytesPayload{Value: secret})
	if err != nil {
		return err
	}

	return nil
}

// RotateJWTSecret atomically updates the JWT secret and deletes all sessions
// within a single transaction. If either operation fails, both are rolled back.
func (db *Database) RotateJWTSecret(ctx context.Context, newSecret []byte) (err error) {
	ctx, finish := startDBQuery(ctx, JWTSecretTableName, "ROTATE")
	defer func() { finish(err) }()

	if _, err := opRotateJWTSecret.Invoke(ctx, db, &bytesPayload{Value: newSecret}); err != nil {
		return err
	}

	return nil
}
