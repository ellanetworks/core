// Copyright 2026 Ella Networks

package db

import (
	"context"
	"crypto/ecdh"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
)

const HomeNetworkKeysTableName = "home_network_keys"

// MaxHomeNetworkKeys is the maximum number of home network keys that can be stored.
// Key identifiers are 8-bit values (0–255) per 3GPP TS 33.501 §6.12.2, but we
// cap at 12 to match the maximum number of identifiers defined in the spec (0–11).
const MaxHomeNetworkKeys = 12

const (
	listHomeNetworkKeysStmtStr                    = "SELECT &HomeNetworkKey.* FROM %s ORDER BY scheme, key_identifier"
	getHomeNetworkKeyStmtStr                      = "SELECT &HomeNetworkKey.* FROM %s WHERE id==$HomeNetworkKey.id"
	getHomeNetworkKeyBySchemeAndIdentifierStmtStr = "SELECT &HomeNetworkKey.* FROM %s WHERE scheme==$HomeNetworkKey.scheme AND key_identifier==$HomeNetworkKey.key_identifier"
	createHomeNetworkKeyStmtStr                   = "INSERT INTO %s (key_identifier, scheme, private_key) VALUES ($HomeNetworkKey.key_identifier, $HomeNetworkKey.scheme, $HomeNetworkKey.private_key)"
	deleteHomeNetworkKeyStmtStr                   = "DELETE FROM %s WHERE id==$HomeNetworkKey.id"
	countHomeNetworkKeysStmtStr                   = "SELECT COUNT(*) AS &NumItems.count FROM %s"
)

// HomeNetworkKey represents a home network key used for SUCI de-concealment.
type HomeNetworkKey struct {
	ID            int    `db:"id"`
	KeyIdentifier int    `db:"key_identifier"`
	Scheme        string `db:"scheme"` // "A" or "B"
	PrivateKey    string `db:"private_key"`
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
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "SELECT", HomeNetworkKeysTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			attribute.String("db.collection", HomeNetworkKeysTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(HomeNetworkKeysTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(HomeNetworkKeysTableName, "select").Inc()

	var keys []HomeNetworkKey

	err := db.conn.Query(ctx, db.listHomeNetworkKeysStmt).GetAll(&keys)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			span.SetStatus(codes.Ok, "no rows")
			return nil, nil
		}

		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return nil, fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return keys, nil
}

// GetHomeNetworkKey retrieves a home network key by its database row ID.
func (db *Database) GetHomeNetworkKey(ctx context.Context, id int) (*HomeNetworkKey, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "SELECT", HomeNetworkKeysTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			attribute.String("db.collection", HomeNetworkKeysTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(HomeNetworkKeysTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(HomeNetworkKeysTableName, "select").Inc()

	row := HomeNetworkKey{ID: id}

	err := db.conn.Query(ctx, db.getHomeNetworkKeyStmt, row).Get(&row)
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

// GetHomeNetworkKeyBySchemeAndIdentifier retrieves a home network key by its (scheme, keyIdentifier) pair.
func (db *Database) GetHomeNetworkKeyBySchemeAndIdentifier(ctx context.Context, scheme string, keyIdentifier int) (*HomeNetworkKey, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "SELECT", HomeNetworkKeysTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			attribute.String("db.collection", HomeNetworkKeysTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(HomeNetworkKeysTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(HomeNetworkKeysTableName, "select").Inc()

	row := HomeNetworkKey{Scheme: scheme, KeyIdentifier: keyIdentifier}

	err := db.conn.Query(ctx, db.getHomeNetworkKeyBySchemeAndIdentifierStmt, row).Get(&row)
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

// CreateHomeNetworkKey inserts a new home network key.
func (db *Database) CreateHomeNetworkKey(ctx context.Context, key *HomeNetworkKey) error {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "INSERT", HomeNetworkKeysTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("INSERT"),
			attribute.String("db.collection", HomeNetworkKeysTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(HomeNetworkKeysTableName, "insert"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(HomeNetworkKeysTableName, "insert").Inc()

	err := db.conn.Query(ctx, db.createHomeNetworkKeyStmt, key).Run()
	if err != nil {
		if isUniqueNameError(err) {
			span.RecordError(ErrAlreadyExists)
			span.SetStatus(codes.Error, "already exists")

			return ErrAlreadyExists
		}

		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

// DeleteHomeNetworkKey removes a home network key by its database row ID.
func (db *Database) DeleteHomeNetworkKey(ctx context.Context, id int) error {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "DELETE", HomeNetworkKeysTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("DELETE"),
			attribute.String("db.collection", HomeNetworkKeysTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(HomeNetworkKeysTableName, "delete"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(HomeNetworkKeysTableName, "delete").Inc()

	err := db.conn.Query(ctx, db.deleteHomeNetworkKeyStmt, HomeNetworkKey{ID: id}).Run()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

// CountHomeNetworkKeys returns the number of home network keys.
func (db *Database) CountHomeNetworkKeys(ctx context.Context) (int, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "SELECT", HomeNetworkKeysTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			attribute.String("db.collection", HomeNetworkKeysTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(HomeNetworkKeysTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(HomeNetworkKeysTableName, "select").Inc()

	var result NumItems

	err := db.conn.Query(ctx, db.countHomeNetworkKeysStmt).Get(&result)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return 0, fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return result.Count, nil
}
