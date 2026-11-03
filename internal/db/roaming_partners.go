// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/canonical/sqlair"
	"github.com/ellanetworks/core/internal/pki"
	"github.com/prometheus/client_golang/prometheus"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
)

const (
	RoamingPartnersTableName     = "roaming_partners"
	RoamingPartnerPLMNsTableName = "roaming_partner_plmns"
)

const roamingSchema = 26

const (
	listRoamingPartnersStmt        = "SELECT &RoamingPartner.* FROM %s ORDER BY name"
	getRoamingPartnerStmt          = "SELECT &RoamingPartner.* FROM %s WHERE id==$RoamingPartner.id"
	insertRoamingPartnerStmt       = "INSERT INTO %s (id, name, profileID, seppFQDN, seppN32cURI, seppN32fURI, seppTrustAnchorPEM, nrfAPIRoot, diameterPeerID) VALUES ($RoamingPartner.id, $RoamingPartner.name, $RoamingPartner.profileID, $RoamingPartner.seppFQDN, $RoamingPartner.seppN32cURI, $RoamingPartner.seppN32fURI, $RoamingPartner.seppTrustAnchorPEM, $RoamingPartner.nrfAPIRoot, $RoamingPartner.diameterPeerID)"
	updateRoamingPartnerStmt       = "UPDATE %s SET name=$RoamingPartner.name, profileID=$RoamingPartner.profileID, seppFQDN=$RoamingPartner.seppFQDN, seppN32cURI=$RoamingPartner.seppN32cURI, seppN32fURI=$RoamingPartner.seppN32fURI, seppTrustAnchorPEM=$RoamingPartner.seppTrustAnchorPEM, nrfAPIRoot=$RoamingPartner.nrfAPIRoot, diameterPeerID=$RoamingPartner.diameterPeerID WHERE id==$RoamingPartner.id"
	deleteRoamingPartnerStmt       = "DELETE FROM %s WHERE id==$RoamingPartner.id"
	updateVisitorsProfileStmt      = "UPDATE %s SET profileID=$RoamingPartner.profileID WHERE roamingPartnerID==$RoamingPartner.id"
	listRoamingPartnerPLMNsStmt    = "SELECT &roamingPartnerPLMNRow.* FROM %s ORDER BY mcc, mnc"
	listPLMNsOfRoamingPartnerStmt  = "SELECT &roamingPartnerPLMNRow.* FROM %s WHERE partnerID==$roamingPartnerPLMNRow.partnerID ORDER BY mcc, mnc"
	insertRoamingPartnerPLMNStmt   = "INSERT INTO %s (partnerID, mcc, mnc) VALUES ($roamingPartnerPLMNRow.partnerID, $roamingPartnerPLMNRow.mcc, $roamingPartnerPLMNRow.mnc)"
	deleteRoamingPartnerPLMNStmt   = "DELETE FROM %s WHERE partnerID==$roamingPartnerPLMNRow.partnerID AND mcc==$roamingPartnerPLMNRow.mcc AND mnc==$roamingPartnerPLMNRow.mnc"
	countVisitorsOfPartnerStmt     = "SELECT COUNT(*) AS &NumItems.count FROM %s WHERE roamingPartnerID==$visitorFilter.partnerID AND imsi GLOB $visitorFilter.imsiGlob"
	countAttachedVisitorsOfRATStmt = "SELECT COUNT(*) AS &NumItems.count FROM %s AS s JOIN %s AS r ON r.imsi = s.imsi WHERE s.roamingPartnerID==$visitorFilter.partnerID AND r.type==$visitorFilter.regType AND r.purged = 0"
	countRoamingPartnersStmt       = "SELECT COUNT(*) AS &NumItems.count FROM %s"
)

var ErrRoamingConflict = errors.New("roaming configuration conflict")

type roamingPartnerWrite struct {
	Partner RoamingPartner `json:"partner"`
}

type RoamingPLMN struct {
	Mcc string `json:"mcc"`
	Mnc string `json:"mnc"`
}

func (p RoamingPLMN) String() string {
	return p.Mcc + "-" + p.Mnc
}

