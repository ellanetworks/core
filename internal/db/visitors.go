// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

const VisitorsTableName = "visitors"

const (
	visitorColumns                    = "v.imsi, v.partnerID, v.msisdn, v.since, v.deregisterUntil, p.profileID, p.withdrawingUntil"
	visitorSource                     = " FROM " + VisitorsTableName + " AS v JOIN " + RoamingPartnersTableName + " AS p ON p.id = v.partnerID"
	getVisitorSQL                     = "SELECT " + visitorColumns + visitorSource + " WHERE v.imsi = ?"
	insertVisitorSQL                  = "INSERT INTO " + VisitorsTableName + " (imsi, partnerID, msisdn, since) VALUES (?, ?, ?, ?)"
	updateVisitorMSISDNSQL            = "UPDATE " + VisitorsTableName + " SET msisdn=? WHERE imsi==?"
	deleteVisitorIfUnregisteredSQL    = "DELETE FROM " + VisitorsTableName + " WHERE imsi==? AND NOT EXISTS (SELECT 1 FROM " + UERegistrationsTableName + " AS r WHERE r.imsi = " + VisitorsTableName + ".imsi AND r.purged = 0)"
	listUnregisteredVisitorsSQL       = "SELECT v.imsi FROM " + VisitorsTableName + " AS v WHERE NOT EXISTS (SELECT 1 FROM " + UERegistrationsTableName + " AS r WHERE r.imsi = v.imsi AND r.purged = 0) ORDER BY v.imsi"
	deleteUERegistrationsOfIMSISQL    = "DELETE FROM " + UERegistrationsTableName + " WHERE imsi==?"
	deleteLeasesOfIMSISQL             = "DELETE FROM " + IPLeasesTableName + " WHERE imsi==?"
	setVisitorDeregisteringSQL        = "UPDATE " + VisitorsTableName + " SET deregisterUntil=? WHERE imsi==?"
	listVisitorRegistrationsOfNodeSQL = "SELECT imsi, type, nodeID, purged, version FROM " + UERegistrationsTableName + " WHERE nodeID==? AND purged==0 AND imsi IN (SELECT imsi FROM " + VisitorsTableName + ") ORDER BY imsi, type"
)

const (
	visitorPageFilterClause = ` WHERE (?1 IS NULL OR v.partnerID = ?1)
    AND (?2 IS NULL
         OR v.imsi LIKE ?2 ESCAPE '\'
         OR (v.msisdn != '' AND '+' || v.msisdn LIKE ?2 ESCAPE '\'))
    AND (?3 IS NULL
         OR v.partnerID IN (SELECT id FROM ` + RoamingPartnersTableName + ` WHERE profileID IN (SELECT profileID FROM ` + PoliciesTableName + ` WHERE dataNetworkID = ?3)))`
	listVisitorsPageSQL  = "SELECT " + visitorColumns + ", COUNT(*) OVER()" + visitorSource + visitorPageFilterClause + " ORDER BY v.imsi LIMIT ?4 OFFSET ?5"
	countVisitorsPageSQL = "SELECT COUNT(*) FROM " + VisitorsTableName + " AS v" + visitorPageFilterClause
)

type Visitor struct {
	Imsi      string
	PartnerID string
	ProfileID string
	Msisdn    string
	Since     int64

	DeregisterUntil  int64
	WithdrawingUntil int64
}

func (v Visitor) Deregistering(now time.Time) bool {
	return now.Unix() < v.DeregisterUntil
}

func (v Visitor) Withdrawing(now time.Time) bool {
	return now.Unix() < v.WithdrawingUntil
}

func (v Visitor) refusal(now time.Time) error {
	switch {
	case v.Deregistering(now):
		return fmt.Errorf("%w: IMSI %s", ErrRoamerDeregistering, v.Imsi)
	case v.Withdrawing(now):
		return fmt.Errorf("%w: IMSI %s", ErrRoamingRefused, v.Imsi)
	}

	return nil
}

func (v Visitor) subscriber() Subscriber {
	partnerID := v.PartnerID

	return Subscriber{Imsi: v.Imsi, ProfileID: v.ProfileID, Msisdn: v.Msisdn, RoamingPartnerID: &partnerID}
}

type VisitorFilters struct {
	PartnerID     *string
	Search        *string
	DataNetworkID *string
}

type visitorPageFilterArgs struct {
	PartnerID     *string
	Search        *string
	DataNetworkID *string
}

