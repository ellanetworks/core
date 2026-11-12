// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
)

const IPSMGWRegistrationsTableName = "ip_sm_gw_registrations"

const ipSMGWRegistrationsSchema = 25

const (
	getIPSMGWRegistrationSQL      = "SELECT imsi, number, diameterHost, diameterRealm FROM " + IPSMGWRegistrationsTableName + " WHERE imsi==?"
	listAllIPSMGWRegistrationsSQL = "SELECT imsi, number, diameterHost, diameterRealm FROM " + IPSMGWRegistrationsTableName + " ORDER BY imsi ASC"
	upsertIPSMGWRegistrationSQL   = "INSERT INTO " + IPSMGWRegistrationsTableName + " (imsi, number, diameterHost, diameterRealm) VALUES (?, ?, ?, ?) ON CONFLICT(imsi) DO UPDATE SET number=excluded.number, diameterHost=excluded.diameterHost, diameterRealm=excluded.diameterRealm"
	deleteIPSMGWRegistrationSQL   = "DELETE FROM " + IPSMGWRegistrationsTableName + " WHERE imsi==?"
)

type IPSMGWRegistration struct {
	IMSI          string `json:"imsi"`
	Number        string `json:"number"`
	DiameterHost  string `json:"diameterHost"`
	DiameterRealm string `json:"diameterRealm"`
}

type ipSMGWRegistrationRow struct {
	Imsi          string
	Number        string
	DiameterHost  string
	DiameterRealm string
}

type ipSMGWRegistrationPayload struct {
	Imsi          string `json:"imsi"`
	Number        string `json:"number,omitempty"`
	DiameterHost  string `json:"diameterHost,omitempty"`
	DiameterRealm string `json:"diameterRealm,omitempty"`
}

func (r ipSMGWRegistrationRow) registration() *IPSMGWRegistration {
	return &IPSMGWRegistration{IMSI: r.Imsi, Number: r.Number, DiameterHost: r.DiameterHost, DiameterRealm: r.DiameterRealm}
}

func (db *Database) GetIPSMGWRegistration(ctx context.Context, imsi string) (*IPSMGWRegistration, error) {
	ctx, span := startIPSMGWRegistrationSpan(ctx, "SELECT")
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(IPSMGWRegistrationsTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(IPSMGWRegistrationsTableName, "select").Inc()

	if !db.appliedSchemaAtLeast(ctx, ipSMGWRegistrationsSchema) {
		return nil, ErrNotFound
	}

	row, err := scanIPSMGWRegistrationRow(db.reader().QueryRowContext(ctx, getIPSMGWRegistrationSQL, imsi))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return row.registration(), nil
}

func (db *Database) ListAllIPSMGWRegistrations(ctx context.Context) ([]IPSMGWRegistration, error) {
	ctx, span := startIPSMGWRegistrationSpan(ctx, "SELECT")
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(IPSMGWRegistrationsTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(IPSMGWRegistrationsTableName, "select").Inc()

	if !db.appliedSchemaAtLeast(ctx, ipSMGWRegistrationsSchema) {
		return nil, nil
	}

	rows, err := queryAll(ctx, db.reader(), listAllIPSMGWRegistrationsSQL, scanIPSMGWRegistrationRow)
	if err != nil {
		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	out := make([]IPSMGWRegistration, 0, len(rows))
	for _, r := range rows {
		out = append(out, *r.registration())
	}

	return out, nil
}

func (db *Database) PutIPSMGWRegistration(ctx context.Context, r IPSMGWRegistration) error {
	if r.IMSI == "" || r.Number == "" || r.DiameterHost == "" || r.DiameterRealm == "" {
		return fmt.Errorf("IMSI, number and Diameter identity are required")
	}

	return db.invokeIPSMGWRegistration(ctx, "UPSERT", opPutIPSMGWRegistration, &ipSMGWRegistrationPayload{
		Imsi: r.IMSI, Number: r.Number, DiameterHost: r.DiameterHost, DiameterRealm: r.DiameterRealm,
	})
}

func (db *Database) DeleteIPSMGWRegistration(ctx context.Context, imsi string) error {
	return db.invokeIPSMGWRegistration(ctx, "DELETE", opDeleteIPSMGWRegistration, &ipSMGWRegistrationPayload{Imsi: imsi})
}

func (db *Database) invokeIPSMGWRegistration(ctx context.Context, operation string, op *ChangesetOp[ipSMGWRegistrationPayload, struct{}], p *ipSMGWRegistrationPayload) error {
	ctx, span := startIPSMGWRegistrationSpan(ctx, operation)
	defer span.End()

	label := strings.ToLower(operation)

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(IPSMGWRegistrationsTableName, label))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(IPSMGWRegistrationsTableName, label).Inc()

	if _, err := op.Invoke(ctx, db, p); err != nil {
		recordSpanError(span, err)
		return err
	}

	return nil
}

func (db *Database) applyPutIPSMGWRegistration(ctx context.Context, p *ipSMGWRegistrationPayload) (any, error) {
	row := ipSMGWRegistrationRow{Imsi: p.Imsi, Number: p.Number, DiameterHost: p.DiameterHost, DiameterRealm: p.DiameterRealm}

	if _, err := db.runner(ctx).ExecContext(ctx, upsertIPSMGWRegistrationSQL, row.Imsi, row.Number, row.DiameterHost, row.DiameterRealm); err != nil {
		if isForeignKeyError(err) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyDeleteIPSMGWRegistration(ctx context.Context, p *ipSMGWRegistrationPayload) (any, error) {
	if _, err := db.runner(ctx).ExecContext(ctx, deleteIPSMGWRegistrationSQL, p.Imsi); err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func startIPSMGWRegistrationSpan(ctx context.Context, operation string) (context.Context, trace.Span) {
	querySummary := fmt.Sprintf("%s %s", operation, IPSMGWRegistrationsTableName)

	return tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName(operation),
			semconv.DBCollectionName(IPSMGWRegistrationsTableName),
		),
	)
}

func scanIPSMGWRegistrationRow(row scanner) (ipSMGWRegistrationRow, error) {
	var v ipSMGWRegistrationRow

	if err := row.Scan(&v.Imsi, &v.Number, &v.DiameterHost, &v.DiameterRealm); err != nil {
		return ipSMGWRegistrationRow{}, err
	}

	return v, nil
}
