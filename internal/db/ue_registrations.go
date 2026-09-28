// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

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
	upsertUERegistrationStmt         = "INSERT INTO %[1]s (imsi, type, nodeID, purged, registrationTime) VALUES ($UERegistration.imsi, $UERegistration.type, $UERegistration.nodeID, 0, $UERegistration.registrationTime) ON CONFLICT(imsi, type) DO UPDATE SET nodeID=excluded.nodeID, purged=0, registrationTime=excluded.registrationTime WHERE %[1]s.nodeID!=excluded.nodeID OR %[1]s.purged!=0"
	purgeUERegistrationStmt          = "UPDATE %s SET purged=1 WHERE imsi==$UERegistration.imsi AND type==$UERegistration.type AND nodeID==$UERegistration.nodeID AND purged==0"
	cancelUERegistrationStmt         = "UPDATE %s SET purged=1 WHERE imsi==$UERegistration.imsi AND type==$UERegistration.type AND purged==0"
	purgeUERegistrationsByNodeStmt   = "UPDATE %s SET purged=1 WHERE nodeID==$UERegistration.nodeID AND purged==0"
	getUERegistrationStmt            = "SELECT &UERegistration.* FROM %s WHERE imsi==$UERegistration.imsi AND type==$UERegistration.type"
	resetUERegistrationsPurgedSQL    = "UPDATE %s SET purged=0 WHERE purged!=0"
	ueRegistrationsTableExistsSQLFmt = "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='%s'"
)

type UERegistration struct {
	Imsi             string `db:"imsi"`
	Type             string `db:"type"`
	NodeID           string `db:"nodeID"`
	Purged           bool   `db:"purged"`
	RegistrationTime int64  `db:"registrationTime"`
}

type registerUEPayload struct {
	Imsi             string   `json:"imsi"`
	Type             string   `json:"type"`
	NodeID           string   `json:"node_id"`
	RegistrationTime int64    `json:"registration_time"`
	Cancel           []string `json:"cancel,omitempty"`
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

func (db *Database) RegisterUE(ctx context.Context, imsi, regType, nodeID string, cancel ...string) error {
	if !IsValidUERegistrationType(regType) {
		return fmt.Errorf("invalid UE registration type %q", regType)
	}

	for _, c := range cancel {
		if !IsValidUERegistrationType(c) || c == regType {
			return fmt.Errorf("invalid UE registration type to cancel %q", c)
		}
	}

	if nodeID == "" {
		return fmt.Errorf("node ID is required")
	}

	ctx, span := startUERegistrationsSpan(ctx, "UPSERT")
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(UERegistrationsTableName, "upsert"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(UERegistrationsTableName, "upsert").Inc()

	_, err := opRegisterUE.Invoke(ctx, db, &registerUEPayload{
		Imsi:             imsi,
		Type:             regType,
		NodeID:           nodeID,
		RegistrationTime: time.Now().Unix(),
		Cancel:           cancel,
	})
	if err != nil {
		recordSpanError(span, err)

		return err
	}

	return nil
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

func (db *Database) applyRegisterUE(ctx context.Context, p *registerUEPayload) (any, error) {
	row := UERegistration{
		Imsi:             p.Imsi,
		Type:             p.Type,
		NodeID:           p.NodeID,
		RegistrationTime: p.RegistrationTime,
	}

	if err := db.runner(ctx).Query(ctx, db.upsertUERegistrationStmt, row).Run(); err != nil {
		if isForeignKeyError(err) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	for _, t := range p.Cancel {
		if err := db.runner(ctx).Query(ctx, db.cancelUERegistrationStmt, UERegistration{Imsi: p.Imsi, Type: t}).Run(); err != nil {
			return nil, fmt.Errorf("query failed: %w", err)
		}
	}

	return nil, nil
}

func (db *Database) applyPurgeUERegistration(ctx context.Context, p *purgeUERegistrationPayload) (any, error) {
	row := UERegistration{Imsi: p.Imsi, Type: p.Type, NodeID: p.NodeID}

	if err := db.runner(ctx).Query(ctx, db.purgeUERegistrationStmt, row).Run(); err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) purgeUERegistrationsByNode(ctx context.Context, nodeID string) error {
	if !db.appliedSchemaAtLeast(ctx, ueRegistrationsSchema) {
		return nil
	}

	if err := db.runner(ctx).Query(ctx, db.purgeUERegistrationsByNodeStmt, UERegistration{NodeID: nodeID}).Run(); err != nil {
		return fmt.Errorf("purge UE registrations of node %s: %w", nodeID, err)
	}

	return nil
}

func resetUERegistrationsPurgedInRestoredDB(ctx context.Context, dbPath string) error {
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

	if _, err := conn.ExecContext(ctx, fmt.Sprintf(resetUERegistrationsPurgedSQL, UERegistrationsTableName)); err != nil {
		return fmt.Errorf("reset %s purged flags: %w", UERegistrationsTableName, err)
	}

	return nil
}
