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

	"github.com/canonical/sqlair"
	"github.com/prometheus/client_golang/prometheus"
)

const RoamingWithdrawalsTableName = "roaming_withdrawals"

const (
	listRoamingWithdrawalsStmt         = "SELECT &RoamingWithdrawal.* FROM %s ORDER BY partnerID, mcc, mnc, type, imsi"
	listWithdrawalsOfPartnerStmt       = "SELECT &RoamingWithdrawal.* FROM %s WHERE partnerID==$RoamingWithdrawal.partnerID ORDER BY mcc, mnc, type, imsi"
	insertRoamingWithdrawalStmt        = "INSERT INTO %s (partnerID, mcc, mnc, type, imsi, expiresAt) VALUES ($RoamingWithdrawal.partnerID, $RoamingWithdrawal.mcc, $RoamingWithdrawal.mnc, $RoamingWithdrawal.type, $RoamingWithdrawal.imsi, $RoamingWithdrawal.expiresAt) ON CONFLICT(partnerID, mcc, mnc, type, imsi) DO UPDATE SET expiresAt = excluded.expiresAt"
	deleteWithdrawalsOfPartnerStmt     = "DELETE FROM %s WHERE partnerID==$RoamingWithdrawal.partnerID AND imsi == ''"
	deleteRoamerWithdrawalStmt         = "DELETE FROM %s WHERE imsi==$RoamingWithdrawal.imsi"
	listVisitorRegistrationsOfNodeStmt = "SELECT &UERegistration.* FROM %s WHERE nodeID==$UERegistration.nodeID AND purged==0 AND imsi IN (SELECT imsi FROM %s) ORDER BY imsi, type"
	deleteExpiredWithdrawalsStmt       = "DELETE FROM %s WHERE expiresAt <= $RoamingWithdrawal.expiresAt"
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
	IMSI      string `db:"imsi" json:"imsi,omitempty"`
	ExpiresAt int64  `db:"expiresAt" json:"expiresAt"`
}