func (p RoamingPLMN) pki() pki.PLMN {
	return pki.PLMN{MCC: p.Mcc, MNC: p.Mnc}
}

func (p RoamingPLMN) conflicts(o RoamingPLMN) bool {
	if p.Mcc != o.Mcc {
		return false
	}

	if p.pki().Equivalent(o.pki()) {
		return true
	}

	return strings.HasPrefix(p.Mnc, o.Mnc) || strings.HasPrefix(o.Mnc, p.Mnc)
}

func (p RoamingPLMN) imsiGlob() string {
	return p.Mcc + p.Mnc + "*"
}

type roamingPartnerPLMNRow struct {
	PartnerID string `db:"partnerID"`
	Mcc       string `db:"mcc"`
	Mnc       string `db:"mnc"`
}

type visitorFilter struct {
	PartnerID string `db:"partnerID"`
	IMSIGlob  string `db:"imsiGlob"`
	RegType   string `db:"regType"`
}

type RoamingPartner struct {
	ID                 string  `db:"id"`
	Name               string  `db:"name"`
	ProfileID          string  `db:"profileID"`
	SEPPFQDN           *string `db:"seppFQDN"`
	SEPPN32cURI        *string `db:"seppN32cURI"`
	SEPPN32fURI        *string `db:"seppN32fURI"`
	SEPPTrustAnchorPEM *string `db:"seppTrustAnchorPEM"`
	NRFAPIRoot         *string `db:"nrfAPIRoot"`
	DiameterPeerID     *string `db:"diameterPeerID"`
	PLMNs              []RoamingPLMN
	Withdrawals        []RoamingWithdrawal
}

func (p RoamingPartner) HasPLMN(plmn RoamingPLMN) bool {
	return slices.Contains(p.PLMNs, plmn)
}

func (p RoamingPartner) PLMNOfIMSI(imsi string) (RoamingPLMN, bool) {
	for _, plmn := range p.PLMNs {
		if strings.HasPrefix(imsi, plmn.Mcc+plmn.Mnc) {
			return plmn, true
		}
	}

	return RoamingPLMN{}, false
}

func (p RoamingPartner) Validate() error {
	if p.ID == "" {
		return errors.New("roaming partner ID is required")
	}

	if strings.TrimSpace(p.Name) == "" {
		return errors.New("roaming partner name is required")
	}

	if err := validateRoamingPLMNs(p.PLMNs); err != nil {
		return err
	}

	if p.ProfileID == "" {
		return errors.New("roaming partner profile is required")
	}

	if err := p.validate5G(); err != nil {
		return err
	}

	if p.DiameterPeerID != nil && *p.DiameterPeerID == "" {
		return errors.New("roaming partner Diameter peer ID must not be empty")
	}

	return nil
}

func validateRoamingPLMNs(plmns []RoamingPLMN) error {
	if len(plmns) == 0 {
		return errors.New("roaming partner needs at least one PLMN")
	}

	for i, plmn := range plmns {
		if !isDigits(plmn.Mcc, 3, 3) {
			return fmt.Errorf("roaming partner MCC must be 3 digits, got %q", plmn.Mcc)
		}

		if !isDigits(plmn.Mnc, 2, 3) {
			return fmt.Errorf("roaming partner MNC must be 2 or 3 digits, got %q", plmn.Mnc)
		}

		for _, other := range plmns[:i] {
			if plmn.conflicts(other) {
				return fmt.Errorf("roaming partner PLMNs %s and %s are duplicates or ambiguous: under one MCC, no MNC may equal or prefix another, nor equal it once zero-padded", other, plmn)
			}
		}
	}

	return nil
}

