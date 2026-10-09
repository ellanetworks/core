// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/canonical/sqlair"
	"github.com/prometheus/client_golang/prometheus"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
)

const IMSRegistrationsTableName = "ims_registrations"

const imsRegistrationsSchema = 23

const (
	getIMSRegistrationStmt    = "SELECT &imsRegistrationRow.* FROM %s WHERE imsi==$imsRegistrationRow.imsi"
	upsertIMSRegistrationStmt = "INSERT INTO %s (imsi, state, serverName, authPending, originHost, originRealm, updatedAt) VALUES ($imsRegistrationRow.imsi, $imsRegistrationRow.state, $imsRegistrationRow.serverName, $imsRegistrationRow.authPending, $imsRegistrationRow.originHost, $imsRegistrationRow.originRealm, $imsRegistrationRow.updatedAt) ON CONFLICT(imsi) DO UPDATE SET state=excluded.state, serverName=excluded.serverName, authPending=excluded.authPending, originHost=excluded.originHost, originRealm=excluded.originRealm, updatedAt=excluded.updatedAt"
	deleteIMSRegistrationStmt = "DELETE FROM %s WHERE imsi==$imsRegistrationRow.imsi"
	countIMSRegistrationsStmt = "SELECT COUNT(*) AS &NumItems.count FROM %s WHERE state==$imsRegistrationRow.state"
	resetIMSRegistrationsSQL  = "DELETE FROM %s"
)

type IMSRegistrationState int

const (
	IMSNotRegistered IMSRegistrationState = 0
	IMSRegistered    IMSRegistrationState = 1
	IMSUnregistered  IMSRegistrationState = 2
)

type IMSRegistration struct {
	IMSI        string               `json:"imsi"`
	State       IMSRegistrationState `json:"state"`
	ServerName  string               `json:"serverName"`
	AuthPending bool                 `json:"authPending"`
	OriginHost  string               `json:"originHost"`
	OriginRealm string               `json:"originRealm"`
	UpdatedAt   int64                `json:"updatedAt"`
}

type imsRegistrationRow struct {
	Imsi        string `db:"imsi"`
	State       int    `db:"state"`
	ServerName  string `db:"serverName"`
	AuthPending bool   `db:"authPending"`
	OriginHost  string `db:"originHost"`
	OriginRealm string `db:"originRealm"`
	UpdatedAt   int64  `db:"updatedAt"`
}

type imsRegistrationCASPayload struct {
	IMSI     string           `json:"imsi"`
	Expected *IMSRegistration `json:"expected,omitempty"`
	Next     *IMSRegistration `json:"next,omitempty"`
}

type imsRegistrationCASResult struct {
	Swapped bool             `json:"swapped"`
	Current *IMSRegistration `json:"current,omitempty"`
}

func (r imsRegistrationRow) registration() *IMSRegistration {
	return &IMSRegistration{
		IMSI:        r.Imsi,
		State:       IMSRegistrationState(r.State),
		ServerName:  r.ServerName,
		AuthPending: r.AuthPending,
		OriginHost:  r.OriginHost,
		OriginRealm: r.OriginRealm,
		UpdatedAt:   r.UpdatedAt,
	}
}

func imsRegistrationRowOf(imsi string, r *IMSRegistration) imsRegistrationRow {
	return imsRegistrationRow{
		Imsi:        imsi,
		State:       int(r.State),
		ServerName:  r.ServerName,
		AuthPending: r.AuthPending,
		OriginHost:  r.OriginHost,
		OriginRealm: r.OriginRealm,
		UpdatedAt:   r.UpdatedAt,
	}
}

func sameIMSRegistration(a, b *IMSRegistration) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}

	return a.State == b.State &&
		a.ServerName == b.ServerName &&
		a.AuthPending == b.AuthPending &&
		a.OriginHost == b.OriginHost &&
		a.OriginRealm == b.OriginRealm &&
		a.UpdatedAt == b.UpdatedAt
}