func (w RoamingWithdrawal) Covers(imsi, regType string, now time.Time) bool {
	if now.Unix() >= w.ExpiresAt {
		return false
	}

	if w.IMSI != "" && w.IMSI != imsi {
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

func (w RoamingWithdrawal) RoamerOnly() bool {
	return w.IMSI != ""
}

func (p RoamingPartner) Withdrawal(imsi, regType string, now time.Time) (RoamingWithdrawal, bool) {
	for _, w := range p.Withdrawals {
		if w.Covers(imsi, regType, now) {
			return w, true
		}
	}

	return RoamingWithdrawal{}, false
}

func RoamingWithdrawalsFor(existing RoamingPartner, updated *RoamingPartner) []RoamingWithdrawal {
	if updated == nil {
		return []RoamingWithdrawal{{PartnerID: existing.ID}}
	}

	var out []RoamingWithdrawal

	for _, plmn := range existing.PLMNs {
		if !updated.HasPLMN(plmn) {
			out = append(out, RoamingWithdrawal{PartnerID: existing.ID, Mcc: plmn.Mcc, Mnc: plmn.Mnc})
		}
	}

	if existing.SEPPFQDN != nil && updated.SEPPFQDN == nil {
		out = append(out, RoamingWithdrawal{PartnerID: existing.ID, Type: UERegistrationTypeAMF3GPPAccess})
	}

	if existing.DiameterPeerID != nil && updated.DiameterPeerID == nil {
		out = append(out, RoamingWithdrawal{PartnerID: existing.ID, Type: UERegistrationTypeMME})
	}

	return out
}

type roamingWithdrawalWrite struct {
	PartnerID   string              `json:"partnerID"`
	Withdrawals []RoamingWithdrawal `json:"withdrawals"`
	TTLSeconds  int64               `json:"ttlSeconds"`
}

type roamerWithdrawalWrite struct {
	IMSI       string `json:"imsi"`
	TTLSeconds int64  `json:"ttlSeconds"`
}

func withdrawalTTLSeconds(ttl time.Duration) (int64, error) {
	seconds := int64(ttl / time.Second)
	if seconds < 1 {
		return 0, fmt.Errorf("a roaming withdrawal needs a lifetime of at least one second, got %s", ttl)
	}

	return seconds, nil
}

var (
	opWithdrawRoamers         = registerChangesetOp("WithdrawRoamers", (*Database).applyWithdrawRoamers, RequireSchema(26), AffectsTopic(TopicRoaming), AffectsTopic(TopicUERegistrations))
	opClearRoamingWithdrawals = registerChangesetOp("ClearRoamingWithdrawals", (*Database).applyClearRoamingWithdrawals, RequireSchema(26), AffectsTopic(TopicRoaming))
	opWithdrawRoamer          = registerChangesetOp("WithdrawRoamer", (*Database).applyWithdrawRoamer, RequireSchema(26), AffectsTopic(TopicRoaming), AffectsTopic(TopicUERegistrations))
	opClearRoamerWithdrawal   = registerChangesetOp("ClearRoamerWithdrawal", (*Database).applyClearRoamerWithdrawal, RequireSchema(26), AffectsTopic(TopicRoaming))
	opDeleteExpiredWithdrawal = registerChangesetOpReturning[emptyPayload, int64]("DeleteExpiredRoamingWithdrawals", (*Database).applyDeleteExpiredWithdrawals, RequireSchema(26), AffectsTopic(TopicRoaming))
)

func (db *Database) WithdrawRoamers(ctx context.Context, partnerID string, withdrawals []RoamingWithdrawal, ttl time.Duration) error {
	ttlSeconds, err := withdrawalTTLSeconds(ttl)
	if err != nil {
		return err
	}

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

	if _, err := opWithdrawRoamers.Invoke(ctx, db, &roamingWithdrawalWrite{PartnerID: partnerID, Withdrawals: withdrawals, TTLSeconds: ttlSeconds}); err != nil {
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
		if w.RoamerOnly() {
			continue
		}

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

	now := time.Now()

	if _, err := db.deleteExpiredWithdrawals(ctx, now); err != nil {
		return nil, err
	}

	for _, row := range w.Withdrawals {
		row.PartnerID = w.PartnerID
		row.ExpiresAt = now.Unix() + w.TTLSeconds

		switch {
		case row.IMSI != "":
			return nil, fmt.Errorf("withdrawal of roaming partner %s names an IMSI", existing.Name)
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

func (db *Database) WithdrawRoamer(ctx context.Context, imsi string, ttl time.Duration) error {
	ttlSeconds, err := withdrawalTTLSeconds(ttl)
	if err != nil {
		return err
	}

	ctx, span := startRoamingPartnersSpan(ctx, "WITHDRAW ROAMER")
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(RoamingWithdrawalsTableName, "insert"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(RoamingWithdrawalsTableName, "insert").Inc()

	if _, err := opWithdrawRoamer.Invoke(ctx, db, &roamerWithdrawalWrite{IMSI: imsi, TTLSeconds: ttlSeconds}); err != nil {
		recordSpanError(span, err)
		return err
	}

	return nil
}

func (db *Database) ClearRoamerWithdrawal(ctx context.Context, imsi string) error {
	ctx, span := startRoamingPartnersSpan(ctx, "CLEAR ROAMER WITHDRAWAL")
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(RoamingWithdrawalsTableName, "delete"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(RoamingWithdrawalsTableName, "delete").Inc()

	if _, err := opClearRoamerWithdrawal.Invoke(ctx, db, &stringPayload{Value: imsi}); err != nil {
		recordSpanError(span, err)
		return err
	}

	return nil
}

func (db *Database) applyWithdrawRoamer(ctx context.Context, w *roamerWithdrawalWrite) (any, error) {
	visitor, err := db.getVisitor(ctx, db.runner(ctx), w.IMSI)
	if err != nil {
		return nil, err
	}

	now := time.Now()

	if _, err := db.deleteExpiredWithdrawals(ctx, now); err != nil {
		return nil, err
	}

	row := RoamingWithdrawal{PartnerID: visitor.PartnerID, IMSI: w.IMSI, ExpiresAt: now.Unix() + w.TTLSeconds}

	if err := db.runner(ctx).Query(ctx, db.insertRoamingWithdrawalStmt, row).Run(); err != nil {
		return nil, fmt.Errorf("insert roamer withdrawal: %w", err)
	}

	return nil, nil
}

func (db *Database) DeleteExpiredRoamingWithdrawals(ctx context.Context) (int64, error) {
	ctx, span := startRoamingPartnersSpan(ctx, "DELETE EXPIRED WITHDRAWALS")
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(RoamingWithdrawalsTableName, "delete"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(RoamingWithdrawalsTableName, "delete").Inc()

	deleted, err := opDeleteExpiredWithdrawal.Invoke(ctx, db, &emptyPayload{})
	if err != nil {
		recordSpanError(span, err)
		return 0, err
	}

	return deleted, nil
}

func (db *Database) applyDeleteExpiredWithdrawals(ctx context.Context, _ *emptyPayload) (any, error) {
	return db.deleteExpiredWithdrawals(ctx, time.Now())
}

func (db *Database) deleteExpiredWithdrawals(ctx context.Context, now time.Time) (int64, error) {
	var outcome sqlair.Outcome

	if err := db.runner(ctx).Query(ctx, db.deleteExpiredWithdrawalsStmt, RoamingWithdrawal{ExpiresAt: now.Unix()}).Get(&outcome); err != nil {
		return 0, fmt.Errorf("delete expired roaming withdrawals: %w", err)
	}

	deleted, err := outcome.Result().RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("rows affected: %w", err)
	}

	return deleted, nil
}

func (db *Database) applyClearRoamerWithdrawal(ctx context.Context, p *stringPayload) (any, error) {
	if err := db.runner(ctx).Query(ctx, db.deleteRoamerWithdrawalStmt, RoamingWithdrawal{IMSI: p.Value}).Run(); err != nil {
		return nil, fmt.Errorf("delete roamer withdrawal: %w", err)
	}

	return nil, nil
}
