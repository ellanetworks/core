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

	"github.com/ellanetworks/core/internal/pki"
)

const (
	RoamingPartnersTableName     = "roaming_partners"
	RoamingPartnerPLMNsTableName = "roaming_partner_plmns"
)

const roamingSchema = 27

const (
	listRoamingPartnersSQL          = "SELECT id, name, profileID, seppFQDN, seppN32cURI, seppN32fURI, seppTrustAnchorPEM, diameterPeerID, withdrawingUntil FROM " + RoamingPartnersTableName + " ORDER BY name"
	getRoamingPartnerSQL            = "SELECT id, name, profileID, seppFQDN, seppN32cURI, seppN32fURI, seppTrustAnchorPEM, diameterPeerID, withdrawingUntil FROM " + RoamingPartnersTableName + " WHERE id==?"
	insertRoamingPartnerSQL         = "INSERT INTO " + RoamingPartnersTableName + " (id, name, profileID, seppFQDN, seppN32cURI, seppN32fURI, seppTrustAnchorPEM, diameterPeerID) VALUES (?, ?, ?, ?, ?, ?, ?, ?)"
	updateRoamingPartnerSQL         = "UPDATE " + RoamingPartnersTableName + " SET name=?, profileID=?, seppFQDN=?, seppN32cURI=?, seppN32fURI=?, seppTrustAnchorPEM=?, diameterPeerID=? WHERE id==?"
	deleteRoamingPartnerSQL         = "DELETE FROM " + RoamingPartnersTableName + " WHERE id==?"
	listRoamingPartnerPLMNsSQL      = "SELECT partnerID, mcc, mnc, nrfAPIRoot FROM " + RoamingPartnerPLMNsTableName + " ORDER BY mcc, mnc"
	listPLMNsOfRoamingPartnerSQL    = "SELECT partnerID, mcc, mnc, nrfAPIRoot FROM " + RoamingPartnerPLMNsTableName + " WHERE partnerID==? ORDER BY mcc, mnc"
	insertRoamingPartnerPLMNSQL     = "INSERT INTO " + RoamingPartnerPLMNsTableName + " (partnerID, mcc, mnc, nrfAPIRoot) VALUES (?, ?, ?, ?)"
	updateRoamingPartnerPLMNSQL     = "UPDATE " + RoamingPartnerPLMNsTableName + " SET nrfAPIRoot=? WHERE partnerID==? AND mcc==? AND mnc==?"
	deleteRoamingPartnerPLMNSQL     = "DELETE FROM " + RoamingPartnerPLMNsTableName + " WHERE partnerID==? AND mcc==? AND mnc==?"
	countVisitorsOfPartnerSQL       = "SELECT COUNT(*) FROM " + VisitorsTableName + " WHERE partnerID==? AND imsi GLOB ?"
	countAttachedVisitorsOfRATSQL   = "SELECT COUNT(*) FROM " + VisitorsTableName + " AS v JOIN " + UERegistrationsTableName + " AS r ON r.imsi = v.imsi WHERE v.partnerID==? AND r.type==? AND r.purged = 0"
	countRoamingPartnersSQL         = "SELECT COUNT(*) FROM " + RoamingPartnersTableName
	setRoamingPartnerWithdrawingSQL = "UPDATE " + RoamingPartnersTableName + " SET withdrawingUntil=? WHERE id==?"
)

var (
	ErrRoamingConflict     = errors.New("roaming configuration conflict")
	ErrRoamersAttached     = fmt.Errorf("%w: roamers are attached", ErrRoamingConflict)
	ErrRoamingRefused      = errors.New("the roaming agreement is being withdrawn")
	ErrRoamerDeregistering = errors.New("the operator is deregistering the roamer")
)

type withdrawingPayload struct {
	Key   string `json:"key"`
	Until int64  `json:"until"`
}

