// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const RoamingWithdrawalsTableName = "roaming_withdrawals"

const (
	listRoamingWithdrawalsStmt         = "SELECT &RoamingWithdrawal.* FROM %s ORDER BY partnerID, mcc, mnc, type"
	listWithdrawalsOfPartnerStmt       = "SELECT &RoamingWithdrawal.* FROM %s WHERE partnerID==$RoamingWithdrawal.partnerID ORDER BY mcc, mnc, type"
	insertRoamingWithdrawalStmt        = "INSERT INTO %s (partnerID, mcc, mnc, type, expiresAt) VALUES ($RoamingWithdrawal.partnerID, $RoamingWithdrawal.mcc, $RoamingWithdrawal.mnc, $RoamingWithdrawal.type, $RoamingWithdrawal.expiresAt)"
	deleteWithdrawalsOfPartnerStmt     = "DELETE FROM %s WHERE partnerID==$RoamingWithdrawal.partnerID"
	listVisitorRegistrationsOfNodeStmt = "SELECT &UERegistration.* FROM %s WHERE nodeID==$UERegistration.nodeID AND purged==0 AND imsi IN (SELECT imsi FROM %s WHERE roamingPartnerID IS NOT NULL) ORDER BY imsi, type"
)

var (
	ErrRoamersAttached = fmt.Errorf("%w: roamers are attached", ErrRoamingConflict)
	ErrRoamingRefused  = errors.New("the roaming agreement is being withdrawn")
)

type RoamingWithdrawal struct {
	PartnerID string `db:"partnerID" json:"partnerID"`
	Mcc       string `db:"mcc" json:"mcc"`
	Mnc       string `db:"mnc" json:"mnc"`
	Type      string `db:"type" json:"type"`
	ExpiresAt int64  `db:"expiresAt" json:"expiresAt"`
}

func (w RoamingWithdrawal) Covers(imsi, regType string, now time.Time) bool {
	if now.Unix() >= w.ExpiresAt {
		return false
	}

	if w.Mcc != "" && !strings.HasPrefix(imsi, w.Mcc+w.Mnc) {
		return false
	}

	return w.Type == "" || w.Type == regType
}

func (w RoamingWithdrawal) AgreementOnly() bool {
	return w.Type != "" && w.Mcc == ""
}

func (p RoamingPartner) Withdrawal(imsi, regType string, now time.Time) (RoamingWithdrawal, bool) {
	for _, w := range p.Withdrawals {
		if w.Covers(imsi, regType, now) {
			return w, true
		}
	}

	return RoamingWithdrawal{}, false
}

func RoamingWithdrawalsFor(existing RoamingPartner, updated *RoamingPartner, expiresAt time.Time) []RoamingWithdrawal {
	at := expiresAt.Unix()

	if updated == nil {
		return []RoamingWithdrawal{{PartnerID: existing.ID, ExpiresAt: at}}
	}

	var out []RoamingWithdrawal

	for _, plmn := range existing.PLMNs {
		if !updated.HasPLMN(plmn) {
			out = append(out, RoamingWithdrawal{PartnerID: existing.ID, Mcc: plmn.Mcc, Mnc: plmn.Mnc, ExpiresAt: at})
		}
	}

	if existing.SEPPFQDN != nil && updated.SEPPFQDN == nil {
		out = append(out, RoamingWithdrawal{PartnerID: existing.ID, Type: UERegistrationTypeAMF3GPPAccess, ExpiresAt: at})
	}

	if existing.DiameterPeerID != nil && updated.DiameterPeerID == nil {
		out = append(out, RoamingWithdrawal{PartnerID: existing.ID, Type: UERegistrationTypeMME, ExpiresAt: at})
	}

	return out
}

type roamingWithdrawalWrite struct {
	PartnerID   string              `json:"partnerID"`
	Withdrawals []RoamingWithdrawal `json:"withdrawals"`
}

var (
	opWithdrawRoamers         = registerChangesetOp("WithdrawRoamers", (*Database).applyWithdrawRoamers, RequireSchema(26), AffectsTopic(TopicRoaming), AffectsTopic(TopicUERegistrations))
	opClearRoamingWithdrawals = registerChangesetOp("ClearRoamingWithdrawals", (*Database).applyClearRoamingWithdrawals, RequireSchema(26), AffectsTopic(TopicRoaming))
)

