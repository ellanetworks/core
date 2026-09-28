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
	upsertUERegistrationStmt         = "INSERT INTO %s (imsi, type, nodeID, purged, version) VALUES ($UERegistration.imsi, $UERegistration.type, $UERegistration.nodeID, 0, $UERegistration.version) ON CONFLICT(imsi, type) DO UPDATE SET nodeID=excluded.nodeID, purged=0, version=excluded.version"
	purgeUERegistrationStmt          = "UPDATE %s SET purged=1, version=$UERegistration.version WHERE imsi==$UERegistration.imsi AND type==$UERegistration.type AND nodeID==$UERegistration.nodeID AND purged==0"
	cancelUERegistrationStmt         = "UPDATE %s SET purged=1, version=$UERegistration.version WHERE imsi==$UERegistration.imsi AND type==$UERegistration.type AND purged==0"
	purgeUERegistrationsByNodeStmt   = "UPDATE %s SET purged=1, version=$UERegistration.version WHERE nodeID==$UERegistration.nodeID AND purged==0"
	getUERegistrationStmt            = "SELECT &UERegistration.* FROM %s WHERE imsi==$UERegistration.imsi AND type==$UERegistration.type"
	maxUERegistrationVersionStmt     = "SELECT COALESCE(MAX(version), 0) AS &ueRegistrationVersion.version FROM %s"
	resetUERegistrationsSQL          = "UPDATE %s SET purged=0, version=0"
	ueRegistrationsTableExistsSQLFmt = "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='%s'"
)

type UERegistration struct {
	Imsi    string `db:"imsi"`
	Type    string `db:"type"`
	NodeID  string `db:"nodeID"`
	Purged  bool   `db:"purged"`
	Version int64  `db:"version"`
}

type ueRegistrationVersion struct {
	Version int64 `db:"version"`
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

func (db *Database) RegisterUE(ctx context.Context, imsi, regType, nodeID, cancel string) (int64, error) {
	if !IsValidUERegistrationType(regType) {
		return 0, fmt.Errorf("invalid UE registration type %q", regType)
	}

	if cancel != "" && (!IsValidUERegistrationType(cancel) || cancel == regType) {
		return 0, fmt.Errorf("invalid UE registration type to cancel %q", cancel)
	}

	if nodeID == "" {
		return 0, fmt.Errorf("node ID is required")
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

	row := UERegistration{Imsi: imsi, Type: regType}

	err := db.conn().Query(ctx, db.getUERegistrationStmt, row).Get(&row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return &row, nil
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
	if db.raftManager != nil {
		return int64(db.raftManager.AppliedIndex()) + 1, nil
	}

	var v ueRegistrationVersion

	if err := db.runner(ctx).Query(ctx, db.maxUERegistrationVersionStmt).Get(&v); err != nil {
		return 0, fmt.Errorf("query failed: %w", err)
	}

	return v.Version + 1, nil
}

func (db *Database) applyRegisterUE(ctx context.Context, p *registerUEPayload) (any, error) {
	existing := UERegistration{Imsi: p.Imsi, Type: p.Type}

	err := db.runner(ctx).Query(ctx, db.getUERegistrationStmt, existing).Get(&existing)
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

		if err := db.runner(ctx).Query(ctx, db.upsertUERegistrationStmt, row).Run(); err != nil {
			if isForeignKeyError(err) {
				return nil, ErrNotFound
			}

			return nil, fmt.Errorf("query failed: %w", err)
		}
	}

	if p.Cancel != "" {
		if err := db.runner(ctx).Query(ctx, db.cancelUERegistrationStmt, UERegistration{Imsi: p.Imsi, Type: p.Cancel, Version: next}).Run(); err != nil {
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

	if err := db.runner(ctx).Query(ctx, db.purgeUERegistrationStmt, row).Run(); err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) purgeUERegistrationsByNode(ctx context.Context, nodeID string) error {
	if !db.appliedSchemaAtLeast(ctx, ueRegistrationsSchema) {
		return nil
	}

	next, err := db.nextUERegistrationVersion(ctx)
	if err != nil {
		return err
	}

	if err := db.runner(ctx).Query(ctx, db.purgeUERegistrationsByNodeStmt, UERegistration{NodeID: nodeID, Version: next}).Run(); err != nil {
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