func (p RoamingPartner) validate5G() error {
	required := []*string{p.SEPPFQDN, p.SEPPN32cURI, p.SEPPTrustAnchorPEM}

	set := 0

	for _, v := range required {
		if v != nil {
			set++
		}
	}

	if set == 0 {
		if p.SEPPN32fURI != nil || p.NRFAPIRoot != nil {
			return errors.New("roaming partner N32-f URI and NRF API root need a SEPP FQDN, N32-c URI and trust anchor")
		}

		return nil
	}

	if set != len(required) {
		return errors.New("roaming partner SEPP FQDN, N32-c URI and trust anchor must be set together")
	}

	plmns := make([]pki.PLMN, 0, len(p.PLMNs))
	for _, plmn := range p.PLMNs {
		plmns = append(plmns, plmn.pki())
	}

	if err := pki.ValidatePeerSEPPFQDN(*p.SEPPFQDN, plmns); err != nil {
		return fmt.Errorf("roaming partner SEPP FQDN: %w", err)
	}

	if err := validateAPIRoot(*p.SEPPN32cURI, "https"); err != nil {
		return fmt.Errorf("roaming partner N32-c URI: %w", err)
	}

	if p.SEPPN32fURI != nil {
		if err := validateAPIRoot(*p.SEPPN32fURI, "https"); err != nil {
			return fmt.Errorf("roaming partner N32-f URI: %w", err)
		}
	}

	if p.NRFAPIRoot != nil {
		if err := validateAPIRoot(*p.NRFAPIRoot, "http", "https"); err != nil {
			return fmt.Errorf("roaming partner NRF API root: %w", err)
		}
	}

	if _, err := pki.ValidateTrustAnchorPEM([]byte(*p.SEPPTrustAnchorPEM), time.Now()); err != nil {
		return fmt.Errorf("roaming partner SEPP trust anchor: %w", err)
	}

	return nil
}

func isDigits(s string, minLen, maxLen int) bool {
	if len(s) < minLen || len(s) > maxLen {
		return false
	}

	for i := range len(s) {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}

	return true
}

func validateAPIRoot(raw string, schemes ...string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid URI %q: %w", raw, err)
	}

	if !slices.Contains(schemes, u.Scheme) {
		return fmt.Errorf("URI %q must use the %s scheme", raw, strings.Join(schemes, " or "))
	}

	if u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.Opaque != "" {
		return fmt.Errorf("URI %q must be an apiRoot: a scheme, a host, an optional port and an optional path prefix, without user info, query or fragment", raw)
	}

	if strings.HasSuffix(u.Host, ":") {
		return fmt.Errorf("URI %q has an empty port", raw)
	}

	if port := u.Port(); port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return fmt.Errorf("URI %q must have a port between 1 and 65535", raw)
		}
	}

	if strings.HasSuffix(u.Path, "/") {
		return fmt.Errorf("URI %q must not end with /", raw)
	}

	return nil
}

func NormalizeAPIRoot(raw string) string {
	return strings.TrimRight(strings.TrimSpace(raw), "/")
}

