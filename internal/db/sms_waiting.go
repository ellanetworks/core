// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
)

const SMSWaitingRetention = 7 * 24 * time.Hour

const (
	SMSWaitingTableName        = "sms_message_waiting"
	SMSWaitingCentresTableName = "sms_message_waiting_centres"
)

const (
	getSMSWaitingStmt             = "SELECT &smsWaitingRow.* FROM %s WHERE imsi==$smsWaitingRow.imsi"
	listSMSWaitingCentresStmt     = "SELECT &smsWaitingCentre.* FROM %s WHERE imsi==$smsWaitingCentre.imsi ORDER BY serviceCentre"
	upsertSMSWaitingStmt          = "INSERT INTO %s (imsi, mcef, updatedAt) VALUES ($smsWaitingRow.imsi, $smsWaitingRow.mcef, $smsWaitingRow.updatedAt) ON CONFLICT(imsi) DO UPDATE SET mcef=MAX(mcef, excluded.mcef), updatedAt=excluded.updatedAt"
	insertSMSWaitingCentreStmt    = "INSERT INTO %s (imsi, serviceCentre) VALUES ($smsWaitingCentre.imsi, $smsWaitingCentre.serviceCentre) ON CONFLICT(imsi, serviceCentre) DO NOTHING"
	clearSMSMemoryFullStmt        = "UPDATE %s SET mcef=0, updatedAt=$smsWaitingRow.updatedAt WHERE imsi==$smsWaitingRow.imsi"
	deleteSMSWaitingCentreStmt    = "DELETE FROM %s WHERE imsi==$smsWaitingCentre.imsi AND serviceCentre==$smsWaitingCentre.serviceCentre"
	deleteSMSWaitingCentresStmt   = "DELETE FROM %s WHERE imsi==$smsWaitingCentre.imsi"
	deleteSMSWaitingStmt          = "DELETE FROM %s WHERE imsi==$smsWaitingRow.imsi"
	deleteSMSWaitingIfEmptyStmt   = "DELETE FROM %s WHERE imsi==$smsWaitingRow.imsi AND NOT EXISTS (SELECT 1 FROM %s WHERE imsi==$smsWaitingRow.imsi)"
	deleteStaleSMSWaitingCentStmt = "DELETE FROM %s WHERE imsi IN (SELECT imsi FROM %s WHERE updatedAt < $smsWaitingRow.updatedAt)"
	deleteStaleSMSWaitingStmt     = "DELETE FROM %s WHERE updatedAt < $smsWaitingRow.updatedAt"
)

type SMSWaiting struct {
	IMSI           string
	MemoryFull     bool
	ServiceCentres []string
	UpdatedAt      time.Time
}

type SMSWaitingUpdate struct {
	IMSI          string
	ServiceCentre string
	MemoryFull    bool
}

type smsWaitingRow struct {
	Imsi      string `db:"imsi"`
	MCEF      bool   `db:"mcef"`
	UpdatedAt int64  `db:"updatedAt"`
}

type smsWaitingCentre struct {
	Imsi          string `db:"imsi"`
	ServiceCentre string `db:"serviceCentre"`
}

type recordSMSWaitingPayload struct {
	Imsi          string `json:"imsi"`
	ServiceCentre string `json:"service_centre"`
	MemoryFull    bool   `json:"mcef"`
	UpdatedAt     int64  `json:"updated_at"`
}

type smsWaitingCentrePayload struct {
	Imsi          string `json:"imsi"`
	ServiceCentre string `json:"service_centre,omitempty"`
	UpdatedAt     int64  `json:"updated_at,omitempty"`
}

type smsWaitingCutoffPayload struct {
	Cutoff int64 `json:"cutoff"`
}

