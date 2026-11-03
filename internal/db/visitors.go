// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/canonical/sqlair"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
)

const (
	getSubscriberOrVisitorStmt      = "SELECT &Subscriber.* FROM %s WHERE imsi==$Subscriber.imsi"
	listVisitorsStmt                = "SELECT &Subscriber.* FROM %s WHERE roamingPartnerID IS NOT NULL ORDER BY imsi"
	insertVisitorStmt               = "INSERT INTO %s (id, imsi, sequenceNumber, permanentKey, opc, profileID, description, msisdn, roamingPartnerID) VALUES ($Subscriber.id, $Subscriber.imsi, '', '', '', $Subscriber.profileID, '', '', $Subscriber.roamingPartnerID)"
	unregisteredVisitorClause       = "roamingPartnerID IS NOT NULL AND NOT EXISTS (SELECT 1 FROM %s AS r WHERE r.imsi = %s.imsi AND r.purged = 0)"
	deleteVisitorIfUnregisteredStmt = "DELETE FROM %s WHERE imsi==$Subscriber.imsi AND " + unregisteredVisitorClause
	deleteUnregisteredVisitorsStmt  = "DELETE FROM %s WHERE " + unregisteredVisitorClause
)

type registerVisitorUEPayload struct {
	Imsi      string `json:"imsi"`
	PartnerID string `json:"partner_id"`
	Type      string `json:"type"`
	NodeID    string `json:"node_id"`
	Cancel    string `json:"cancel,omitempty"`
}

func (db *Database) GetSubscriberOrVisitor(ctx context.Context, imsi string) (*Subscriber, error) {
	ctx, span := startVisitorsSpan(ctx, "SELECT", "subscriber or visitor")
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(SubscribersTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(SubscribersTableName, "select").Inc()

	stmt := db.getSubscriberOrVisitorStmt
	if !db.appliedSchemaAtLeast(ctx, roamingSchema) {
		stmt = db.homeSubscriberStmt(ctx)
	}

	row := Subscriber{Imsi: imsi}

	if err := db.conn().Query(ctx, stmt, row).Get(&row); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return &row, nil
}

func (db *Database) ListVisitors(ctx context.Context) ([]Subscriber, error) {
	ctx, span := startVisitorsSpan(ctx, "SELECT", "visitors")
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(SubscribersTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(SubscribersTableName, "select").Inc()

	if !db.appliedSchemaAtLeast(ctx, roamingSchema) {
		return nil, nil
	}

	var visitors []Subscriber

	if err := db.conn().Query(ctx, db.listVisitorsStmt).GetAll(&visitors); err != nil && !errors.Is(err, sql.ErrNoRows) {
		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return visitors, nil
}

func (db *Database) RegisterVisitorUE(ctx context.Context, imsi, partnerID, regType, nodeID, cancel string) (int64, error) {
	if err := validateRegisterUE(regType, nodeID, cancel); err != nil {
		return 0, err
	}

	if partnerID == "" {
		return 0, fmt.Errorf("roaming partner ID is required")
	}

	ctx, span := startVisitorsSpan(ctx, "UPSERT", "register visitor")
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(UERegistrationsTableName, "upsert"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(UERegistrationsTableName, "upsert").Inc()

	version, err := opRegisterVisitorUE.Invoke(ctx, db, &registerVisitorUEPayload{
		Imsi:      imsi,
		PartnerID: partnerID,
		Type:      regType,
		NodeID:    nodeID,
		Cancel:    cancel,
	})
	if err != nil {
		recordSpanError(span, err)

		return 0, err
	}

	return version, nil
}

func (db *Database) DeleteVisitorIfUnregistered(ctx context.Context, imsi string) (bool, error) {
	ctx, span := startVisitorsSpan(ctx, "DELETE", "unregistered visitor")
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(SubscribersTableName, "delete"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(SubscribersTableName, "delete").Inc()

	deleted, err := opDeleteVisitorIfUnregistered.Invoke(ctx, db, &stringPayload{Value: imsi})
	if err != nil {
		recordSpanError(span, err)

		return false, err
	}

	return deleted > 0, nil
}

func (db *Database) DeleteUnregisteredVisitors(ctx context.Context) (int64, error) {
	ctx, span := startVisitorsSpan(ctx, "DELETE", "unregistered visitors")
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(SubscribersTableName, "delete"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(SubscribersTableName, "delete").Inc()

	deleted, err := opDeleteUnregisteredVisitors.Invoke(ctx, db, &emptyPayload{})
	if err != nil {
		recordSpanError(span, err)

		return 0, err
	}

	return deleted, nil
}

func (db *Database) applyRegisterVisitorUE(ctx context.Context, p *registerVisitorUEPayload) (any, error) {
	if err := db.insertVisitorIfAbsent(ctx, p.Imsi, p.PartnerID); err != nil {
		return nil, err
	}

	return db.applyRegisterUE(ctx, &registerUEPayload{Imsi: p.Imsi, Type: p.Type, NodeID: p.NodeID, Cancel: p.Cancel})
}

func (db *Database) insertVisitorIfAbsent(ctx context.Context, imsi, partnerID string) error {
	runner := db.runner(ctx)

	existing := Subscriber{Imsi: imsi}

	err := runner.Query(ctx, db.getSubscriberOrVisitorStmt, existing).Get(&existing)
	if err == nil {
		if existing.RoamingPartnerID != nil && *existing.RoamingPartnerID == partnerID {
			return nil
		}

		return fmt.Errorf("%w: IMSI %s is held by another subscriber", ErrAlreadyExists, imsi)
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("query failed: %w", err)
	}

	partner, err := db.getRoamingPartner(ctx, runner, partnerID)
	if err != nil {
		return err
	}

	if _, ok := partner.PLMNOfIMSI(imsi); !ok {
		return fmt.Errorf("%w: IMSI %s is not in a PLMN of roaming partner %s", ErrRoamingConflict, imsi, partner.Name)
	}

	id, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("generate visitor id: %w", err)
	}

	visitor := Subscriber{
		ID:               id.String(),
		Imsi:             imsi,
		ProfileID:        partner.ProfileID,
		RoamingPartnerID: &partner.ID,
	}

	if err := runner.Query(ctx, db.insertVisitorStmt, visitor).Run(); err != nil {
		return fmt.Errorf("insert visitor: %w", err)
	}

	return nil
}

func (db *Database) applyDeleteVisitorIfUnregistered(ctx context.Context, p *stringPayload) (any, error) {
	return db.deleteVisitors(ctx, db.deleteVisitorIfUnregisteredStmt, Subscriber{Imsi: p.Value})
}

func (db *Database) applyDeleteUnregisteredVisitors(ctx context.Context, _ *emptyPayload) (any, error) {
	return db.deleteVisitors(ctx, db.deleteUnregisteredVisitorsStmt)
}

func (db *Database) deleteVisitors(ctx context.Context, stmt *sqlair.Statement, args ...any) (any, error) {
	var outcome sqlair.Outcome

	if err := db.runner(ctx).Query(ctx, stmt, args...).Get(&outcome); err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	deleted, err := outcome.Result().RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("rows affected: %w", err)
	}

	return deleted, nil
}

func startVisitorsSpan(ctx context.Context, operation, detail string) (context.Context, trace.Span) {
	querySummary := fmt.Sprintf("%s %s (%s)", operation, SubscribersTableName, detail)

	return tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName(operation),
			semconv.DBCollectionName(SubscribersTableName),
		),
	)
}
