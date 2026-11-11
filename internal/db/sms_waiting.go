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
	getSMSWaitingStmt             = "SELECT imsi, mcef, updatedAt FROM " + SMSWaitingTableName + " WHERE imsi==?"
	listAllSMSWaitingStmt         = "SELECT imsi, mcef, updatedAt FROM " + SMSWaitingTableName + " ORDER BY imsi ASC"
	listAllSMSWaitingCentresStmt  = "SELECT imsi, serviceCentre FROM " + SMSWaitingCentresTableName + " ORDER BY imsi, serviceCentre"
	listSMSWaitingCentresStmt     = "SELECT imsi, serviceCentre FROM " + SMSWaitingCentresTableName + " WHERE imsi==? ORDER BY serviceCentre"
	upsertSMSWaitingStmt          = "INSERT INTO " + SMSWaitingTableName + " (imsi, mcef, updatedAt) VALUES (?, ?, ?) ON CONFLICT(imsi) DO UPDATE SET mcef=MAX(mcef, excluded.mcef), updatedAt=excluded.updatedAt"
	insertSMSWaitingCentreStmt    = "INSERT INTO " + SMSWaitingCentresTableName + " (imsi, serviceCentre) VALUES (?, ?) ON CONFLICT(imsi, serviceCentre) DO NOTHING"
	clearSMSMemoryFullStmt        = "UPDATE " + SMSWaitingTableName + " SET mcef=0, updatedAt=? WHERE imsi==?"
	deleteSMSWaitingCentreStmt    = "DELETE FROM " + SMSWaitingCentresTableName + " WHERE imsi==? AND serviceCentre==?"
	deleteSMSWaitingCentresStmt   = "DELETE FROM " + SMSWaitingCentresTableName + " WHERE imsi==?"
	deleteSMSWaitingStmt          = "DELETE FROM " + SMSWaitingTableName + " WHERE imsi==?"
	deleteSMSWaitingIfEmptyStmt   = "DELETE FROM " + SMSWaitingTableName + " WHERE imsi==? AND NOT EXISTS (SELECT 1 FROM " + SMSWaitingCentresTableName + " WHERE imsi==?)"
	deleteStaleSMSWaitingCentStmt = "DELETE FROM " + SMSWaitingCentresTableName + " WHERE imsi IN (SELECT imsi FROM " + SMSWaitingTableName + " WHERE updatedAt < ?)"
	deleteStaleSMSWaitingStmt     = "DELETE FROM " + SMSWaitingTableName + " WHERE updatedAt < ?"
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
	Imsi      string
	MCEF      bool
	UpdatedAt int64
}

type smsWaitingCentre struct {
	Imsi          string
	ServiceCentre string
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

func (db *Database) ListAllSMSWaiting(ctx context.Context) ([]SMSWaiting, error) {
	ctx, span := startSMSWaitingSpan(ctx, "SELECT")
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(SMSWaitingTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(SMSWaitingTableName, "select").Inc()

	if !db.appliedSchemaAtLeast(ctx, smsSchema) {
		return nil, nil
	}

	rows, err := queryAll(ctx, db.reader(), listAllSMSWaitingStmt, scanSMSWaitingRow)
	if err != nil {
		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	if len(rows) == 0 {
		return nil, nil
	}

	waiting := make([]SMSWaiting, 0, len(rows))

	centres, err := queryAll(ctx, db.reader(), listAllSMSWaitingCentresStmt, scanSMSWaitingCentre)
	if err != nil {
		recordSpanError(span, err)
		return nil, fmt.Errorf("query failed: %w", err)
	}

	byIMSI := make(map[string][]string)
	for _, centre := range centres {
		byIMSI[centre.Imsi] = append(byIMSI[centre.Imsi], centre.ServiceCentre)
	}

	for _, row := range rows {
		w := SMSWaiting{
			IMSI:           row.Imsi,
			MemoryFull:     row.MCEF,
			UpdatedAt:      time.Unix(row.UpdatedAt, 0).UTC(),
			ServiceCentres: byIMSI[row.Imsi],
		}

		waiting = append(waiting, w)
	}

	return waiting, nil
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

	row, err := scanSMSWaitingRow(db.reader().QueryRowContext(ctx, getSMSWaitingStmt, imsi))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	centres, err := queryAll(ctx, db.reader(), listSMSWaitingCentresStmt, scanSMSWaitingCentre, imsi)
	if err != nil {
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

	if _, err := db.runner(ctx).ExecContext(ctx, upsertSMSWaitingStmt, row.Imsi, row.MCEF, row.UpdatedAt); err != nil {
		if isForeignKeyError(err) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	if _, err := db.runner(ctx).ExecContext(ctx, insertSMSWaitingCentreStmt, p.Imsi, p.ServiceCentre); err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyClearSMSMemoryFull(ctx context.Context, p *smsWaitingCentrePayload) (any, error) {
	if _, err := db.runner(ctx).ExecContext(ctx, clearSMSMemoryFullStmt, p.UpdatedAt, p.Imsi); err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyRemoveSMSWaitingCentre(ctx context.Context, p *smsWaitingCentrePayload) (any, error) {
	if _, err := db.runner(ctx).ExecContext(ctx, deleteSMSWaitingCentreStmt, p.Imsi, p.ServiceCentre); err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	if _, err := db.runner(ctx).ExecContext(ctx, clearSMSMemoryFullStmt, p.UpdatedAt, p.Imsi); err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	if _, err := db.runner(ctx).ExecContext(ctx, deleteSMSWaitingIfEmptyStmt, p.Imsi, p.Imsi); err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyDeleteSMSWaiting(ctx context.Context, p *smsWaitingCentrePayload) (any, error) {
	if _, err := db.runner(ctx).ExecContext(ctx, deleteSMSWaitingCentresStmt, p.Imsi); err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	if _, err := db.runner(ctx).ExecContext(ctx, deleteSMSWaitingStmt, p.Imsi); err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyDeleteStaleSMSWaiting(ctx context.Context, p *smsWaitingCutoffPayload) (any, error) {
	if _, err := db.runner(ctx).ExecContext(ctx, deleteStaleSMSWaitingCentStmt, p.Cutoff); err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	if _, err := db.runner(ctx).ExecContext(ctx, deleteStaleSMSWaitingStmt, p.Cutoff); err != nil {
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

func scanSMSWaitingRow(row scanner) (smsWaitingRow, error) {
	var v smsWaitingRow

	if err := row.Scan(&v.Imsi, &v.MCEF, &v.UpdatedAt); err != nil {
		return smsWaitingRow{}, err
	}

	return v, nil
}

func scanSMSWaitingCentre(row scanner) (smsWaitingCentre, error) {
	var v smsWaitingCentre

	if err := row.Scan(&v.Imsi, &v.ServiceCentre); err != nil {
		return smsWaitingCentre{}, err
	}

	return v, nil
}