func (db *Database) GetSMSWaiting(ctx context.Context, imsi string) (*SMSWaiting, error) {
	ctx, span := startSMSWaitingSpan(ctx, "SELECT")
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(SMSWaitingTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(SMSWaitingTableName, "select").Inc()

	if !db.appliedSchemaAtLeast(ctx, smsSchema) {
		return nil, ErrNotFound
	}

	row := smsWaitingRow{Imsi: imsi}

	if err := db.conn().Query(ctx, db.getSMSWaitingStmt, row).Get(&row); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	var centres []smsWaitingCentre

	if err := db.conn().Query(ctx, db.listSMSWaitingCentresStmt, smsWaitingCentre{Imsi: imsi}).GetAll(&centres); err != nil && !errors.Is(err, sql.ErrNoRows) {
		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	w := &SMSWaiting{
		IMSI:       row.Imsi,
		MemoryFull: row.MCEF,
		UpdatedAt:  time.Unix(row.UpdatedAt, 0).UTC(),
	}

	for _, c := range centres {
		w.ServiceCentres = append(w.ServiceCentres, c.ServiceCentre)
	}

	return w, nil
}

func (db *Database) RecordSMSWaiting(ctx context.Context, u SMSWaitingUpdate) error {
	if u.IMSI == "" || u.ServiceCentre == "" {
		return fmt.Errorf("IMSI and service centre are required")
	}

	ctx, span := startSMSWaitingSpan(ctx, "UPSERT")
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(SMSWaitingTableName, "upsert"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(SMSWaitingTableName, "upsert").Inc()

	_, err := opRecordSMSWaiting.Invoke(ctx, db, &recordSMSWaitingPayload{
		Imsi:          u.IMSI,
		ServiceCentre: u.ServiceCentre,
		MemoryFull:    u.MemoryFull,
		UpdatedAt:     time.Now().Unix(),
	})
	if err != nil {
		recordSpanError(span, err)
		return err
	}

	return nil
}

func (db *Database) ClearSMSMemoryFull(ctx context.Context, imsi string) error {
	return db.invokeSMSWaiting(ctx, "UPDATE", opClearSMSMemoryFull, &smsWaitingCentrePayload{Imsi: imsi, UpdatedAt: time.Now().Unix()})
}

func (db *Database) RemoveSMSWaitingCentre(ctx context.Context, imsi, serviceCentre string) error {
	return db.invokeSMSWaiting(ctx, "DELETE", opRemoveSMSWaitingCentre, &smsWaitingCentrePayload{Imsi: imsi, ServiceCentre: serviceCentre, UpdatedAt: time.Now().Unix()})
}

func (db *Database) DeleteSMSWaiting(ctx context.Context, imsi string) error {
	return db.invokeSMSWaiting(ctx, "DELETE", opDeleteSMSWaiting, &smsWaitingCentrePayload{Imsi: imsi})
}

func (db *Database) DeleteStaleSMSWaiting(ctx context.Context, maxAge time.Duration) error {
	ctx, span := startSMSWaitingSpan(ctx, "DELETE")
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(SMSWaitingTableName, "delete"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(SMSWaitingTableName, "delete").Inc()

	if !db.appliedSchemaAtLeast(ctx, smsSchema) {
		return nil
	}

	if _, err := opDeleteStaleSMSWaiting.Invoke(ctx, db, &smsWaitingCutoffPayload{Cutoff: time.Now().Add(-maxAge).Unix()}); err != nil {
		recordSpanError(span, err)
		return err
	}

	return nil
}

func (db *Database) invokeSMSWaiting(ctx context.Context, operation string, op *ChangesetOp[smsWaitingCentrePayload, struct{}], p *smsWaitingCentrePayload) error {
	ctx, span := startSMSWaitingSpan(ctx, operation)
	defer span.End()

	label := strings.ToLower(operation)

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(SMSWaitingTableName, label))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(SMSWaitingTableName, label).Inc()

	if _, err := op.Invoke(ctx, db, p); err != nil {
		recordSpanError(span, err)
		return err
	}

	return nil
}

func (db *Database) applyRecordSMSWaiting(ctx context.Context, p *recordSMSWaitingPayload) (any, error) {
	row := smsWaitingRow{Imsi: p.Imsi, MCEF: p.MemoryFull, UpdatedAt: p.UpdatedAt}

	if err := db.runner(ctx).Query(ctx, db.upsertSMSWaitingStmt, row).Run(); err != nil {
		if isForeignKeyError(err) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	if err := db.runner(ctx).Query(ctx, db.insertSMSWaitingCentreStmt, smsWaitingCentre{Imsi: p.Imsi, ServiceCentre: p.ServiceCentre}).Run(); err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyClearSMSMemoryFull(ctx context.Context, p *smsWaitingCentrePayload) (any, error) {
	if err := db.runner(ctx).Query(ctx, db.clearSMSMemoryFullStmt, smsWaitingRow{Imsi: p.Imsi, UpdatedAt: p.UpdatedAt}).Run(); err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyRemoveSMSWaitingCentre(ctx context.Context, p *smsWaitingCentrePayload) (any, error) {
	if err := db.runner(ctx).Query(ctx, db.deleteSMSWaitingCentreStmt, smsWaitingCentre{Imsi: p.Imsi, ServiceCentre: p.ServiceCentre}).Run(); err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	if err := db.runner(ctx).Query(ctx, db.clearSMSMemoryFullStmt, smsWaitingRow{Imsi: p.Imsi, UpdatedAt: p.UpdatedAt}).Run(); err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	if err := db.runner(ctx).Query(ctx, db.deleteSMSWaitingIfEmptyStmt, smsWaitingRow{Imsi: p.Imsi}).Run(); err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyDeleteSMSWaiting(ctx context.Context, p *smsWaitingCentrePayload) (any, error) {
	if err := db.runner(ctx).Query(ctx, db.deleteSMSWaitingCentresStmt, smsWaitingCentre{Imsi: p.Imsi}).Run(); err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	if err := db.runner(ctx).Query(ctx, db.deleteSMSWaitingStmt, smsWaitingRow{Imsi: p.Imsi}).Run(); err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyDeleteStaleSMSWaiting(ctx context.Context, p *smsWaitingCutoffPayload) (any, error) {
	cutoff := smsWaitingRow{UpdatedAt: p.Cutoff}

	if err := db.runner(ctx).Query(ctx, db.deleteStaleSMSWaitingCentStmt, cutoff).Run(); err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	if err := db.runner(ctx).Query(ctx, db.deleteStaleSMSWaitingStmt, cutoff).Run(); err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func startSMSWaitingSpan(ctx context.Context, operation string) (context.Context, trace.Span) {
	querySummary := fmt.Sprintf("%s %s", operation, SMSWaitingTableName)

	return tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName(operation),
			semconv.DBCollectionName(SMSWaitingTableName),
		),
	)
}
