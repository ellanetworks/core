package db

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
)

const JWTSecretTableName = "jwt_secret"

const (
	getJWTSecretStmt    = `SELECT &JWTSecret.* FROM %s WHERE singleton=TRUE`                                                                                  // #nosec: G101
	upsertJWTSecretStmt = `INSERT INTO %s (singleton, secret) VALUES (TRUE, $JWTSecret.secret) ON CONFLICT(singleton) DO UPDATE SET secret=$JWTSecret.secret` // #nosec: G101
)

type JWTSecret struct {
	Secret []byte `db:"secret"`
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

func (db *Database) GetJWTSecret(ctx context.Context) ([]byte, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "SELECT", JWTSecretTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			attribute.String("db.collection", JWTSecretTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(JWTSecretTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(JWTSecretTableName, "select").Inc()

	var row JWTSecret

	err := db.conn().Query(ctx, db.getJWTSecretStmt).Get(&row)
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

	return row.Secret, nil
}

func (db *Database) SetJWTSecret(ctx context.Context, secret []byte) error {
	_, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "UPSERT", JWTSecretTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("UPSERT"),
			attribute.String("db.collection", JWTSecretTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(JWTSecretTableName, "update"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(JWTSecretTableName, "update").Inc()

	_, err := opSetJWTSecret.Invoke(db, &bytesPayload{Value: secret})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return err
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

// RotateJWTSecret atomically updates the JWT secret and deletes all sessions
// within a single transaction. If either operation fails, both are rolled back.
func (db *Database) RotateJWTSecret(ctx context.Context, newSecret []byte) error {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "ROTATE", JWTSecretTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("ROTATE"),
			attribute.String("db.collection", JWTSecretTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(JWTSecretTableName, "rotate"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(JWTSecretTableName, "rotate").Inc()

	tx, err := db.conn().Begin(ctx, nil)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "begin transaction failed")

		return fmt.Errorf("begin transaction failed: %w", err)
	}

	row := JWTSecret{Secret: newSecret}

	err = tx.Query(ctx, db.upsertJWTSecretStmt, row).Run()
	if err != nil {
		_ = tx.Rollback()

		span.RecordError(err)
		span.SetStatus(codes.Error, "upsert secret failed")

		return fmt.Errorf("upsert secret failed: %w", err)
	}

	err = tx.Query(ctx, db.deleteAllSessionsStmt).Run()
	if err != nil {
		_ = tx.Rollback()

		span.RecordError(err)
		span.SetStatus(codes.Error, "delete sessions failed")

		return fmt.Errorf("delete sessions failed: %w", err)
	}

	if err := tx.Commit(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "commit failed")

		return fmt.Errorf("commit failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return nil
}
