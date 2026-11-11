// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// SPDX-FileCopyrightText: Ella Networks Inc.

package db

import (
	"context"
	"crypto/ecdh"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
)

const HomeNetworkKeysTableName = "home_network_keys"

// MaxHomeNetworkKeys is the maximum number of home network keys that can be stored.
// Key identifiers are 8-bit values (0–255) per 3GPP TS 33.501 §6.12.2, but we
// cap at 12 to match the maximum number of identifiers defined in the spec (0–11).
const MaxHomeNetworkKeys = 12

const (
	listHomeNetworkKeysSQL                    = "SELECT id, key_identifier, scheme, private_key FROM " + HomeNetworkKeysTableName + " ORDER BY scheme, key_identifier"
	getHomeNetworkKeySQL                      = "SELECT id, key_identifier, scheme, private_key FROM " + HomeNetworkKeysTableName + " WHERE id==?"
	getHomeNetworkKeyBySchemeAndIdentifierSQL = "SELECT id, key_identifier, scheme, private_key FROM " + HomeNetworkKeysTableName + " WHERE scheme==? AND key_identifier==?"
	createHomeNetworkKeySQL                   = "INSERT INTO " + HomeNetworkKeysTableName + " (id, key_identifier, scheme, private_key) VALUES (?, ?, ?, ?)"
	deleteHomeNetworkKeySQL                   = "DELETE FROM " + HomeNetworkKeysTableName + " WHERE id==?"
	countHomeNetworkKeysSQL                   = "SELECT COUNT(*) FROM " + HomeNetworkKeysTableName
)

// HomeNetworkKey represents a home network key used for SUCI de-concealment.
type HomeNetworkKey struct {
	ID            string // UUIDv7, generated at the request handler
	KeyIdentifier int
	Scheme        string // "A" or "B"
	PrivateKey    string
}

// GetPublicKey derives the public key from the private key based on the scheme.
func (k *HomeNetworkKey) GetPublicKey() (string, error) {
	switch k.Scheme {
	case "A":
		return deriveHomeNetworkPublicKey(k.PrivateKey)
	case "B":
		return deriveHomeNetworkPublicKeyProfileB(k.PrivateKey)
	default:
		return "", fmt.Errorf("unknown scheme: %s", k.Scheme)
	}
}

// deriveHomeNetworkPublicKeyProfileB derives a compressed P-256 public key from a hex private key.
func deriveHomeNetworkPublicKeyProfileB(privateKeyHex string) (string, error) {
	privBytes, err := hex.DecodeString(privateKeyHex)
	if err != nil {
		return "", fmt.Errorf("failed to decode Profile B private key hex: %w", err)
	}

	ecdhKey, err := ecdh.P256().NewPrivateKey(privBytes)
	if err != nil {
		return "", fmt.Errorf("failed to create P-256 private key: %w", err)
	}

	// crypto/ecdh returns uncompressed SEC1 (65 bytes: 0x04 || x || y).
	// Compress to 33 bytes (0x02/0x03 || x) without using deprecated elliptic.MarshalCompressed.
	pub := ecdhKey.PublicKey().Bytes()

	prefix := byte(0x02)
	if pub[64]%2 != 0 {
		prefix = 0x03
	}

	compressed := make([]byte, 33)
	compressed[0] = prefix
	copy(compressed[1:], pub[1:33])

	return hex.EncodeToString(compressed), nil
}

// ListHomeNetworkKeys returns all home network keys ordered by scheme and key identifier.
func (db *Database) ListHomeNetworkKeys(ctx context.Context) ([]HomeNetworkKey, error) {
	ctx, span, timer := startDBQuery(ctx, HomeNetworkKeysTableName, "SELECT", "select")
	defer span.End()
	defer timer.ObserveDuration()

	keys, err := queryAll(ctx, db.reader(), listHomeNetworkKeysSQL, scanHomeNetworkKey)
	if err != nil {
		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return keys, nil
}

// GetHomeNetworkKey retrieves a home network key by its UUID.
func (db *Database) GetHomeNetworkKey(ctx context.Context, id string) (*HomeNetworkKey, error) {
	ctx, span, timer := startDBQuery(ctx, HomeNetworkKeysTableName, "SELECT", "select")
	defer span.End()
	defer timer.ObserveDuration()

	row, err := scanHomeNetworkKey(db.reader().QueryRowContext(ctx, getHomeNetworkKeySQL, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return &row, nil
}

// GetHomeNetworkKeyBySchemeAndIdentifier retrieves a home network key by its (scheme, keyIdentifier) pair.
func (db *Database) GetHomeNetworkKeyBySchemeAndIdentifier(ctx context.Context, scheme string, keyIdentifier int) (*HomeNetworkKey, error) {
	ctx, span, timer := startDBQuery(ctx, HomeNetworkKeysTableName, "SELECT", "select")
	defer span.End()
	defer timer.ObserveDuration()

	row, err := scanHomeNetworkKey(db.reader().QueryRowContext(ctx, getHomeNetworkKeyBySchemeAndIdentifierSQL, scheme, keyIdentifier))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return &row, nil
}

// CreateHomeNetworkKey inserts a new home network key.
func (db *Database) CreateHomeNetworkKey(ctx context.Context, key *HomeNetworkKey) error {
	ctx, span, timer := startDBQuery(ctx, HomeNetworkKeysTableName, "INSERT", "insert")
	defer span.End()
	defer timer.ObserveDuration()

	if key.ID == "" {
		err := fmt.Errorf("CreateHomeNetworkKey: ID must be set by the caller")
		recordSpanError(span, err)

		return err
	}

	_, err := opCreateHomeNetworkKey.Invoke(ctx, db, key)
	if err != nil {
		recordSpanError(span, err)

		return err
	}

	return nil
}

// DeleteHomeNetworkKey removes a home network key by its UUID.
func (db *Database) DeleteHomeNetworkKey(ctx context.Context, id string) error {
	ctx, span, timer := startDBQuery(ctx, HomeNetworkKeysTableName, "DELETE", "delete")
	defer span.End()
	defer timer.ObserveDuration()

	_, err := opDeleteHomeNetworkKey.Invoke(ctx, db, &stringPayload{Value: id})
	if err != nil {
		recordSpanError(span, err)

		return err
	}

	return nil
}

// CountHomeNetworkKeys returns the number of home network keys.
func (db *Database) CountHomeNetworkKeys(ctx context.Context) (int, error) {
	ctx, span, timer := startDBQuery(ctx, HomeNetworkKeysTableName, "SELECT", "select")
	defer span.End()
	defer timer.ObserveDuration()

	count, err := queryCount(ctx, db.reader(), countHomeNetworkKeysSQL)
	if err != nil {
		recordSpanError(span, err)

		return 0, fmt.Errorf("query failed: %w", err)
	}

	return count, nil
}

func scanHomeNetworkKey(row scanner) (HomeNetworkKey, error) {
	var v HomeNetworkKey

	if err := row.Scan(&v.ID, &v.KeyIdentifier, &v.Scheme, &v.PrivateKey); err != nil {
		return HomeNetworkKey{}, err
	}

	return v, nil
}
