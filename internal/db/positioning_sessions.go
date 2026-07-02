// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/canonical/sqlair"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
)

const (
	PositioningSessionsTableName = "positioning_sessions"

	SessionTypeImmediate = 0
	SessionTypePeriodic  = 1
	SessionTypeTriggered = 2

	SessionStatusActive    = 0
	SessionStatusCompleted = 1
	SessionStatusFailed    = 2
	SessionStatusCancelled = 3
)

const (
	createPositioningSessionStmt      = `INSERT INTO %s (id, supi, session_type, method, qos_response_time_ms, qos_horizontal_accuracy_m, status, last_result, created_at, updated_at) VALUES ($PositioningSession.id, $PositioningSession.supi, $PositioningSession.session_type, $PositioningSession.method, $PositioningSession.qos_response_time_ms, $PositioningSession.qos_horizontal_accuracy_m, $PositioningSession.status, $PositioningSession.last_result, $PositioningSession.created_at, $PositioningSession.updated_at);`
	getPositioningSessionStmt         = `SELECT &PositioningSession.* FROM %s WHERE id==$PositioningSession.id;`
	listPositioningSessionsBySupiStmt = `SELECT &PositioningSession.* FROM %s WHERE supi==$PositioningSession.supi AND status==$PositioningSession.status ORDER BY created_at DESC;`
	listPositioningSessionsAllStmt    = `SELECT &PositioningSession.* FROM %s WHERE supi==$PositioningSession.supi ORDER BY created_at DESC;`
	updatePositioningSessionStmt      = `UPDATE %s SET status==$PositioningSession.status, last_result==$PositioningSession.last_result, updated_at==$PositioningSession.updated_at WHERE id==$PositioningSession.id;`
	deletePositioningSessionStmt      = `DELETE FROM %s WHERE id==$PositioningSession.id;`
)

// PositioningSession tracks a single positioning request from a UE.
type PositioningSession struct {
	ID                string  `db:"id"`
	SUPI              string  `db:"supi"`
	SessionType       int     `db:"session_type"`
	Method            string  `db:"method"`
	QoSResponseTimeMs *int    `db:"qos_response_time_ms"`
	QOSHAccuracyM     *int    `db:"qos_horizontal_accuracy_m"`
	Status            int     `db:"status"`
	LastResult        *string `db:"last_result"`
	CreatedAt         string  `db:"created_at"`
	UpdatedAt         string  `db:"updated_at"`
}

type PositioningSessionResult struct {
	Result json.RawMessage `json:"result"`
}

func (db *Database) CreatePositioningSession(ctx context.Context, s *PositioningSession) error {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "INSERT", PositioningSessionsTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("INSERT"),
			attribute.String("db.collection", PositioningSessionsTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(PositioningSessionsTableName, "insert"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(PositioningSessionsTableName, "insert").Inc()

	if s.ID == "" {
		id, err := uuid.NewV7()
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "uuid generation failed")

			return fmt.Errorf("generate session id: %w", err)
		}

		s.ID = id.String()
	}

	now := time.Now().Format(time.RFC3339)
	s.CreatedAt = now
	s.UpdatedAt = now

	err := db.conn().Query(ctx, db.createPositioningSessionStmt, s).Run()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

func (db *Database) GetPositioningSession(ctx context.Context, id string) (*PositioningSession, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "SELECT", PositioningSessionsTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			attribute.String("db.collection", PositioningSessionsTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(PositioningSessionsTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(PositioningSessionsTableName, "select").Inc()

	var s PositioningSession

	filter := PositioningSession{ID: id}

	err := db.conn().Query(ctx, db.getPositioningSessionStmt, filter).Get(&s)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return nil, fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return &s, nil
}

func (db *Database) ListPositioningSessions(ctx context.Context, supi string, status int) ([]PositioningSession, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "SELECT", PositioningSessionsTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			attribute.String("db.collection", PositioningSessionsTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(PositioningSessionsTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(PositioningSessionsTableName, "select").Inc()

	var (
		sessions []PositioningSession
		err      error
	)

	filter := PositioningSession{SUPI: supi}
	if status >= 0 {
		filter.Status = status
		err = db.conn().Query(ctx, db.listPositioningSessionsBySupiStmt, filter).GetAll(&sessions)
	} else {
		// List all sessions for this SUPI (no status filter)
		err = db.conn().Query(ctx, db.listPositioningSessionsAllStmt, filter).GetAll(&sessions)
	}

	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return nil, fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return sessions, nil
}

func (db *Database) UpdatePositioningSessionStatus(ctx context.Context, id string, status int, result *string) error {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "UPDATE", PositioningSessionsTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("UPDATE"),
			attribute.String("db.collection", PositioningSessionsTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(PositioningSessionsTableName, "update"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(PositioningSessionsTableName, "update").Inc()

	now := time.Now().Format(time.RFC3339)

	filter := PositioningSession{ID: id, Status: status, LastResult: result, UpdatedAt: now}

	err := db.conn().Query(ctx, db.updatePositioningSessionStmt, filter).Run()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

func (db *Database) DeletePositioningSession(ctx context.Context, id string) error {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "DELETE", PositioningSessionsTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("DELETE"),
			attribute.String("db.collection", PositioningSessionsTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(PositioningSessionsTableName, "delete"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(PositioningSessionsTableName, "delete").Inc()

	var outcome sqlair.Outcome

	err := db.conn().Query(ctx, db.deletePositioningSessionStmt, PositioningSession{ID: id}).Get(&outcome)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return fmt.Errorf("query failed: %w", err)
	}

	rowsAffected, err := outcome.Result().RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}

	if rowsAffected == 0 {
		span.SetStatus(codes.Error, "not found")
		return ErrNotFound
	}

	span.SetStatus(codes.Ok, "")

	return nil
}
