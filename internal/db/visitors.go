// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/canonical/sqlair"
	"github.com/prometheus/client_golang/prometheus"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
)

const VisitorsTableName = "visitors"

const (
	visitorColumns                  = "v.imsi AS &Visitor.imsi, v.partnerID AS &Visitor.partnerID, v.msisdn AS &Visitor.msisdn, v.since AS &Visitor.since, p.profileID AS &Visitor.profileID FROM %s AS v JOIN %s AS p ON p.id = v.partnerID"
	getVisitorStmt                  = "SELECT " + visitorColumns + " WHERE v.imsi = $Visitor.imsi"
	listVisitorsStmt                = "SELECT " + visitorColumns + " ORDER BY v.imsi"
	insertVisitorStmt               = "INSERT INTO %s (imsi, partnerID, msisdn, since) VALUES ($Visitor.imsi, $Visitor.partnerID, $Visitor.msisdn, $Visitor.since)"
	updateVisitorMSISDNStmt         = "UPDATE %s SET msisdn=$Visitor.msisdn WHERE imsi==$Visitor.imsi"
	deleteVisitorIfUnregisteredStmt = "DELETE FROM %s WHERE imsi==$Visitor.imsi AND NOT EXISTS (SELECT 1 FROM %s AS r WHERE r.imsi = %s.imsi AND r.purged = 0)"
	listUnregisteredVisitorsStmt    = "SELECT v.imsi AS &Visitor.imsi FROM %s AS v WHERE NOT EXISTS (SELECT 1 FROM %s AS r WHERE r.imsi = v.imsi AND r.purged = 0) ORDER BY v.imsi"
	deleteUERegistrationsOfIMSIStmt = "DELETE FROM %s WHERE imsi==$subscriberIMSI.imsi"
	deleteLeasesOfIMSIStmt          = "DELETE FROM %s WHERE imsi==$subscriberIMSI.imsi"
)

type Visitor struct {
	Imsi      string `db:"imsi"`
	PartnerID string `db:"partnerID"`
	ProfileID string `db:"profileID"`
	Msisdn    string `db:"msisdn"`
	Since     int64  `db:"since"`
}

func (v Visitor) subscriber() Subscriber {
	partnerID := v.PartnerID

	return Subscriber{Imsi: v.Imsi, ProfileID: v.ProfileID, Msisdn: v.Msisdn, RoamingPartnerID: &partnerID}
}

type registerVisitorUEPayload struct {
	Imsi      string `json:"imsi"`
	PartnerID string `json:"partner_id"`
	Type      string `json:"type"`
	NodeID    string `json:"node_id"`
	Cancel    string `json:"cancel,omitempty"`
}

type visitorMSISDNPayload struct {
	Imsi   string `json:"imsi"`
	Msisdn string `json:"msisdn"`
}

var opSetVisitorMSISDN = registerChangesetOp("SetVisitorMSISDN", (*Database).applySetVisitorMSISDN, RequireSchema(26))

