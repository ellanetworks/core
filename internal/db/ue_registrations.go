// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
)

const UERegistrationsTableName = "ue_registrations"

const ueRegistrationsSchema = 21

const (
	UERegistrationTypeMME           = "mme"
	UERegistrationTypeAMF3GPPAccess = "amf-3gpp-access"
)

const (
	upsertUERegistrationStmt         = "INSERT INTO " + UERegistrationsTableName + " (imsi, type, nodeID, purged, version) VALUES (?, ?, ?, 0, ?) ON CONFLICT(imsi, type) DO UPDATE SET nodeID=excluded.nodeID, purged=0, version=excluded.version"
	purgeUERegistrationStmt          = "UPDATE " + UERegistrationsTableName + " SET purged=1, version=? WHERE imsi==? AND type==? AND nodeID==? AND purged==0"
	cancelUERegistrationStmt         = "UPDATE " + UERegistrationsTableName + " SET purged=1, version=? WHERE imsi==? AND type==? AND purged==0"
	purgeUERegistrationsByNodeStmt   = "UPDATE " + UERegistrationsTableName + " SET purged=1, version=? WHERE nodeID==? AND purged==0"
	getUERegistrationStmt            = "SELECT imsi, type, nodeID, purged, version FROM " + UERegistrationsTableName + " WHERE imsi==? AND type==?"
	listAllUERegistrationsStmt       = "SELECT imsi, type, nodeID, purged, version FROM " + UERegistrationsTableName + " ORDER BY imsi ASC"
	maxUERegistrationVersionStmt     = "SELECT COALESCE(MAX(version), 0) FROM " + UERegistrationsTableName
	resetUERegistrationsSQL          = "UPDATE %s SET purged=0, version=0"
	ueRegistrationsTableExistsSQLFmt = "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='%s'"
)

type UERegistration struct {
	Imsi    string
	Type    string
	NodeID  string
	Purged  bool
	Version int64
}

type registerUEPayload struct {
	Imsi   string `json:"imsi"`
	Type   string `json:"type"`
	NodeID string `json:"node_id"`
	Cancel string `json:"cancel,omitempty"`
}

type purgeUERegistrationPayload struct {
	Imsi   string `json:"imsi"`
	Type   string `json:"type"`
	NodeID string `json:"node_id"`
}

func IsValidUERegistrationType(t string) bool {
	switch t {
	case UERegistrationTypeMME, UERegistrationTypeAMF3GPPAccess:
		return true
	}

	return false
}

func validateRegisterUE(regType, nodeID, cancel string) error {
	if !IsValidUERegistrationType(regType) {
		return fmt.Errorf("invalid UE registration type %q", regType)
	}

	if cancel != "" && (!IsValidUERegistrationType(cancel) || cancel == regType) {
		return fmt.Errorf("invalid UE registration type to cancel %q", cancel)
	}

	if nodeID == "" {
		return fmt.Errorf("node ID is required")
	}

	return nil
}