func (db *Database) ListRoamingPartners(ctx context.Context) ([]RoamingPartner, error) {
	ctx, span := startRoamingPartnersSpan(ctx, "SELECT")
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(RoamingPartnersTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(RoamingPartnersTableName, "select").Inc()

	if !db.appliedSchemaAtLeast(ctx, roamingSchema) {
		return nil, nil
	}

	partners, err := db.listRoamingPartners(ctx, db.conn())
	if err != nil {
		recordSpanError(span, err)
		return nil, err
	}

	return partners, nil
}

func (db *Database) GetRoamingPartner(ctx context.Context, id string) (*RoamingPartner, error) {
	ctx, span := startRoamingPartnersSpan(ctx, "SELECT")
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(RoamingPartnersTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(RoamingPartnersTableName, "select").Inc()

	if !db.appliedSchemaAtLeast(ctx, roamingSchema) {
		return nil, ErrNotFound
	}

	partner, err := db.getRoamingPartner(ctx, db.conn(), id)
	if err != nil {
		if !errors.Is(err, ErrNotFound) {
			recordSpanError(span, err)
		}

		return nil, err
	}

	return partner, nil
}

func (db *Database) CreateRoamingPartner(ctx context.Context, partner *RoamingPartner) error {
	return db.writeRoamingPartner(ctx, "INSERT", opCreateRoamingPartner, partner)
}

func (db *Database) UpdateRoamingPartner(ctx context.Context, partner *RoamingPartner) error {
	return db.writeRoamingPartner(ctx, "UPDATE", opUpdateRoamingPartner, partner)
}

func (db *Database) writeRoamingPartner(ctx context.Context, operation string, op *ChangesetOp[roamingPartnerWrite, struct{}], partner *RoamingPartner) error {
	if err := partner.Validate(); err != nil {
		return err
	}

	ctx, span := startRoamingPartnersSpan(ctx, operation)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(RoamingPartnersTableName, strings.ToLower(operation)))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(RoamingPartnersTableName, strings.ToLower(operation)).Inc()

	if _, err := op.Invoke(ctx, db, &roamingPartnerWrite{Partner: *partner}); err != nil {
		recordSpanError(span, err)
		return err
	}

	return nil
}

func (db *Database) DeleteRoamingPartner(ctx context.Context, id string) error {
	ctx, span := startRoamingPartnersSpan(ctx, "DELETE")
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(RoamingPartnersTableName, "delete"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(RoamingPartnersTableName, "delete").Inc()

	if _, err := opDeleteRoamingPartner.Invoke(ctx, db, &stringPayload{Value: id}); err != nil {
		recordSpanError(span, err)
		return err
	}

	return nil
}

func (db *Database) listRoamingPartners(ctx context.Context, runner *sqlair.DB) ([]RoamingPartner, error) {
	var partners []RoamingPartner

	if err := runner.Query(ctx, db.listRoamingPartnersStmt).GetAll(&partners); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	var rows []roamingPartnerPLMNRow

	if err := runner.Query(ctx, db.listRoamingPartnerPLMNsStmt).GetAll(&rows); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	byPartner := map[string][]RoamingPLMN{}
	for _, r := range rows {
		byPartner[r.PartnerID] = append(byPartner[r.PartnerID], RoamingPLMN{Mcc: r.Mcc, Mnc: r.Mnc})
	}

	for i := range partners {
		partners[i].PLMNs = byPartner[partners[i].ID]
	}

	if err := db.attachWithdrawals(ctx, partners); err != nil {
		return nil, err
	}

	return partners, nil
}

func (db *Database) getRoamingPartner(ctx context.Context, runner *sqlair.DB, id string) (*RoamingPartner, error) {
	row := RoamingPartner{ID: id}

	if err := runner.Query(ctx, db.getRoamingPartnerStmt, row).Get(&row); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	var rows []roamingPartnerPLMNRow

	if err := runner.Query(ctx, db.listPLMNsOfRoamingPartnerStmt, roamingPartnerPLMNRow{PartnerID: id}).GetAll(&rows); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	for _, r := range rows {
		row.PLMNs = append(row.PLMNs, RoamingPLMN{Mcc: r.Mcc, Mnc: r.Mnc})
	}

	withdrawals, err := db.listWithdrawalsOfPartner(ctx, id)
	if err != nil {
		return nil, err
	}

	row.Withdrawals = withdrawals

	return &row, nil
}

func (db *Database) prepareHomePLMNChange(ctx context.Context, op *Operator) (bool, error) {
	if !db.appliedSchemaAtLeast(ctx, roamingSchema) {
		return false, nil
	}

	runner := db.runner(ctx)

	var current Operator

	if err := runner.Query(ctx, db.getOperatorStmt).Get(&current); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return false, fmt.Errorf("query failed: %w", err)
	}

	if current.Mcc == op.Mcc && current.Mnc == op.Mnc {
		return false, nil
	}

	var count NumItems

	if err := runner.Query(ctx, db.countRoamingPartnersStmt).Get(&count); err != nil {
		return false, fmt.Errorf("query failed: %w", err)
	}

	if count.Count > 0 {
		return false, fmt.Errorf("%w: the home PLMN cannot change while roaming partners exist", ErrRoamingConflict)
	}

	return db.deleteSEPPIdentities(ctx)
}

func (db *Database) countVisitors(ctx context.Context, stmt *sqlair.Statement, filter visitorFilter) (int, error) {
	var count NumItems

	if err := db.runner(ctx).Query(ctx, stmt, filter).Get(&count); err != nil {
		return 0, fmt.Errorf("query failed: %w", err)
	}

	return count.Count, nil
}

func (db *Database) checkRoamingPartner(ctx context.Context, p *RoamingPartner) error {
	runner := db.runner(ctx)

	var op Operator

	if err := runner.Query(ctx, db.getOperatorStmt).Get(&op); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("query failed: %w", err)
	}

	home := RoamingPLMN{Mcc: op.Mcc, Mnc: op.Mnc}

	for _, plmn := range p.PLMNs {
		if plmn.conflicts(home) {
			return fmt.Errorf("%w: PLMN %s is or is ambiguous with the home PLMN %s", ErrRoamingConflict, plmn, home)
		}
	}

	if err := runner.Query(ctx, db.getProfileByIDStmt, Profile{ID: p.ProfileID}).Get(&Profile{}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("%w: profile %s does not exist", ErrRoamingConflict, p.ProfileID)
		}

		return fmt.Errorf("query failed: %w", err)
	}

	if p.DiameterPeerID != nil {
		peer := diameterPeerRow{ID: *p.DiameterPeerID}

		if err := runner.Query(ctx, db.getDiameterPeerStmt, peer).Get(&peer); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("%w: Diameter peer %s does not exist", ErrRoamingConflict, peer.ID)
			}

			return fmt.Errorf("query failed: %w", err)
		}

		if peer.Role != DiameterPeerRoleRoaming {
			return fmt.Errorf("%w: Diameter peer %s is not a roaming peer", ErrRoamingConflict, peer.ID)
		}
	}

	partners, err := db.listRoamingPartners(ctx, runner)
	if err != nil {
		return err
	}

	for _, other := range partners {
		if other.ID == p.ID {
			continue
		}

		for _, plmn := range p.PLMNs {
			for _, theirs := range other.PLMNs {
				if plmn.conflicts(theirs) {
					return fmt.Errorf("%w: PLMN %s is or is ambiguous with PLMN %s of roaming partner %s", ErrRoamingConflict, plmn, theirs, other.Name)
				}
			}
		}
	}

	return nil
}