func (db *Database) GetIMSRegistration(ctx context.Context, imsi string) (*IMSRegistration, error) {
	ctx, span := startIMSRegistrationsSpan(ctx, "SELECT")
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(IMSRegistrationsTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(IMSRegistrationsTableName, "select").Inc()

	if !db.appliedSchemaAtLeast(ctx, imsRegistrationsSchema) {
		return nil, nil
	}

	reg, err := db.getIMSRegistration(ctx, db.conn(), imsi)
	if err != nil {
		recordSpanError(span, err)

		return nil, err
	}

	return reg, nil
}

func (db *Database) CountIMSRegistrations(ctx context.Context, state IMSRegistrationState) (int, error) {
	ctx, span := startIMSRegistrationsSpan(ctx, "SELECT")
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(IMSRegistrationsTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(IMSRegistrationsTableName, "select").Inc()

	if !db.appliedSchemaAtLeast(ctx, imsRegistrationsSchema) {
		return 0, nil
	}

	var result NumItems

	if err := db.conn().Query(ctx, db.countIMSRegistrationsStmt, imsRegistrationRow{State: int(state)}).Get(&result); err != nil {
		recordSpanError(span, err)

		return 0, fmt.Errorf("query failed: %w", err)
	}

	return result.Count, nil
}

func (db *Database) CompareAndSwapIMSRegistration(ctx context.Context, imsi string, expected, next *IMSRegistration) (*IMSRegistration, bool, error) {
	ctx, span := startIMSRegistrationsSpan(ctx, "UPDATE")
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(IMSRegistrationsTableName, "update"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(IMSRegistrationsTableName, "update").Inc()

	result, err := opCompareAndSwapIMSRegistration.Invoke(ctx, db, &imsRegistrationCASPayload{IMSI: imsi, Expected: expected, Next: next})
	if err != nil {
		recordSpanError(span, err)

		return nil, false, err
	}

	if result == nil {
		err = fmt.Errorf("compare and swap IMS registration for subscriber %s: leader returned no result", imsi)
		recordSpanError(span, err)

		return nil, false, err
	}

	return result.Current, result.Swapped, nil
}

func (db *Database) applyCompareAndSwapIMSRegistration(ctx context.Context, payload *imsRegistrationCASPayload) (any, error) {
	current, err := db.getIMSRegistration(ctx, db.runner(ctx), payload.IMSI)
	if err != nil {
		return nil, err
	}

	if !sameIMSRegistration(current, payload.Expected) {
		return &imsRegistrationCASResult{Current: current}, nil
	}

	switch {
	case sameIMSRegistration(current, payload.Next):
	case payload.Next == nil:
		if err := db.runner(ctx).Query(ctx, db.deleteIMSRegistrationStmt, imsRegistrationRow{Imsi: payload.IMSI}).Run(); err != nil {
			return nil, fmt.Errorf("query failed: %w", err)
		}
	default:
		if err := db.runner(ctx).Query(ctx, db.upsertIMSRegistrationStmt, imsRegistrationRowOf(payload.IMSI, payload.Next)).Run(); err != nil {
			if isForeignKeyError(err) {
				return nil, ErrNotFound
			}

			return nil, fmt.Errorf("query failed: %w", err)
		}
	}

	next := payload.Next
	if next != nil {
		stored := *next
		stored.IMSI = payload.IMSI
		next = &stored
	}

	return &imsRegistrationCASResult{Swapped: true, Current: next}, nil
}

func (db *Database) getIMSRegistration(ctx context.Context, q *sqlair.DB, imsi string) (*IMSRegistration, error) {
	row := imsRegistrationRow{Imsi: imsi}

	if err := q.Query(ctx, db.getIMSRegistrationStmt, row).Get(&row); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return row.registration(), nil
}

func resetIMSRegistrationsInRestoredDB(ctx context.Context, conn *sql.DB) error {
	var count int
	if err := conn.QueryRowContext(ctx, fmt.Sprintf(ueRegistrationsTableExistsSQLFmt, IMSRegistrationsTableName)).Scan(&count); err != nil {
		return fmt.Errorf("look up %s: %w", IMSRegistrationsTableName, err)
	}

	if count == 0 {
		return nil
	}

	if _, err := conn.ExecContext(ctx, fmt.Sprintf(resetIMSRegistrationsSQL, IMSRegistrationsTableName)); err != nil {
		return fmt.Errorf("reset %s: %w", IMSRegistrationsTableName, err)
	}

	return nil
}

func startIMSRegistrationsSpan(ctx context.Context, operation string) (context.Context, trace.Span) {
	querySummary := fmt.Sprintf("%s %s", operation, IMSRegistrationsTableName)

	return tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName(operation),
			semconv.DBCollectionName(IMSRegistrationsTableName),
		),
	)
}