var (
	opSetRoamingPartnerWithdrawing = registerChangesetOp("SetRoamingPartnerWithdrawing", (*Database).applySetRoamingPartnerWithdrawing, RequireSchema(roamingSchema), AffectsTopic(TopicRoaming), AffectsTopic(TopicUERegistrations))
	opSetVisitorDeregistering      = registerChangesetOp("SetVisitorDeregistering", (*Database).applySetVisitorDeregistering, RequireSchema(roamingSchema), AffectsTopic(TopicUERegistrations))
)

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

type RoamingNRFAPIRoot struct {
	RoamingPLMN
	APIRoot string `json:"apiRoot"`
}

type roamingPartnerPLMNRow struct {
	PartnerID  string
	Mcc        string
	Mnc        string
	NRFAPIRoot *string
}

type RoamingPartner struct {
	ID                 string
	Name               string
	ProfileID          string
	SEPPFQDN           *string
	SEPPN32cURI        *string
	SEPPN32fURI        *string
	SEPPTrustAnchorPEM *string
	DiameterPeerID     *string
	PLMNs              []RoamingPLMN
	WithdrawingUntil   int64
	NRFAPIRoots        []RoamingNRFAPIRoot
}

func (p RoamingPartner) Withdrawing(now time.Time) bool {
	return now.Unix() < p.WithdrawingUntil
}

func (p RoamingPartner) hasAgreement(regType string) bool {
	switch regType {
	case UERegistrationTypeAMF3GPPAccess:
		return p.SEPPFQDN != nil
	case UERegistrationTypeMME:
		return p.DiameterPeerID != nil
	}

	return false
}

func (p RoamingPartner) HasPLMN(plmn RoamingPLMN) bool {
	return slices.Contains(p.PLMNs, plmn)
}