func (f *VisitorFilters) args() visitorPageFilterArgs {
	if f == nil {
		return visitorPageFilterArgs{}
	}

	args := visitorPageFilterArgs{PartnerID: f.PartnerID, DataNetworkID: f.DataNetworkID}

	if f.Search != nil {
		pattern := "%" + likeEscaper.Replace(*f.Search) + "%"
		args.Search = &pattern
	}

	return args
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

var opSetVisitorMSISDN = registerChangesetOp("SetVisitorMSISDN", (*Database).applySetVisitorMSISDN, RequireSchema(roamingSchema))

func (db *Database) GetSubscriberOrVisitor(ctx context.Context, imsi string) (_ *Subscriber, err error) {
	ctx, finish := startDBQueryNamed(ctx, "SELECT "+VisitorsTableName+" (subscriber or visitor)", VisitorsTableName, "SELECT")
	defer func() { finish(err) }()

	row, err := db.getOwnSubscriber(ctx, db.reader(), imsi)
	if err == nil {
		return &row, nil
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	if !db.appliedSchemaAtLeast(ctx, roamingSchema) {
		return nil, ErrNotFound
	}

	visitor, err := db.getVisitor(ctx, db.reader(), imsi)
	if err != nil {
		return nil, err
	}

	sub := visitor.subscriber()

	return &sub, nil
}

func (db *Database) GetVisitor(ctx context.Context, imsi string) (_ *Visitor, err error) {
	ctx, finish := startDBQueryNamed(ctx, "SELECT "+VisitorsTableName+" (visitor)", VisitorsTableName, "SELECT")
	defer func() { finish(err) }()

	if !db.appliedSchemaAtLeast(ctx, roamingSchema) {
		return nil, ErrNotFound
	}

	return db.getVisitor(ctx, db.reader(), imsi)
}

func (db *Database) ListVisitorsPage(ctx context.Context, filters *VisitorFilters, page int, perPage int) (_ []Visitor, _ int, err error) {
	ctx, finish := startDBQueryNamed(ctx, "SELECT "+VisitorsTableName+" (paged)", VisitorsTableName, "SELECT")
	defer func() { finish(err) }()

	if !db.appliedSchemaAtLeast(ctx, roamingSchema) {
		return nil, 0, nil
	}

	filterArgs := filters.args()

	rows, count, err := queryPageWithTotal(ctx, db.reader(), listVisitorsPageSQL, scanVisitor, filterArgs.PartnerID, filterArgs.Search, filterArgs.DataNetworkID, perPage, (page-1)*perPage)
	if err != nil {
		return nil, 0, fmt.Errorf("query failed: %w", err)
	}

	if len(rows) == 0 {
		total, err := queryCount(ctx, db.reader(), countVisitorsPageSQL, filterArgs.PartnerID, filterArgs.Search, filterArgs.DataNetworkID)
		if err != nil {
			return nil, 0, fmt.Errorf("query failed: %w", err)
		}

		return nil, total, nil
	}

	return rows, count, nil
}

func (db *Database) RegisterVisitorUE(ctx context.Context, imsi, partnerID, regType, nodeID, cancel string) (_ int64, err error) {
	if err := validateRegisterUE(regType, nodeID, cancel); err != nil {
		return 0, err
	}

	if partnerID == "" {
		return 0, fmt.Errorf("roaming partner ID is required")
	}

	ctx, finish := startDBQueryNamed(ctx, "UPSERT "+VisitorsTableName+" (register visitor)", VisitorsTableName, "UPSERT")
	defer func() { finish(err) }()

	version, err := opRegisterVisitorUE.Invoke(ctx, db, &registerVisitorUEPayload{
		Imsi:      imsi,
		PartnerID: partnerID,
		Type:      regType,
		NodeID:    nodeID,
		Cancel:    cancel,
	})
	if err != nil {
		return 0, err
	}

	return version, nil
}

func (db *Database) SetVisitorMSISDN(ctx context.Context, imsi, msisdn string) (err error) {
	if msisdn != "" && !isDigits(msisdn, 1, 15) {
		return fmt.Errorf("visitor MSISDN must be 1 to 15 digits, got %q", msisdn)
	}

	ctx, finish := startDBQueryNamed(ctx, "UPDATE "+VisitorsTableName+" (visitor msisdn)", VisitorsTableName, "UPDATE")
	defer func() { finish(err) }()

	if _, err := opSetVisitorMSISDN.Invoke(ctx, db, &visitorMSISDNPayload{Imsi: imsi, Msisdn: msisdn}); err != nil {
		return err
	}

	return nil
}

func (db *Database) ListVisitorRegistrationsOfNode(ctx context.Context, nodeID string) ([]UERegistration, error) {
	if !db.appliedSchemaAtLeast(ctx, roamingSchema) {
		return nil, nil
	}

	regs, err := queryAll(ctx, db.reader(), listVisitorRegistrationsOfNodeSQL, scanUERegistration, nodeID)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return regs, nil
}

func (db *Database) getVisitor(ctx context.Context, q querier, imsi string) (*Visitor, error) {
	row, err := scanVisitor(q.QueryRowContext(ctx, getVisitorSQL, imsi))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return &row, nil
}

func (db *Database) imsiKnown(ctx context.Context, imsi string) (bool, error) {
	_, err := db.getOwnSubscriber(ctx, db.runner(ctx), imsi)
	if err == nil {
		return true, nil
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return false, fmt.Errorf("query failed: %w", err)
	}

	if !db.appliedSchemaAtLeast(ctx, roamingSchema) {
		return false, nil
	}

	if _, err := db.getVisitor(ctx, db.runner(ctx), imsi); err != nil {
		if errors.Is(err, ErrNotFound) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

func (db *Database) applyRegisterVisitorUE(ctx context.Context, p *registerVisitorUEPayload) (any, error) {
	partner, err := db.getRoamingPartner(ctx, db.runner(ctx), p.PartnerID)
	if err != nil {
		return nil, err
	}

	if !partner.hasAgreement(p.Type) {
		return nil, fmt.Errorf("%w: IMSI %s: roaming partner %s has no agreement for %s", ErrRoamingRefused, p.Imsi, partner.Name, p.Type)
	}

	if err := db.insertVisitorIfAbsent(ctx, p.Imsi, partner); err != nil {
		return nil, err
	}

	return db.applyRegisterUE(ctx, &registerUEPayload{Imsi: p.Imsi, Type: p.Type, NodeID: p.NodeID, Cancel: p.Cancel})
}

func (db *Database) insertVisitorIfAbsent(ctx context.Context, imsi string, partner *RoamingPartner) error {
	now := time.Now()

	existing, err := db.getVisitor(ctx, db.runner(ctx), imsi)

	switch {
	case err == nil && existing.PartnerID == partner.ID:
		return existing.refusal(now)
	case err == nil:
		return fmt.Errorf("%w: IMSI %s is a visitor of another roaming partner", ErrAlreadyExists, imsi)
	case !errors.Is(err, ErrNotFound):
		return err
	}

	_, err = scanSubscriber(db.runner(ctx).QueryRowContext(ctx, getSubscriberSQL, imsi))

	switch {
	case err == nil:
		return fmt.Errorf("%w: IMSI %s is a home subscriber", ErrAlreadyExists, imsi)
	case !errors.Is(err, sql.ErrNoRows):
		return fmt.Errorf("query failed: %w", err)
	}

	if partner.Withdrawing(now) {
		return fmt.Errorf("%w: IMSI %s", ErrRoamingRefused, imsi)
	}

	if _, ok := partner.PLMNOfIMSI(imsi); !ok {
		return fmt.Errorf("%w: IMSI %s is not in a PLMN of roaming partner %s", ErrRoamingConflict, imsi, partner.Name)
	}

	if err := db.checkSubscriberCapacity(ctx); err != nil {
		return err
	}

	visitor := Visitor{Imsi: imsi, PartnerID: partner.ID, Since: time.Now().Unix()}

	if _, err := db.runner(ctx).ExecContext(ctx, insertVisitorSQL, visitor.Imsi, visitor.PartnerID, visitor.Msisdn, visitor.Since); err != nil {
		return fmt.Errorf("insert visitor: %w", err)
	}

	return nil
}

func (db *Database) applySetVisitorMSISDN(ctx context.Context, p *visitorMSISDNPayload) (any, error) {
	res, err := db.runner(ctx).ExecContext(ctx, updateVisitorMSISDNSQL, p.Msisdn, p.Imsi)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	n, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("rows affected: %w", err)
	}

	if n == 0 {
		return nil, ErrNotFound
	}

	return nil, nil
}

func (db *Database) deleteUnregisteredVisitors(ctx context.Context) error {
	imsis, err := queryAll(ctx, db.runner(ctx), listUnregisteredVisitorsSQL, scanValue[string])
	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	for _, imsi := range imsis {
		if err := db.deleteVisitorIfUnregistered(ctx, imsi); err != nil {
			return err
		}
	}

	return nil
}

func (db *Database) deleteVisitorIfUnregistered(ctx context.Context, imsi string) error {
	res, err := db.runner(ctx).ExecContext(ctx, deleteVisitorIfUnregisteredSQL, imsi)
	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	deleted, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}

	if deleted == 0 {
		return nil
	}

	return db.deleteIMSIState(ctx, imsi)
}

func (db *Database) deleteIMSIState(ctx context.Context, imsi string) error {
	runner := db.runner(ctx)

	if _, err := runner.ExecContext(ctx, deleteUERegistrationsOfIMSISQL, imsi); err != nil {
		return fmt.Errorf("delete UE registrations: %w", err)
	}

	if _, err := runner.ExecContext(ctx, deleteLeasesOfIMSISQL, imsi); err != nil {
		return fmt.Errorf("delete IP leases: %w", err)
	}

	return nil
}

func scanVisitor(row scanner) (Visitor, error) {
	var v Visitor

	if err := row.Scan(&v.Imsi, &v.PartnerID, &v.Msisdn, &v.Since, &v.DeregisterUntil, &v.ProfileID, &v.WithdrawingUntil); err != nil {
		return Visitor{}, err
	}

	return v, nil
}