func (db *Database) GetSubscriberOrVisitor(ctx context.Context, imsi string) (*Subscriber, error) {
	ctx, span := startVisitorsSpan(ctx, "SELECT", "subscriber or visitor")
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(VisitorsTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(VisitorsTableName, "select").Inc()

	row := Subscriber{Imsi: imsi}

	err := db.conn().Query(ctx, db.ownSubscriberStmt(ctx), row).Get(&row)
	if err == nil {
		return &row, nil
	}

	if !errors.Is(err, sql.ErrNoRows) {
		recordSpanError(span, err)
		return nil, fmt.Errorf("query failed: %w", err)
	}

	if !db.appliedSchemaAtLeast(ctx, roamingSchema) {
		return nil, ErrNotFound
	}

	visitor, err := db.getVisitor(ctx, db.conn(), imsi)
	if err != nil {
		if !errors.Is(err, ErrNotFound) {
			recordSpanError(span, err)
		}

		return nil, err
	}

	sub := visitor.subscriber()

	return &sub, nil
}

func (db *Database) GetVisitor(ctx context.Context, imsi string) (*Visitor, error) {
	ctx, span := startVisitorsSpan(ctx, "SELECT", "visitor")
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(VisitorsTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(VisitorsTableName, "select").Inc()

	if !db.appliedSchemaAtLeast(ctx, roamingSchema) {
		return nil, ErrNotFound
	}

	visitor, err := db.getVisitor(ctx, db.conn(), imsi)
	if err != nil && !errors.Is(err, ErrNotFound) {
		recordSpanError(span, err)
	}

	return visitor, err
}

func (db *Database) ListVisitors(ctx context.Context) ([]Subscriber, error) {
	ctx, span := startVisitorsSpan(ctx, "SELECT", "visitors")
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(VisitorsTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(VisitorsTableName, "select").Inc()

	if !db.appliedSchemaAtLeast(ctx, roamingSchema) {
		return nil, nil
	}

	var rows []Visitor

	if err := db.conn().Query(ctx, db.listVisitorsStmt).GetAll(&rows); err != nil && !errors.Is(err, sql.ErrNoRows) {
		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	visitors := make([]Subscriber, 0, len(rows))
	for _, v := range rows {
		visitors = append(visitors, v.subscriber())
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

func (db *Database) SetVisitorMSISDN(ctx context.Context, imsi, msisdn string) error {
	if msisdn != "" && !isDigits(msisdn, 1, 15) {
		return fmt.Errorf("visitor MSISDN must be 1 to 15 digits, got %q", msisdn)
	}

	ctx, span := startVisitorsSpan(ctx, "UPDATE", "visitor msisdn")
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(VisitorsTableName, "update"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(VisitorsTableName, "update").Inc()

	if _, err := opSetVisitorMSISDN.Invoke(ctx, db, &visitorMSISDNPayload{Imsi: imsi, Msisdn: msisdn}); err != nil {
		recordSpanError(span, err)
		return err
	}

	return nil
}

func (db *Database) DeleteVisitorIfUnregistered(ctx context.Context, imsi string) (bool, error) {
	ctx, span := startVisitorsSpan(ctx, "DELETE", "unregistered visitor")
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(VisitorsTableName, "delete"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(VisitorsTableName, "delete").Inc()

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

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(VisitorsTableName, "delete"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(VisitorsTableName, "delete").Inc()

	deleted, err := opDeleteUnregisteredVisitors.Invoke(ctx, db, &emptyPayload{})
	if err != nil {
		recordSpanError(span, err)

		return 0, err
	}

	return deleted, nil
}

func (db *Database) getVisitor(ctx context.Context, runner *sqlair.DB, imsi string) (*Visitor, error) {
	row := Visitor{Imsi: imsi}

	if err := runner.Query(ctx, db.getVisitorStmt, row).Get(&row); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return &row, nil
}

func (db *Database) imsiKnown(ctx context.Context, imsi string) (bool, error) {
	runner := db.runner(ctx)

	err := runner.Query(ctx, db.ownSubscriberStmt(ctx), Subscriber{Imsi: imsi}).Get(&Subscriber{})
	if err == nil {
		return true, nil
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return false, fmt.Errorf("query failed: %w", err)
	}

	if !db.appliedSchemaAtLeast(ctx, roamingSchema) {
		return false, nil
	}

	if _, err := db.getVisitor(ctx, runner, imsi); err != nil {
		if errors.Is(err, ErrNotFound) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

func (db *Database) applyRegisterVisitorUE(ctx context.Context, p *registerVisitorUEPayload) (any, error) {
	if err := db.refuseWithdrawnVisitor(ctx, p.Imsi, p.PartnerID, p.Type); err != nil {
		return nil, err
	}

	if err := db.insertVisitorIfAbsent(ctx, p.Imsi, p.PartnerID); err != nil {
		return nil, err
	}

	return db.applyRegisterUE(ctx, &registerUEPayload{Imsi: p.Imsi, Type: p.Type, NodeID: p.NodeID, Cancel: p.Cancel})
}

func (db *Database) insertVisitorIfAbsent(ctx context.Context, imsi, partnerID string) error {
	runner := db.runner(ctx)

	existing, err := db.getVisitor(ctx, runner, imsi)

	switch {
	case err == nil && existing.PartnerID == partnerID:
		return nil
	case err == nil:
		return fmt.Errorf("%w: IMSI %s is a visitor of another roaming partner", ErrAlreadyExists, imsi)
	case !errors.Is(err, ErrNotFound):
		return err
	}

	err = runner.Query(ctx, db.getSubscriberStmt, Subscriber{Imsi: imsi}).Get(&Subscriber{})

	switch {
	case err == nil:
		return fmt.Errorf("%w: IMSI %s is a home subscriber", ErrAlreadyExists, imsi)
	case !errors.Is(err, sql.ErrNoRows):
		return fmt.Errorf("query failed: %w", err)
	}

	partner, err := db.getRoamingPartner(ctx, runner, partnerID)
	if err != nil {
		return err
	}

	if _, ok := partner.PLMNOfIMSI(imsi); !ok {
		return fmt.Errorf("%w: IMSI %s is not in a PLMN of roaming partner %s", ErrRoamingConflict, imsi, partner.Name)
	}

	if err := db.checkSubscriberCapacity(ctx); err != nil {
		return err
	}

	visitor := Visitor{Imsi: imsi, PartnerID: partner.ID, Since: time.Now().Unix()}

	if err := runner.Query(ctx, db.insertVisitorStmt, visitor).Run(); err != nil {
		return fmt.Errorf("insert visitor: %w", err)
	}

	return nil
}

func (db *Database) applySetVisitorMSISDN(ctx context.Context, p *visitorMSISDNPayload) (any, error) {
	var outcome sqlair.Outcome

	if err := db.runner(ctx).Query(ctx, db.updateVisitorMSISDNStmt, Visitor{Imsi: p.Imsi, Msisdn: p.Msisdn}).Get(&outcome); err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	n, err := outcome.Result().RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("rows affected: %w", err)
	}

	if n == 0 {
		return nil, ErrNotFound
	}

	return nil, nil
}

func (db *Database) applyDeleteVisitorIfUnregistered(ctx context.Context, p *stringPayload) (any, error) {
	return db.deleteVisitorIfUnregistered(ctx, p.Value)
}

func (db *Database) applyDeleteUnregisteredVisitors(ctx context.Context, _ *emptyPayload) (any, error) {
	var rows []Visitor

	if err := db.runner(ctx).Query(ctx, db.listUnregisteredVisitorsStmt).GetAll(&rows); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	var deleted int64

	for _, v := range rows {
		n, err := db.deleteVisitorIfUnregistered(ctx, v.Imsi)
		if err != nil {
			return nil, err
		}

		deleted += n
	}

	return deleted, nil
}

func (db *Database) deleteVisitorIfUnregistered(ctx context.Context, imsi string) (int64, error) {
	runner := db.runner(ctx)

	var outcome sqlair.Outcome

	if err := runner.Query(ctx, db.deleteVisitorIfUnregisteredStmt, Visitor{Imsi: imsi}).Get(&outcome); err != nil {
		return 0, fmt.Errorf("query failed: %w", err)
	}

	deleted, err := outcome.Result().RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("rows affected: %w", err)
	}

	if deleted == 0 {
		return 0, nil
	}

	if err := db.deleteIMSIState(ctx, imsi); err != nil {
		return 0, err
	}

	return deleted, nil
}

func (db *Database) deleteIMSIState(ctx context.Context, imsi string) error {
	runner := db.runner(ctx)
	key := subscriberIMSI{Imsi: imsi}

	if err := runner.Query(ctx, db.deleteUERegistrationsOfIMSIStmt, key).Run(); err != nil {
		return fmt.Errorf("delete UE registrations: %w", err)
	}

	if err := runner.Query(ctx, db.deleteLeasesOfIMSIStmt, key).Run(); err != nil {
		return fmt.Errorf("delete IP leases: %w", err)
	}

	return nil
}

func startVisitorsSpan(ctx context.Context, operation, detail string) (context.Context, trace.Span) {
	querySummary := fmt.Sprintf("%s %s (%s)", operation, VisitorsTableName, detail)

	return tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName(operation),
			semconv.DBCollectionName(VisitorsTableName),
		),
	)
}