func (db *Database) insertRoamingPartnerPLMNs(ctx context.Context, partnerID string, plmns []RoamingPLMN) error {
	for _, plmn := range plmns {
		row := roamingPartnerPLMNRow{PartnerID: partnerID, Mcc: plmn.Mcc, Mnc: plmn.Mnc}

		if err := db.runner(ctx).Query(ctx, db.insertRoamingPartnerPLMNStmt, row).Run(); err != nil {
			if isPrimaryKeyError(err) || isUniqueNameError(err) {
				return fmt.Errorf("%w: PLMN %s belongs to another roaming partner", ErrRoamingConflict, plmn)
			}

			return fmt.Errorf("insert roaming partner PLMN: %w", err)
		}
	}

	return nil
}

func (db *Database) insertRoamingPartnerRow(ctx context.Context, stmt *sqlair.Statement, p *RoamingPartner) error {
	err := db.runner(ctx).Query(ctx, stmt, p).Run()
	if err == nil {
		return nil
	}

	if isUniqueNameError(err) || isPrimaryKeyError(err) {
		return ErrAlreadyExists
	}

	return fmt.Errorf("query failed: %w", err)
}

func (db *Database) applyCreateRoamingPartner(ctx context.Context, w *roamingPartnerWrite) (any, error) {
	p := &w.Partner

	if err := db.checkRoamingPartner(ctx, p); err != nil {
		return nil, err
	}

	if err := db.insertRoamingPartnerRow(ctx, db.insertRoamingPartnerStmt, p); err != nil {
		return nil, err
	}

	if err := db.insertRoamingPartnerPLMNs(ctx, p.ID, p.PLMNs); err != nil {
		return nil, err
	}

	return nil, nil
}