func (db *Database) RegisterUE(ctx context.Context, imsi, regType, nodeID, cancel string) (int64, error) {
	if err := validateRegisterUE(regType, nodeID, cancel); err != nil {
		return 0, err
	}

	ctx, span := startUERegistrationsSpan(ctx, "UPSERT")
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(UERegistrationsTableName, "upsert"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(UERegistrationsTableName, "upsert").Inc()

	version, err := opRegisterUE.Invoke(ctx, db, &registerUEPayload{
		Imsi:   imsi,
		Type:   regType,
		NodeID: nodeID,
		Cancel: cancel,
	})
	if err != nil {
		recordSpanError(span, err)

		return 0, err
	}

	return version, nil
}

func (db *Database) PurgeUERegistration(ctx context.Context, imsi, regType, nodeID string) error {
	if !IsValidUERegistrationType(regType) {
		return fmt.Errorf("invalid UE registration type %q", regType)
	}

	ctx, span := startUERegistrationsSpan(ctx, "UPDATE")
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(UERegistrationsTableName, "update"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(UERegistrationsTableName, "update").Inc()

	_, err := opPurgeUERegistration.Invoke(ctx, db, &purgeUERegistrationPayload{
		Imsi:   imsi,
		Type:   regType,
		NodeID: nodeID,
	})
	if err != nil {
		recordSpanError(span, err)

		return err
	}

	return nil
}

func (db *Database) GetUERegistration(ctx context.Context, imsi, regType string) (*UERegistration, error) {
	ctx, span := startUERegistrationsSpan(ctx, "SELECT")
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(UERegistrationsTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(UERegistrationsTableName, "select").Inc()

	row, err := scanUERegistration(db.reader().QueryRowContext(ctx, getUERegistrationStmt, imsi, regType))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return &row, nil
}

func (db *Database) ListAllUERegistrations(ctx context.Context) ([]UERegistration, error) {
	ctx, span := startUERegistrationsSpan(ctx, "SELECT")
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(UERegistrationsTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(UERegistrationsTableName, "select").Inc()

	if !db.appliedSchemaAtLeast(ctx, ueRegistrationsSchema) {
		return nil, nil
	}

	regs, err := queryAll(ctx, db.reader(), listAllUERegistrationsStmt, scanUERegistration)
	if err != nil {
		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return regs, nil
}

func startUERegistrationsSpan(ctx context.Context, operation string) (context.Context, trace.Span) {
	querySummary := fmt.Sprintf("%s %s", operation, UERegistrationsTableName)

	return tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName(operation),
			semconv.DBCollectionName(UERegistrationsTableName),
		),
	)
}

func (db *Database) nextUERegistrationVersion(ctx context.Context) (int64, error) {
	var version int64

	if err := db.runner(ctx).QueryRowContext(ctx, maxUERegistrationVersionStmt).Scan(&version); err != nil {
		return 0, fmt.Errorf("query failed: %w", err)
	}

	next := version + 1

	if db.raftManager != nil {
		next = max(next, int64(db.raftManager.AppliedIndex())+1)
	}

	return next, nil
}

func (db *Database) applyRegisterUE(ctx context.Context, p *registerUEPayload) (any, error) {
	known, err := db.imsiKnown(ctx, p.Imsi)
	if err != nil {
		return nil, err
	}

	if !known {
		return nil, ErrNotFound
	}

	existing, err := scanUERegistration(db.runner(ctx).QueryRowContext(ctx, getUERegistrationStmt, p.Imsi, p.Type))
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	unchanged := err == nil && existing.NodeID == p.NodeID && !existing.Purged && existing.Version != 0

	next, err := db.nextUERegistrationVersion(ctx)
	if err != nil {
		return nil, err
	}

	version := existing.Version

	if !unchanged {
		version = next

		row := UERegistration{
			Imsi:    p.Imsi,
			Type:    p.Type,
			NodeID:  p.NodeID,
			Version: next,
		}

		if _, err := db.runner(ctx).ExecContext(ctx, upsertUERegistrationStmt, row.Imsi, row.Type, row.NodeID, row.Version); err != nil {
			if isForeignKeyError(err) {
				return nil, ErrNotFound
			}

			return nil, fmt.Errorf("query failed: %w", err)
		}
	}

	if p.Cancel != "" {
		if _, err := db.runner(ctx).ExecContext(ctx, cancelUERegistrationStmt, next, p.Imsi, p.Cancel); err != nil {
			return nil, fmt.Errorf("query failed: %w", err)
		}
	}

	return version, nil
}

func (db *Database) applyPurgeUERegistration(ctx context.Context, p *purgeUERegistrationPayload) (any, error) {
	next, err := db.nextUERegistrationVersion(ctx)
	if err != nil {
		return nil, err
	}

	row := UERegistration{Imsi: p.Imsi, Type: p.Type, NodeID: p.NodeID, Version: next}

	if _, err := db.runner(ctx).ExecContext(ctx, purgeUERegistrationStmt, row.Version, row.Imsi, row.Type, row.NodeID); err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	if !db.appliedSchemaAtLeast(ctx, roamingSchema) {
		return nil, nil
	}

	return nil, db.deleteVisitorIfUnregistered(ctx, p.Imsi)
}

func (db *Database) purgeUERegistrationsByNode(ctx context.Context, nodeID string) error {
	if !db.appliedSchemaAtLeast(ctx, ueRegistrationsSchema) {
		return nil
	}

	next, err := db.nextUERegistrationVersion(ctx)
	if err != nil {
		return err
	}

	if _, err := db.runner(ctx).ExecContext(ctx, purgeUERegistrationsByNodeStmt, next, nodeID); err != nil {
		return fmt.Errorf("purge UE registrations of node %s: %w", nodeID, err)
	}

	return nil
}

func resetUERegistrationsInRestoredDB(ctx context.Context, dbPath string) error {
	conn, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("open restored db: %w", err)
	}

	defer func() { _ = conn.Close() }()

	var count int
	if err := conn.QueryRowContext(ctx, fmt.Sprintf(ueRegistrationsTableExistsSQLFmt, UERegistrationsTableName)).Scan(&count); err != nil {
		return fmt.Errorf("look up %s: %w", UERegistrationsTableName, err)
	}

	if count == 0 {
		return nil
	}

	if _, err := conn.ExecContext(ctx, fmt.Sprintf(resetUERegistrationsSQL, UERegistrationsTableName)); err != nil {
		return fmt.Errorf("reset %s: %w", UERegistrationsTableName, err)
	}

	return nil
}

func scanUERegistration(row scanner) (UERegistration, error) {
	var v UERegistration

	if err := row.Scan(&v.Imsi, &v.Type, &v.NodeID, &v.Purged, &v.Version); err != nil {
		return UERegistration{}, err
	}

	return v, nil
}