func (db *Database) WithdrawRoamers(ctx context.Context, partnerID string, withdrawals []RoamingWithdrawal) error {
	for _, w := range withdrawals {
		if w.PartnerID != partnerID {
			return fmt.Errorf("withdrawal of roaming partner %s given for %s", w.PartnerID, partnerID)
		}
	}

	ctx, span := startRoamingPartnersSpan(ctx, "WITHDRAW")
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(RoamingWithdrawalsTableName, "insert"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(RoamingWithdrawalsTableName, "insert").Inc()

	if _, err := opWithdrawRoamers.Invoke(ctx, db, &roamingWithdrawalWrite{PartnerID: partnerID, Withdrawals: withdrawals}); err != nil {
		recordSpanError(span, err)
		return err
	}

	return nil
}

func (db *Database) ClearRoamingWithdrawals(ctx context.Context, partnerID string) error {
	ctx, span := startRoamingPartnersSpan(ctx, "CLEAR WITHDRAWALS")
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(RoamingWithdrawalsTableName, "delete"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(RoamingWithdrawalsTableName, "delete").Inc()

	if _, err := opClearRoamingWithdrawals.Invoke(ctx, db, &stringPayload{Value: partnerID}); err != nil {
		recordSpanError(span, err)
		return err
	}

	return nil
}

func (db *Database) CountWithdrawnRoamers(ctx context.Context, partnerID string) (int, error) {
	ctx, span := startRoamingPartnersSpan(ctx, "COUNT WITHDRAWN")
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(RoamingWithdrawalsTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(RoamingWithdrawalsTableName, "select").Inc()

	if !db.appliedSchemaAtLeast(ctx, roamingSchema) {
		return 0, nil
	}

	withdrawals, err := db.listWithdrawalsOfPartner(ctx, partnerID)
	if err != nil {
		recordSpanError(span, err)
		return 0, err
	}

	total := 0

	for _, w := range withdrawals {
		n, err := db.countWithdrawn(ctx, w)
		if err != nil {
			recordSpanError(span, err)
			return 0, err
		}

		total += n
	}

	return total, nil
}

func (db *Database) RoamingWithdrawalOf(ctx context.Context, imsi, regType string) (*RoamingWithdrawal, error) {
	if !db.appliedSchemaAtLeast(ctx, roamingSchema) {
		return nil, ErrNotFound
	}

	visitor, err := db.GetSubscriberOrVisitor(ctx, imsi)
	if err != nil {
		return nil, err
	}

	if visitor.RoamingPartnerID == nil {
		return nil, ErrNotFound
	}

	withdrawals, err := db.listWithdrawalsOfPartner(ctx, *visitor.RoamingPartnerID)
	if err != nil {
		return nil, err
	}

	now := time.Now()

	for _, w := range withdrawals {
		if w.Covers(imsi, regType, now) {
			return &w, nil
		}
	}

	return nil, ErrNotFound
}

func (db *Database) ListVisitorRegistrationsOfNode(ctx context.Context, nodeID string) ([]UERegistration, error) {
	if !db.appliedSchemaAtLeast(ctx, roamingSchema) {
		return nil, nil
	}

	var regs []UERegistration

	if err := db.conn().Query(ctx, db.listVisitorRegistrationsOfNodeStmt, UERegistration{NodeID: nodeID}).GetAll(&regs); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return regs, nil
}

func (db *Database) listWithdrawalsOfPartner(ctx context.Context, partnerID string) ([]RoamingWithdrawal, error) {
	var out []RoamingWithdrawal

	if err := db.runner(ctx).Query(ctx, db.listWithdrawalsOfPartnerStmt, RoamingWithdrawal{PartnerID: partnerID}).GetAll(&out); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return out, nil
}

func (db *Database) attachWithdrawals(ctx context.Context, partners []RoamingPartner) error {
	var rows []RoamingWithdrawal

	if err := db.runner(ctx).Query(ctx, db.listRoamingWithdrawalsStmt).GetAll(&rows); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("query failed: %w", err)
	}

	for i := range partners {
		for _, w := range rows {
			if w.PartnerID == partners[i].ID {
				partners[i].Withdrawals = append(partners[i].Withdrawals, w)
			}
		}
	}

	return nil
}

func (db *Database) countWithdrawn(ctx context.Context, w RoamingWithdrawal) (int, error) {
	if w.Type != "" && w.Mcc == "" {
		return db.countVisitors(ctx, db.countAttachedVisitorsOfRATStmt, visitorFilter{PartnerID: w.PartnerID, RegType: w.Type})
	}

	glob := "*"
	if w.Mcc != "" {
		glob = RoamingPLMN{Mcc: w.Mcc, Mnc: w.Mnc}.imsiGlob()
	}

	return db.countVisitors(ctx, db.countVisitorsOfPartnerStmt, visitorFilter{PartnerID: w.PartnerID, IMSIGlob: glob})
}

func (db *Database) applyWithdrawRoamers(ctx context.Context, w *roamingWithdrawalWrite) (any, error) {
	existing, err := db.getRoamingPartner(ctx, db.runner(ctx), w.PartnerID)
	if err != nil {
		return nil, err
	}

	if err := db.deleteWithdrawals(ctx, w.PartnerID); err != nil {
		return nil, err
	}

	for _, row := range w.Withdrawals {
		switch {
		case row.Mcc != "" && !existing.HasPLMN(RoamingPLMN{Mcc: row.Mcc, Mnc: row.Mnc}):
			return nil, fmt.Errorf("%w: PLMN %s-%s is not a PLMN of roaming partner %s", ErrRoamingConflict, row.Mcc, row.Mnc, existing.Name)
		case row.Type != "" && !slices.Contains([]string{UERegistrationTypeMME, UERegistrationTypeAMF3GPPAccess}, row.Type):
			return nil, fmt.Errorf("invalid withdrawal registration type %q", row.Type)
		}

		if err := db.runner(ctx).Query(ctx, db.insertRoamingWithdrawalStmt, row).Run(); err != nil {
			if isPrimaryKeyError(err) {
				continue
			}

			return nil, fmt.Errorf("insert roaming withdrawal: %w", err)
		}
	}

	return nil, nil
}

func (db *Database) applyClearRoamingWithdrawals(ctx context.Context, p *stringPayload) (any, error) {
	return nil, db.deleteWithdrawals(ctx, p.Value)
}

func (db *Database) deleteWithdrawals(ctx context.Context, partnerID string) error {
	if err := db.runner(ctx).Query(ctx, db.deleteWithdrawalsOfPartnerStmt, RoamingWithdrawal{PartnerID: partnerID}).Run(); err != nil {
		return fmt.Errorf("delete roaming withdrawals: %w", err)
	}

	return nil
}

func (db *Database) refuseWithdrawnVisitor(ctx context.Context, imsi, partnerID, regType string) error {
	withdrawals, err := db.listWithdrawalsOfPartner(ctx, partnerID)
	if err != nil {
		return err
	}

	now := time.Now()

	for _, w := range withdrawals {
		if w.Covers(imsi, regType, now) {
			return fmt.Errorf("%w: IMSI %s", ErrRoamingRefused, imsi)
		}
	}

	return nil
}