func (db *Database) checkRemovedAgreements(ctx context.Context, existing, p *RoamingPartner) error {
	for _, plmn := range existing.PLMNs {
		if p.HasPLMN(plmn) {
			continue
		}

		n, err := db.countVisitors(ctx, db.countVisitorsOfPartnerStmt, visitorFilter{PartnerID: p.ID, IMSIGlob: plmn.imsiGlob()})
		if err != nil {
			return err
		}

		if n > 0 {
			return fmt.Errorf("%w: PLMN %s cannot be removed while %d of its roamers are attached", ErrRoamersAttached, plmn, n)
		}
	}

	rats := []struct {
		removed bool
		regType string
		name    string
	}{
		{existing.SEPPFQDN != nil && p.SEPPFQDN == nil, UERegistrationTypeAMF3GPPAccess, "5G"},
		{existing.DiameterPeerID != nil && p.DiameterPeerID == nil, UERegistrationTypeMME, "4G"},
	}

	for _, rat := range rats {
		if !rat.removed {
			continue
		}

		n, err := db.countVisitors(ctx, db.countAttachedVisitorsOfRATStmt, visitorFilter{PartnerID: p.ID, RegType: rat.regType})
		if err != nil {
			return err
		}

		if n > 0 {
			return fmt.Errorf("%w: the %s agreement cannot be removed while %d roamers are attached over %s", ErrRoamersAttached, rat.name, n, rat.name)
		}
	}

	return nil
}

func (db *Database) applyUpdateRoamingPartner(ctx context.Context, w *roamingPartnerWrite) (any, error) {
	p := &w.Partner

	existing, err := db.getRoamingPartner(ctx, db.runner(ctx), p.ID)
	if err != nil {
		return nil, err
	}

	if err := db.checkRoamingPartner(ctx, p); err != nil {
		return nil, err
	}

	if err := db.checkRemovedAgreements(ctx, existing, p); err != nil {
		return nil, err
	}

	if err := db.deleteWithdrawals(ctx, p.ID); err != nil {
		return nil, err
	}

	if err := db.insertRoamingPartnerRow(ctx, db.updateRoamingPartnerStmt, p); err != nil {
		return nil, err
	}

	var added []RoamingPLMN

	for _, plmn := range existing.PLMNs {
		if p.HasPLMN(plmn) {
			continue
		}

		row := roamingPartnerPLMNRow{PartnerID: p.ID, Mcc: plmn.Mcc, Mnc: plmn.Mnc}
		if err := db.runner(ctx).Query(ctx, db.deleteRoamingPartnerPLMNStmt, row).Run(); err != nil {
			return nil, fmt.Errorf("delete roaming partner PLMN: %w", err)
		}
	}

	for _, plmn := range p.PLMNs {
		if !existing.HasPLMN(plmn) {
			added = append(added, plmn)
		}
	}

	if err := db.insertRoamingPartnerPLMNs(ctx, p.ID, added); err != nil {
		return nil, err
	}

	if existing.ProfileID != p.ProfileID {
		if err := db.runner(ctx).Query(ctx, db.updateVisitorsProfileStmt, p).Run(); err != nil {
			return nil, fmt.Errorf("update visitors profile: %w", err)
		}
	}

	return nil, nil
}

func (db *Database) applyDeleteRoamingPartner(ctx context.Context, p *stringPayload) (any, error) {
	n, err := db.countVisitors(ctx, db.countVisitorsOfPartnerStmt, visitorFilter{PartnerID: p.Value, IMSIGlob: "*"})
	if err != nil {
		return nil, err
	}

	if n > 0 {
		return nil, fmt.Errorf("%w: the roaming partner cannot be deleted while %d of its roamers are attached", ErrRoamersAttached, n)
	}

	if err := db.deleteWithdrawals(ctx, p.Value); err != nil {
		return nil, err
	}

	var outcome sqlair.Outcome

	if err := db.runner(ctx).Query(ctx, db.deleteRoamingPartnerStmt, RoamingPartner{ID: p.Value}).Get(&outcome); err != nil {
		if isForeignKeyRestrictError(err) {
			return nil, ErrInUse
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	rowsAffected, err := outcome.Result().RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return nil, ErrNotFound
	}

	return nil, nil
}

func startRoamingPartnersSpan(ctx context.Context, operation string) (context.Context, trace.Span) {
	querySummary := fmt.Sprintf("%s %s", operation, RoamingPartnersTableName)

	return tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName(operation),
			semconv.DBCollectionName(RoamingPartnersTableName),
		),
	)
}