func (p RoamingPartner) NRFAPIRootOf(plmn RoamingPLMN) *string {
	for _, r := range p.NRFAPIRoots {
		if r.RoamingPLMN == plmn {
			return &r.APIRoot
		}
	}

	return nil
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
		if p.SEPPN32fURI != nil || len(p.NRFAPIRoots) > 0 {
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

	for i, r := range p.NRFAPIRoots {
		if !p.HasPLMN(r.RoamingPLMN) {
			return fmt.Errorf("roaming partner NRF API root is for PLMN %s, which is not a PLMN of the partner", r.RoamingPLMN)
		}

		if slices.ContainsFunc(p.NRFAPIRoots[:i], func(o RoamingNRFAPIRoot) bool { return o.RoamingPLMN == r.RoamingPLMN }) {
			return fmt.Errorf("roaming partner has more than one NRF API root for PLMN %s", r.RoamingPLMN)
		}

		if err := validateAPIRoot(r.APIRoot, "http", "https"); err != nil {
			return fmt.Errorf("roaming partner NRF API root of PLMN %s: %w", r.RoamingPLMN, err)
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

func (db *Database) ListRoamingPartners(ctx context.Context) (_ []RoamingPartner, err error) {
	ctx, finish := startDBQuery(ctx, RoamingPartnersTableName, "SELECT")
	defer func() { finish(err) }()

	if !db.appliedSchemaAtLeast(ctx, roamingSchema) {
		return nil, nil
	}

	partners, err := db.listRoamingPartners(ctx, db.reader())
	if err != nil {
		return nil, err
	}

	return partners, nil
}

func (db *Database) GetRoamingPartner(ctx context.Context, id string) (_ *RoamingPartner, err error) {
	ctx, finish := startDBQuery(ctx, RoamingPartnersTableName, "SELECT")
	defer func() { finish(err) }()

	if !db.appliedSchemaAtLeast(ctx, roamingSchema) {
		return nil, ErrNotFound
	}

	partner, err := db.getRoamingPartner(ctx, db.reader(), id)
	if err != nil {
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

func (db *Database) writeRoamingPartner(ctx context.Context, operation string, op *ChangesetOp[roamingPartnerWrite, struct{}], partner *RoamingPartner) (err error) {
	if err := partner.Validate(); err != nil {
		return err
	}

	ctx, finish := startDBQuery(ctx, RoamingPartnersTableName, operation)
	defer func() { finish(err) }()

	if _, err := op.Invoke(ctx, db, &roamingPartnerWrite{Partner: *partner}); err != nil {
		return err
	}

	return nil
}

func (db *Database) DeleteRoamingPartner(ctx context.Context, id string) (err error) {
	ctx, finish := startDBQuery(ctx, RoamingPartnersTableName, "DELETE")
	defer func() { finish(err) }()

	if _, err := opDeleteRoamingPartner.Invoke(ctx, db, &stringPayload{Value: id}); err != nil {
		return err
	}

	return nil
}

func (db *Database) listRoamingPartners(ctx context.Context, q querier) ([]RoamingPartner, error) {
	partners, err := queryAll(ctx, q, listRoamingPartnersSQL, scanRoamingPartner)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	rows, err := queryAll(ctx, q, listRoamingPartnerPLMNsSQL, scanRoamingPartnerPLMNRow)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	byPartner := map[string][]roamingPartnerPLMNRow{}
	for _, r := range rows {
		byPartner[r.PartnerID] = append(byPartner[r.PartnerID], r)
	}

	for i := range partners {
		partners[i].setPLMNs(byPartner[partners[i].ID])
	}

	return partners, nil
}

func (db *Database) getRoamingPartner(ctx context.Context, q querier, id string) (*RoamingPartner, error) {
	row, err := scanRoamingPartner(q.QueryRowContext(ctx, getRoamingPartnerSQL, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	rows, err := queryAll(ctx, q, listPLMNsOfRoamingPartnerSQL, scanRoamingPartnerPLMNRow, id)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	row.setPLMNs(rows)

	return &row, nil
}

func (p *RoamingPartner) setPLMNs(rows []roamingPartnerPLMNRow) {
	for _, r := range rows {
		plmn := RoamingPLMN{Mcc: r.Mcc, Mnc: r.Mnc}
		p.PLMNs = append(p.PLMNs, plmn)

		if r.NRFAPIRoot != nil {
			p.NRFAPIRoots = append(p.NRFAPIRoots, RoamingNRFAPIRoot{RoamingPLMN: plmn, APIRoot: *r.NRFAPIRoot})
		}
	}
}

func (db *Database) prepareHomePLMNChange(ctx context.Context, op *Operator) (bool, error) {
	if !db.appliedSchemaAtLeast(ctx, roamingSchema) {
		return false, nil
	}

	current, err := getOperator(ctx, db.runner(ctx))
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return false, fmt.Errorf("query failed: %w", err)
	}

	if current.Mcc == op.Mcc && current.Mnc == op.Mnc {
		return false, nil
	}

	count, err := queryCount(ctx, db.runner(ctx), countRoamingPartnersSQL)
	if err != nil {
		return false, fmt.Errorf("query failed: %w", err)
	}

	if count > 0 {
		return false, fmt.Errorf("%w: the home PLMN cannot change while roaming partners exist", ErrRoamingConflict)
	}

	return db.deleteSEPPIdentities(ctx)
}

func (db *Database) countVisitors(ctx context.Context, query string, args ...any) (int, error) {
	count, err := queryCount(ctx, db.readQuerier(ctx), query, args...)
	if err != nil {
		return 0, fmt.Errorf("query failed: %w", err)
	}

	return count, nil
}

func (db *Database) checkRoamingPartner(ctx context.Context, p *RoamingPartner) error {
	op, err := getOperator(ctx, db.runner(ctx))
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("query failed: %w", err)
	}

	own := RoamingPLMN{Mcc: op.Mcc, Mnc: op.Mnc}

	for _, plmn := range p.PLMNs {
		if plmn.conflicts(own) {
			return fmt.Errorf("%w: PLMN %s is or is ambiguous with the home PLMN %s", ErrRoamingConflict, plmn, own)
		}
	}

	if _, err := scanProfile(db.runner(ctx).QueryRowContext(ctx, getProfileByIDSQL, p.ProfileID)); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("%w: profile %s does not exist", ErrRoamingConflict, p.ProfileID)
		}

		return fmt.Errorf("query failed: %w", err)
	}

	if p.DiameterPeerID != nil {
		peer, err := scanDiameterPeerRow(db.runner(ctx).QueryRowContext(ctx, getDiameterPeerSQL, *p.DiameterPeerID))
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("%w: Diameter peer %s does not exist", ErrRoamingConflict, *p.DiameterPeerID)
			}

			return fmt.Errorf("query failed: %w", err)
		}

		if peer.Role != DiameterPeerRoleRoaming {
			return fmt.Errorf("%w: Diameter peer %s is not a roaming peer", ErrRoamingConflict, peer.ID)
		}
	}

	partners, err := db.listRoamingPartners(ctx, db.runner(ctx))
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

func (db *Database) insertRoamingPartnerPLMNs(ctx context.Context, p *RoamingPartner, plmns []RoamingPLMN) error {
	for _, plmn := range plmns {
		row := roamingPartnerPLMNRow{PartnerID: p.ID, Mcc: plmn.Mcc, Mnc: plmn.Mnc, NRFAPIRoot: p.NRFAPIRootOf(plmn)}

		if _, err := db.runner(ctx).ExecContext(ctx, insertRoamingPartnerPLMNSQL, row.PartnerID, row.Mcc, row.Mnc, row.NRFAPIRoot); err != nil {
			if isPrimaryKeyError(err) || isUniqueNameError(err) {
				return fmt.Errorf("%w: PLMN %s belongs to another roaming partner", ErrRoamingConflict, plmn)
			}

			return fmt.Errorf("insert roaming partner PLMN: %w", err)
		}
	}

	return nil
}

func (db *Database) insertRoamingPartnerRow(ctx context.Context, query string, args ...any) error {
	_, err := db.runner(ctx).ExecContext(ctx, query, args...)
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

	if err := db.insertRoamingPartnerRow(ctx, insertRoamingPartnerSQL, p.ID, p.Name, p.ProfileID, p.SEPPFQDN, p.SEPPN32cURI, p.SEPPN32fURI, p.SEPPTrustAnchorPEM, p.DiameterPeerID); err != nil {
		return nil, err
	}

	if err := db.insertRoamingPartnerPLMNs(ctx, p, p.PLMNs); err != nil {
		return nil, err
	}

	return nil, nil
}

func (db *Database) checkRemovedAgreements(ctx context.Context, existing, p *RoamingPartner) error {
	for _, plmn := range existing.PLMNs {
		if p.HasPLMN(plmn) {
			continue
		}

		n, err := db.countVisitors(ctx, countVisitorsOfPartnerSQL, p.ID, plmn.imsiGlob())
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

		n, err := db.countVisitors(ctx, countAttachedVisitorsOfRATSQL, p.ID, rat.regType)
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

	if err := db.insertRoamingPartnerRow(ctx, updateRoamingPartnerSQL, p.Name, p.ProfileID, p.SEPPFQDN, p.SEPPN32cURI, p.SEPPN32fURI, p.SEPPTrustAnchorPEM, p.DiameterPeerID, p.ID); err != nil {
		return nil, err
	}

	var added []RoamingPLMN

	for _, plmn := range existing.PLMNs {
		if p.HasPLMN(plmn) {
			continue
		}

		row := roamingPartnerPLMNRow{PartnerID: p.ID, Mcc: plmn.Mcc, Mnc: plmn.Mnc}
		if _, err := db.runner(ctx).ExecContext(ctx, deleteRoamingPartnerPLMNSQL, row.PartnerID, row.Mcc, row.Mnc); err != nil {
			return nil, fmt.Errorf("delete roaming partner PLMN: %w", err)
		}
	}

	for _, plmn := range p.PLMNs {
		if !existing.HasPLMN(plmn) {
			added = append(added, plmn)
			continue
		}

		row := roamingPartnerPLMNRow{PartnerID: p.ID, Mcc: plmn.Mcc, Mnc: plmn.Mnc, NRFAPIRoot: p.NRFAPIRootOf(plmn)}
		if _, err := db.runner(ctx).ExecContext(ctx, updateRoamingPartnerPLMNSQL, row.NRFAPIRoot, row.PartnerID, row.Mcc, row.Mnc); err != nil {
			return nil, fmt.Errorf("update roaming partner PLMN: %w", err)
		}
	}

	if err := db.insertRoamingPartnerPLMNs(ctx, p, added); err != nil {
		return nil, err
	}

	return nil, nil
}

func (db *Database) applyDeleteRoamingPartner(ctx context.Context, p *stringPayload) (any, error) {
	n, err := db.countVisitors(ctx, countVisitorsOfPartnerSQL, p.Value, "*")
	if err != nil {
		return nil, err
	}

	if n > 0 {
		return nil, fmt.Errorf("%w: the roaming partner cannot be deleted while %d of its roamers are attached", ErrRoamersAttached, n)
	}

	res, err := db.runner(ctx).ExecContext(ctx, deleteRoamingPartnerSQL, p.Value)
	if err != nil {
		if isForeignKeyRestrictError(err) {
			return nil, ErrInUse
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return nil, ErrNotFound
	}

	return nil, nil
}

func (db *Database) SetRoamingPartnerWithdrawing(ctx context.Context, id string, until time.Time) error {
	return db.setWithdrawing(ctx, opSetRoamingPartnerWithdrawing, RoamingPartnersTableName, id, until)
}

func (db *Database) SetVisitorDeregistering(ctx context.Context, imsi string, until time.Time) error {
	return db.setWithdrawing(ctx, opSetVisitorDeregistering, VisitorsTableName, imsi, until)
}

func (db *Database) setWithdrawing(ctx context.Context, op *ChangesetOp[withdrawingPayload, struct{}], table, key string, until time.Time) (err error) {
	ctx, finish := startDBQuery(ctx, table, "UPDATE")
	defer func() { finish(err) }()

	payload := &withdrawingPayload{Key: key}
	if !until.IsZero() {
		payload.Until = until.Unix()
	}

	if _, err := op.Invoke(ctx, db, payload); err != nil {
		return err
	}

	return nil
}

func (db *Database) CountVisitorsOfPartner(ctx context.Context, partnerID string) (int, error) {
	if !db.appliedSchemaAtLeast(ctx, roamingSchema) {
		return 0, nil
	}

	return db.countVisitors(ctx, countVisitorsOfPartnerSQL, partnerID, "*")
}

func (db *Database) applySetRoamingPartnerWithdrawing(ctx context.Context, p *withdrawingPayload) (any, error) {
	return nil, db.updateOne(ctx, setRoamingPartnerWithdrawingSQL, p.Until, p.Key)
}

func (db *Database) applySetVisitorDeregistering(ctx context.Context, p *withdrawingPayload) (any, error) {
	return nil, db.updateOne(ctx, setVisitorDeregisteringSQL, p.Until, p.Key)
}

func (db *Database) updateOne(ctx context.Context, query string, args ...any) error {
	res, err := db.runner(ctx).ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}

	if n == 0 {
		return ErrNotFound
	}

	return nil
}

func scanRoamingPartner(row scanner) (RoamingPartner, error) {
	var v RoamingPartner

	if err := row.Scan(&v.ID, &v.Name, &v.ProfileID, &v.SEPPFQDN, &v.SEPPN32cURI, &v.SEPPN32fURI, &v.SEPPTrustAnchorPEM, &v.DiameterPeerID, &v.WithdrawingUntil); err != nil {
		return RoamingPartner{}, err
	}

	return v, nil
}

func scanRoamingPartnerPLMNRow(row scanner) (roamingPartnerPLMNRow, error) {
	var v roamingPartnerPLMNRow

	if err := row.Scan(&v.PartnerID, &v.Mcc, &v.Mnc, &v.NRFAPIRoot); err != nil {
		return roamingPartnerPLMNRow{}, err
	}

	return v, nil
}
